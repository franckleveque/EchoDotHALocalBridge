# Page de logs des appels HA Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Exposer dans l'admin, derrière l'authentification existante, les derniers appels vers Home Assistant (URL, payload, code, corps de l'erreur).

**Architecture:** Un type `HACallRecord` et une erreur `HAAPIError` dans le domaine, un port `HACallLog`, un adaptateur de sortie en tampon circulaire. Le client HA enregistre chaque appel de service et renvoie `HAAPIError` (code + corps). `BridgeService` expose la lecture via `AdminPort`, le serveur HTTP sert `/admin/logs` et `/admin/logs/data`.

**Tech Stack:** Go 1.25, `net/http`, testify (mock/assert), tests e2e avec tag `e2e`.

**Spec:** `docs/superpowers/specs/2026-10-24-admin-diagnostics-and-test-consistency-design.md`

## Global Constraints

- Hexagonal : jamais d'import de `internal/adapters/...` depuis `internal/domain/...` ; injection par interfaces de `internal/ports` (`agents.md`, `internal/architecture_test.go`).
- `make test` (`go test -v ./...`) doit passer avant chaque commit ; couverture > 80 % dans `internal/domain/service` et `internal/domain/translator`.
- Tampon : 200 entrées, en mémoire. Corps de réponse conservé seulement si code >= 400, tronqué à 2048 octets. Jamais d'en-tête `Authorization` enregistré.
- Seuls les appels `/api/services/...` (dont effets) sont enregistrés, pas `GET /api/states`.
- Les routes de logs passent par `s.withBasicAuth`. Valeurs affichées via `textContent`, jamais `innerHTML`.
- Interface en anglais. Pas de tiret cadratin dans les textes.

## Review Focus

- Corps d'erreur HA contenant `<script>` ou du HTML : affiché comme texte brut (test sur le HTML servi : pas d'`innerHTML` pour ces champs).
- Corps d'erreur très gros (> 2 Ko) : tronqué, pas d'explosion mémoire.
- Appels concurrents (10 workers) vers le tampon : pas de data race (`go test -race`).
- HA injoignable (erreur de transport, pas de réponse) : une entrée avec `Error` renseigné et `Status` 0.
- Accès à `/admin/logs` sans identifiants : 401 ; avant le setup : 403.

---

### Task 1: Types du domaine, port et tampon circulaire

**Files:**
- Create: `internal/domain/model/hacall.go`
- Create: `internal/adapters/output/calllog/ring.go`
- Modify: `internal/ports/bridge_port.go`
- Test: `internal/domain/model/hacall_test.go`, `internal/adapters/output/calllog/ring_test.go`

**Interfaces:**
- Produces:
  - `model.HACallRecord{Time time.Time; Method, URL, Payload string; Status int; ResponseBody, Error string; DurationMs int64}` avec tags JSON `time, method, url, payload, status, response_body (omitempty), error (omitempty), duration_ms`.
  - `model.HAAPIError{Status int; Body string}`, méthode `(*HAAPIError) Error() string` : `"HA API error: 400: <body>"`, ou `"HA API error: 400"` si le corps est vide.
  - `ports.HACallLog` : `Record(model.HACallRecord)` et `List() []model.HACallRecord` (plus récent d'abord).
  - `calllog.NewRing(capacity int) *Ring` implémentant `ports.HACallLog`, sûr en concurrence.

- [ ] **Step 1: Écrire les tests qui échouent**
  - `TestHAAPIError_Error` : avec corps `{"message":"x"}` → `HA API error: 400: {"message":"x"}` ; corps vide → `HA API error: 400`.
  - `TestRing_NewestFirstAndCapacity` : capacité 3, 5 enregistrements numérotés par `Status` 1..5 → `List()` renvoie les statuts `[5,4,3]`.
  - `TestRing_ListIsCopy` : modifier la slice retournée ne change pas un second `List()`.
  - `TestRing_Concurrent` : 20 goroutines × 50 `Record` en parallèle, puis `List()` de longueur 200 (capacité 200).
- [ ] **Step 2: Lancer et vérifier l'échec** : `go test ./internal/domain/model ./internal/adapters/output/calllog` → échec de compilation (types absents).
- [ ] **Step 3: Implémenter** les types dans `hacall.go`, l'interface `HACallLog` dans `bridge_port.go`, `Ring` dans `ring.go` (slice circulaire protégée par `sync.Mutex`).
- [ ] **Step 4: Lancer** `go test -race ./internal/domain/model ./internal/adapters/output/calllog ./internal/` → PASS (inclut `architecture_test.go`).
- [ ] **Step 5: Commit** `feat: add HA call record, HAAPIError and ring buffer`

### Task 2: Le client HA enregistre les appels et renvoie HAAPIError

**Files:**
- Modify: `internal/adapters/output/homeassistant/client.go` (`SetState` l.~85-135, `executeEffect`, `GetRawStates`)
- Test: `internal/adapters/output/homeassistant/client_test.go` (nouveau)

**Interfaces:**
- Consumes: `ports.HACallLog`, `model.HAAPIError`, `model.HACallRecord`.
- Produces: `(*Client).SetCallLog(l ports.HACallLog)` ; `SetState` et `GetRawStates` renvoient `*model.HAAPIError` pour tout code >= 400 (`errors.As` fonctionne).

- [ ] **Step 1: Tests qui échouent** (serveur `httptest`, client configuré dessus, `calllog.NewRing(10)`) :
  - `TestSetState_RecordsSuccess` : HA répond 200 → une entrée avec `Method=POST`, `URL` finissant par `/api/services/switch/turn_on`, `Payload` contenant l'entité, `Status=200`, `ResponseBody` vide.
  - `TestSetState_400ReturnsHAAPIErrorWithBody` : HA répond 400 `{"message":"Service not found."}` → `errors.As(err, &*model.HAAPIError)` avec `Status=400`, `Body` identique ; l'entrée enregistrée porte le même corps.
  - `TestSetState_BodyTruncated` : corps de 5000 octets → `len(Body) <= 2048`.
  - `TestSetState_TransportError` : serveur fermé → entrée avec `Status=0` et `Error` non vide.
  - `TestSetState_NoAuthorizationInRecord` : le jeton `test-token` n'apparaît dans aucun champ de l'entrée.
  - `TestSetState_NilCallLogIsSafe` : sans `SetCallLog`, `SetState` fonctionne.
  - `TestExecuteEffect_Recorded` : une commande avec `Effect: "script.fx"` produit deux entrées.
- [ ] **Step 2: Lancer** `go test ./internal/adapters/output/homeassistant -run 'SetState|Effect'` → FAIL.
- [ ] **Step 3: Implémenter** : champ `callLog ports.HACallLog` ; helper privé `record(method, url, payload string, status int, body []byte, err error, start time.Time)` ; lecture du corps avec `io.LimitReader(resp.Body, 2048)` uniquement si code >= 400 ; `SetState` renvoie `&model.HAAPIError{...}`. `GetRawStates` renvoie aussi `HAAPIError` mais sans enregistrer.
- [ ] **Step 4: Lancer** `go test -race ./internal/adapters/output/homeassistant ./internal/e2e -tags e2e` → PASS (les e2e existants vérifient que rien n'a régressé).
- [ ] **Step 5: Commit** `feat: record HA service calls and return HA error body`

### Task 3: Lecture des appels via BridgeService et AdminPort

**Files:**
- Modify: `internal/ports/bridge_port.go` (`AdminPort`), `internal/domain/service/bridge.go`, `cmd/bridge/main.go` (l.60 et l.92)
- Test: `internal/domain/service/bridge_test.go`

**Interfaces:**
- Consumes: `ports.HACallLog`.
- Produces: `AdminPort.GetHACalls(ctx context.Context) []model.HACallRecord` ; `(*BridgeService).SetCallLog(l ports.HACallLog)`. Sans journal configuré, `GetHACalls` renvoie une slice vide (non nil).

- [ ] **Step 1: Tests qui échouent** : `TestBridgeService_GetHACalls_Empty` (sans `SetCallLog`, longueur 0, non nil) ; `TestBridgeService_GetHACalls_ReturnsLog` (un faux `HACallLog` renvoie 2 entrées, le service les rend telles quelles).
- [ ] **Step 2: Lancer** `go test ./internal/domain/service -run GetHACalls` → FAIL.
- [ ] **Step 3: Implémenter** la méthode et le setter ; ajouter `GetHACalls` à `AdminPort` ; dans `main.go` créer `ring := calllog.NewRing(200)` et appeler `haClient.SetCallLog(ring)` et `bridgeService.SetCallLog(ring)`. Si un mock d'`AdminPort` existe dans les tests, lui ajouter la méthode.
- [ ] **Step 4: Lancer** `go build ./... && go test ./...` → PASS.
- [ ] **Step 5: Commit** `feat: expose HA call log through AdminPort`

### Task 4: Routes et page /admin/logs

**Files:**
- Create: `internal/adapters/input/http/logs_handlers.go`
- Modify: `internal/adapters/input/http/server.go:48-51` (routes), `internal/adapters/input/http/admin_handlers.go` (lien dans la page admin près du `<h1>` l.238)
- Test: `internal/e2e/admin_api_test.go` (tag `e2e`), `internal/adapters/input/http/logs_handlers_test.go`

**Interfaces:**
- Consumes: `AdminPort.GetHACalls`.
- Produces: `GET /admin/logs` (HTML), `GET /admin/logs/data` (JSON : tableau de `HACallRecord`, `[]` si vide). Méthodes autres que GET → 405.

- [ ] **Step 1: Tests e2e qui échouent** (faux HA qui répond 400 sur `/api/services/`, utiliser `newTestStack`, setup via `/admin/setup`, Basic Auth `admin`/`password123`) :
  - `TestAdminLogs_RequiresAuth` : sans identifiants → 401 sur `/admin/logs` et `/admin/logs/data`.
  - `TestAdminLogs_ShowsFailedCall` : un `POST /admin/test-action` provoque un appel 400 ; ensuite `GET /admin/logs/data` contient une entrée avec `status=400`, `url` finissant par `/api/services/switch/turn_on` et `response_body` égal au corps du faux HA (attendre l'appel avec `assert.Eventually`, l'appel est encore asynchrone à ce stade).
  - `TestAdminLogs_EmptyIsJSONArray` : sans appel, corps `[]`.
- [ ] **Step 2: Test unitaire** `TestLogsPage_UsesTextContent` : le HTML de `/admin/logs` contient `textContent` et ne contient pas `innerHTML`.
- [ ] **Step 3: Lancer** `go test -tags e2e ./internal/e2e -run AdminLogs` et `go test ./internal/adapters/input/http` → FAIL (404).
- [ ] **Step 4: Implémenter** `handleLogsPage` et `handleLogsData` ; enregistrer les deux routes avec `s.withBasicAuth` ; page avec tableau (heure, méthode, URL, payload, code, durée, erreur / corps), case « Errors only », bouton Refresh qui recharge `/admin/logs/data` ; lien « Logs » dans la page admin.
- [ ] **Step 5: Lancer** `make test && go test -race -tags e2e ./...` → PASS.
- [ ] **Step 6: Vérification manuelle** : lancer le binaire contre un faux HA qui répond 400, ouvrir `/admin/logs`, constater l'appel et son corps. Dire explicitement ce qui n'a pas pu être vérifié (HA réel).
- [ ] **Step 7: Commit** `feat: add /admin/logs page for Home Assistant calls`
