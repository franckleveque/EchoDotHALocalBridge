# Socle de logging et page /admin/logs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Envoyer les logs `slog` vers la sortie standard ET vers un tampon mémoire borné, journaliser chaque appel HA avec le corps de l'erreur, et afficher ces logs dans l'admin derrière l'authentification existante.

**Architecture:** On garde `log/slog`. Un handler de diffusion (`slog.NewMultiHandler`, Go 1.26) alimente la sortie standard (inchangée) et un `LineRing` (tampon de lignes). Le client HA journalise requête, réponse et corps d'erreur via `slog` et renvoie `model.HAAPIError`. Le serveur HTTP lit le tampon via un port `LogSource` et sert `/admin/logs` et `/admin/logs/data`.

**Tech Stack:** Go 1.26, `log/slog`, `net/http`, testify, tests e2e (tag `e2e`).

**Spec:** `docs/superpowers/specs/2026-10-24-admin-diagnostics-and-test-consistency-design.md`

## Global Constraints

- Cible : Raspberry Pi. **Aucun fichier de logs**, aucune écriture disque ajoutée par l'application. Mémoire bornée : 500 lignes maximum, 4096 octets maximum par ligne (pire cas environ 2 Mo).
- `go.mod` passe à `go 1.26.0` (CI et Dockerfile utilisent déjà 1.26).
- Hexagonal : pas d'import de `internal/adapters/...` depuis `internal/domain/...` ; le serveur HTTP lit les logs via `internal/ports` (`agents.md`, `internal/architecture_test.go`).
- `make test` doit passer avant chaque commit ; couverture > 80 % dans `internal/domain/service` et `internal/domain/translator`.
- Le jeton HA et l'en-tête `Authorization` ne sont jamais écrits dans un log.
- Corps de réponse HA journalisé seulement pour les codes >= 400, tronqué à 2048 octets.
- La sortie standard garde le même format texte qu'aujourd'hui (`slog.NewTextHandler`) ; le tampon utilise le même format.
- Routes de logs derrière `s.withBasicAuth`. Valeurs affichées via `textContent`, jamais `innerHTML`.
- Interface en anglais. Pas de tiret cadratin dans les textes.

## Review Focus

- Ligne de log très longue (corps HA de 1 Mo) : tronquée dans le tampon, sortie standard intacte.
- Rafale de logs (10 workers + requêtes HTTP) : pas de data race (`go test -race`), mémoire bornée.
- Corps d'erreur HA contenant `<script>` : rendu comme texte brut.
- Jeton HA : absent du tampon, y compris après une erreur d'appel.
- `/admin/logs` sans identifiants : 401 ; avant le setup : 403.
- Le tampon vide : la page affiche un message vide et `/admin/logs/data` renvoie `[]`.

---

### Task 1: Type d'erreur HA, tampon de lignes, handler de diffusion, go 1.26

**Files:**
- Modify: `go.mod` (`go 1.26.0`)
- Create: `internal/domain/model/hacall.go`
- Create: `internal/adapters/output/logging/ring.go`, `internal/adapters/output/logging/handler.go`
- Modify: `internal/ports/bridge_port.go` (interface `LogSource`)
- Test: `internal/domain/model/hacall_test.go`, `internal/adapters/output/logging/ring_test.go`, `internal/adapters/output/logging/handler_test.go`

**Interfaces:**
- Produces:
  - `model.HAAPIError{Status int; Body string}` ; `(*HAAPIError) Error() string` : `"HA API error: 400: <body>"`, ou `"HA API error: 400"` si le corps est vide.
  - `ports.LogSource` : `Lines() []string` (ordre chronologique, copie).
  - `logging.NewLineRing(maxLines, maxLineBytes int) *LineRing` : `io.Writer` ; chaque appel à `Write` est une ligne (retour à la ligne final retiré, troncature à `maxLineBytes`), implémente `ports.LogSource`, sûr en concurrence.
  - `logging.NewHandler(out io.Writer, ring io.Writer, level slog.Leveler) slog.Handler` : diffusion vers deux `slog.NewTextHandler` via `slog.NewMultiHandler`.

- [ ] **Step 1: Écrire les tests qui échouent**
  - `TestHAAPIError_Error` : corps `{"message":"x"}` → `HA API error: 400: {"message":"x"}` ; corps vide → `HA API error: 400`.
  - `TestLineRing_KeepsLastN` : capacité 3, 5 écritures `l1`..`l5` → `Lines()` = `[l3 l4 l5]`.
  - `TestLineRing_TruncatesLongLine` : ligne de 10 000 octets avec `maxLineBytes=4096` → longueur <= 4096.
  - `TestLineRing_StripsTrailingNewline` : `Write("abc\n")` → ligne `abc`.
  - `TestLineRing_LinesIsCopy` : modifier la slice retournée ne change pas le tampon.
  - `TestLineRing_Concurrent` : 20 goroutines × 100 écritures, puis longueur égale à la capacité.
  - `TestNewHandler_WritesToBoth` : un `slog.New(NewHandler(buf, ring, slog.LevelInfo))` qui journalise `msg=hello` écrit dans `buf` et dans le tampon ; un message de niveau Debug n'apparaît dans aucun des deux.
- [ ] **Step 2: Lancer** `go test ./internal/domain/model ./internal/adapters/output/logging` → échec de compilation.
- [ ] **Step 3: Implémenter** : `go mod edit -go=1.26.0` ; types et sentinelles dans `hacall.go` ; `LineRing` (mutex, slice circulaire) ; `NewHandler`.
- [ ] **Step 4: Lancer** `go build ./... && go test -race ./internal/domain/model ./internal/adapters/output/logging ./internal/` → PASS (inclut `architecture_test.go`).
- [ ] **Step 5: Commit** `feat: add logging ring buffer, fan-out handler and HAAPIError (go 1.26)`

### Task 2: Brancher le handler dans main.go

**Files:**
- Modify: `cmd/bridge/main.go:21-33` (création du handler), `cmd/bridge/main.go:116` (serveur HTTP)
- Modify: `internal/adapters/input/http/server.go` (champ `logs ports.LogSource`, méthode `SetLogSource`)

**Interfaces:**
- Consumes: `logging.NewLineRing`, `logging.NewHandler`, `ports.LogSource`.
- Produces: `(*Server).SetLogSource(ports.LogSource)`. Sans source configurée, les handlers de logs répondent comme un tampon vide.

- [ ] **Step 1: Implémenter** : `ring := logging.NewLineRing(500, 4096)` ; `handler := logging.NewHandler(os.Stdout, ring, levelVar)` à la place de `slog.NewTextHandler(os.Stdout, ...)` ; `httpServer.SetLogSource(ring)` après `NewServer`. La logique `LOG_LEVEL` existante est inchangée.
- [ ] **Step 2: Vérifier** : `go build ./... && make test` → PASS ; lancer le binaire (`LOG_LEVEL=INFO CONFIG_PATH=/tmp/claude-0/cfg.json ./bridge`) et constater que la sortie standard est identique à avant (mêmes lignes `time=... level=... msg=...`).
- [ ] **Step 3: Commit** `feat: send slog output to stdout and in-memory ring`

### Task 3: Le client HA journalise les appels et renvoie HAAPIError

**Files:**
- Modify: `internal/adapters/output/homeassistant/client.go` (`SetState`, `executeEffect`, `GetRawStates`)
- Test: `internal/adapters/output/homeassistant/client_test.go` (nouveau)

**Interfaces:**
- Consumes: `model.HAAPIError`.
- Produces: `SetState` et `GetRawStates` renvoient `*model.HAAPIError` pour tout code >= 400 (`errors.As` fonctionne). Le journal `slog` contient : ligne existante `HA Service Call` (url, payload), nouvelle ligne `HA Service Response` (url, status, duration_ms) à l'info, et `HA Service Error` (url, status, body tronqué à 2048) au niveau error pour >= 400. Les erreurs de transport sont journalisées au niveau error avec l'URL.

- [ ] **Step 1: Tests qui échouent** (serveur `httptest`, client configuré dessus ; capturer les logs avec un handler `slog` de test posé via `slog.SetDefault` puis restauré) :
  - `TestSetState_400ReturnsHAAPIErrorWithBody` : HA répond 400 `{"message":"Service not found."}` → `errors.As` donne `Status=400` et le même `Body`.
  - `TestSetState_LogsErrorBody` : le journal capturé contient `Service not found.` et `status=400`.
  - `TestSetState_BodyTruncated` : corps de 5000 octets → `len(Body) <= 2048`.
  - `TestSetState_LogsSuccess` : HA répond 200 → le journal contient `HA Service Response` et `status=200`.
  - `TestSetState_TransportError` : serveur fermé → erreur non nulle, journal d'erreur avec l'URL.
  - `TestSetState_TokenNeverLogged` : `test-token` est absent du journal dans tous les cas.
  - `TestSetState_LogsActualPayload` : une commande dont `Data` contient `{"value":0}` produit un journal contenant ce payload.
  - `TestExecuteEffect_Logged` : un `Effect: "script.fx"` produit une ligne de réponse pour l'effet.
- [ ] **Step 2: Lancer** `go test ./internal/adapters/output/homeassistant` → FAIL.
- [ ] **Step 3: Implémenter** : lecture du corps avec `io.LimitReader(resp.Body, 2048)` seulement si code >= 400 ; retour de `&model.HAAPIError{...}` ; journaux ci-dessus.
- [ ] **Step 4: Lancer** `go test -race ./internal/adapters/output/homeassistant && go test -race -tags e2e ./internal/e2e` → PASS.
- [ ] **Step 5: Commit** `feat: log HA service calls and return HA error body`

### Task 4: Routes et page /admin/logs

**Files:**
- Create: `internal/adapters/input/http/logs_handlers.go`
- Modify: `internal/adapters/input/http/server.go:48-51` (routes), `internal/adapters/input/http/admin_handlers.go` (lien « Logs » près du `<h1>` l.238), `internal/e2e/helpers_test.go` (brancher un `LineRing` + `SetLogSource` dans `newTestStack`, handler `slog` par défaut vers ce tampon pendant le test)
- Test: `internal/adapters/input/http/logs_handlers_test.go`, `internal/e2e/admin_api_test.go`

**Interfaces:**
- Consumes: `ports.LogSource`.
- Produces: `GET /admin/logs` (HTML), `GET /admin/logs/data` (JSON : tableau de chaînes, `[]` si vide). Autres méthodes → 405.

- [ ] **Step 1: Tests qui échouent** :
  - e2e `TestAdminLogs_RequiresAuth` : sans identifiants → 401 sur les deux routes ; avant `/admin/setup` → 403.
  - e2e `TestAdminLogs_ShowsFailedHACall` : faux HA qui répond 400 + `{"message":"Service not found."}` ; un appel (via `/admin/test-action`) ; `assert.Eventually` sur `/admin/logs/data` : une ligne contient `Service not found.` et `status=400`.
  - e2e `TestAdminLogs_EmptyIsJSONArray` : tampon vide → corps `[]`.
  - unitaire `TestLogsPage_UsesTextContent` : le HTML de `/admin/logs` contient `textContent` et pas `innerHTML`.
  - unitaire `TestLogsData_MethodNotAllowed` : POST → 405.
- [ ] **Step 2: Lancer** `go test ./internal/adapters/input/http` et `go test -tags e2e ./internal/e2e -run AdminLogs` → FAIL (404).
- [ ] **Step 3: Implémenter** `handleLogsPage` et `handleLogsData` ; page : zone `<pre>`, champ de filtre texte, bouton Refresh, case « Auto-refresh (5 s) » décochée par défaut ; enregistrer les routes avec `s.withBasicAuth` ; lien « Logs » dans la page admin.
- [ ] **Step 4: Lancer** `make test && go test -race -tags e2e ./...` → PASS.
- [ ] **Step 5: Vérification manuelle** : binaire contre un faux HA qui répond 400, ouvrir `/admin/logs`, constater l'appel et le corps de l'erreur. Dire ce qui n'a pas été vérifié (HA réel, Raspberry Pi réel).
- [ ] **Step 6: Commit** `feat: add /admin/logs page`

### Task 5: Protéger le disque du Pi côté Docker

**Files:**
- Modify: `docker-compose.yml`, `README.md` (section logs, près de la ligne 115)

**Interfaces:** aucune.

- [ ] **Step 1: Vérifier le point de départ** : le pilote `json-file` de Docker n'a pas de limite de taille par défaut (`max-size` vaut -1, source : https://docs.docker.com/engine/logging/drivers/json-file.md) et le compose actuel fixe `LOG_LEVEL=DEBUG` sans bloc `logging`.
- [ ] **Step 2: Modifier** `docker-compose.yml` : ajouter au service un bloc `logging` avec `driver: json-file` et `options: { max-size: "10m", max-file: "3" }` (`max-file` n'agit que si `max-size` est défini). Ne pas changer `LOG_LEVEL`, mais ajouter dans le README une note : `DEBUG` journalise aussi les corps de requêtes Hue, utiliser `INFO` en usage normal.
- [ ] **Step 3: Vérifier** `docker compose config` (si Docker est disponible ; sinon le dire et valider le YAML avec un parseur).
- [ ] **Step 4: Commit** `chore: cap docker log size for Raspberry Pi`
