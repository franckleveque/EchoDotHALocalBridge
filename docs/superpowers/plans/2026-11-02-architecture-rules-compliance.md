# Conformité aux règles d'architecture OpenGrep Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Amener `opengrep scan -f .opengrep/architecture.yml --error .` à `exit=0` (155 violations au départ, `errors` vide) sans changer `architecture.yml` ni le comportement observable, puis brancher le scan dans la CI.

**Architecture:** Refactors par règle, du plus mécanique au plus structurant : (1) un type par fichier, (2) remplacement des `switch` / `else if`, (3) Demeter par délégation, (4) logger injecté par constructeur, (5) `nosemgrep` motivés pour les chaînes imposées par le framework, (6) CI et PR.

**Tech Stack:** Go 1.26, `log/slog`, `net/http` (`ServeMux` à méthodes et jokers), testify, OpenGrep 1.33.1.

**Spec:** `docs/superpowers/specs/2026-11-02-architecture-rules-compliance-design.md`

## Global Constraints

- `.opengrep/architecture.yml` n'est jamais modifié. Une règle jugée fausse est signalée dans la PR, pas contournée.
- `nosemgrep` uniquement pour `demeter-chain`, uniquement pour un appel imposé par le langage ou le framework, avec la raison sur la ligne.
- Aucun changement de comportement hors la section « Changements de comportement » de la spec. Tests de caractérisation écrits et verts **avant** chaque refactor.
- Tests existants : seuls les appels de constructeurs changent (argument logger). Aucune assertion modifiée. Exception unique déclarée : le test `TestHAEntityState_IsSupported` suit le code déplacé vers `EntityFilter`, mêmes cas, mêmes valeurs attendues.
- `make test` et `go test -race -tags e2e ./...` verts avant chaque commit ; couverture domaine 100 % et globale >= 70 % (contrôles du `ci.yml`).
- Aucun secret dans un log : jeton HA, mot de passe, `Authorization`, `HUE_ENCRYPTION_KEY`.
- OpenGrep ne scanne que les fichiers suivis par git : `git add` des nouveaux fichiers avant chaque scan.
- `opengrep-report.*` n'est jamais commité (ajouté à `.gitignore`).
- Interface en anglais, pas de tiret cadratin dans les textes.
- Un commit par tâche, message en anglais, branche `claude/funny-brahmagupta-u32i1x`.

## Review Focus

- Chaque `nosemgrep` : l'appel appartient-il vraiment au framework ?
- Les 3 arbitrages de la règle logger (spec D3) : `EntityFilter`, `HAAPIError.Error`, `LineRing`.
- Les changements de comportement (405 sur méthodes non gérées de `/admin/setup` et `/admin/config`).
- Routes `/api/...` : mêmes réponses qu'avant pour les chemins inconnus et avec ou sans `/` final (Alexa).
- Aucun secret dans les nouveaux logs.

## Couverture des règles (155 violations)

| Règle | Avant | Tâche | Après |
|---|---:|---|---:|
| `one-type-per-file` | 27 | 1 | 0 |
| `ocp-no-switch-or-else-if` | 16 | 2, 3 | 0 |
| `demeter-chain` (code) | 4 | 4 | 0 |
| `demeter-chain` (framework) | 22 | 6 | 0 (supprimées par `nosemgrep`) |
| `logger-global-not-injected` | 26 | 5 | 0 |
| `logger-missing-in-decision-type` | 60 | 5 | 0 |
| `hexa-*`, `ddd-*`, `dip-*`, `port-*`, `isp-*`, `lsp-*`, `no-utility-package`, `go-context-first-param` | 0 | toutes | 0 (non régression) |

Hexagonal, DDD, DIP, OCP (structure), ISP, LSP : aucune violation au départ, donc aucun changement de code propre à ces règles. Les nouveaux fichiers et dépendances (`*slog.Logger` est de la stdlib, autorisé dans le domaine) sont conçus pour ne pas en créer ; le scan final le prouve.

---

### Task 0: Mise en place

**Files:**
- Create: `.opengrep/architecture.yml` (copie du fichier fourni, intact)
- Modify: `.gitignore` (`opengrep-report.*`)
- Create: `docs/superpowers/specs/2026-11-02-architecture-rules-compliance-design.md`, ce plan

- [ ] Vérifier `opengrep --version` = 1.33.1, `diff` du YAML avec l'original = vide.
- [ ] Ajouter `opengrep-report.*` à `.gitignore`.
- [ ] Baseline : `go test -race -tags e2e ./...` vert (déjà constaté) ; scan = 155, `errors` vide.
- [ ] Commit : `Add OpenGrep architecture rules, spec and plan`.

---

### Task 1: `one-type-per-file` (27 → 0), déplacements purs

Aucun changement de code, seulement des déplacements. Garde-fou : `go build ./... && go test ./...` vert après chaque lot ; `gofmt -l` vide.

**Files (Create, puis retrait du type du fichier d'origine) :**
- `internal/domain/model/config.go` : garde `Config` ; crée `mapping_type.go` (`MappingType` + constantes), `action_config.go`, `virtual_device.go`
- `internal/domain/model/device.go` : garde `Device` ; crée `hue_metadata.go`, `device_state.go`, `ha_fields.go`, `ha_entity_state.go` (avec `IsSupported` pour l'instant), `home_assistant_command.go`
- `internal/domain/translator/translator.go` : garde `Translator` ; crée `translator_factory.go`
- `internal/ports/auth_port.go` : garde `AuthPort` ; crée `auth_service.go`
- `internal/ports/bridge_port.go` : crée `home_assistant_entity.go`, `hue_emulation_port.go`, `admin_port.go`, `home_assistant_port.go`, `reconfigurable_home_assistant_port.go`, `reconfigurable.go`, `log_source.go`, `translator_aliases.go` (alias, non comptés par la règle) ; supprime `bridge_port.go`
- `internal/adapters/input/http/server.go` : garde `Server` ; crée `logging_response_writer.go`
- `internal/adapters/output/persistence/json_repo.go` : garde `JSONConfigRepository` ; crée `legacy_config.go`, `legacy_entity_mapping.go`, `legacy_custom_formula.go`

- [ ] Déplacer lot par lot (model, translator, ports, http, persistence) ; après chaque lot : `gofmt`, `go build`, `go test ./...`.
- [ ] `git add -A` puis scan : `one-type-per-file` = 0, total attendu 128.
- [ ] Commit : `Split files so each holds a single type`.

---

### Task 2: OCP hors HTTP (7 findings)

TDD : les tests existants servent de caractérisation ; ajouter un test pour chaque nouvelle fonction avant de l'écrire.

**Files:**
- Create: `internal/adapters/output/logging/level.go` + `level_test.go` : `ParseLevel(s string) slog.Level` (table `DEBUG`, `WARN`, `ERROR`, défaut `INFO`).
- Modify: `cmd/bridge/main.go` : `levelVar.Set(logging.ParseLevel(os.Getenv("LOG_LEVEL")))` ; `getLocalIP` extrait l'IP via `strings.Cut(addr.String(), "/")` puis `net.ParseIP` (plus de `switch addr.(type)`). Test dans `cmd/bridge/main_test.go` : `*net.IPNet` (`192.168.1.10/24`), `*net.IPAddr`, IPv6 ignorée.
- Modify: `internal/adapters/output/persistence/json_repo.go` : `NewJSONConfigRepository` itère `aesKeySizes = []int{32, 24, 16}`, premier `size <= len(envKey)` ; si aucun, clé par défaut. Avertissement unique `HUE_ENCRYPTION_KEY is longer than N characters and will be truncated to N` quand `len > size`. Tests : longueurs 40, 32, 28, 24, 20, 16, 10 (clé retenue et avertissement attendus).
- Modify: `internal/domain/translator/custom.go` :
  - `inputAttributes = []string{"brightness", "current_position", "temperature", "value"}`, premier `float64` présent gagne (ordre identique à la chaîne actuelle) ;
  - `switch domain` remplacé par une table `map[string]DomainMapping` (nouveau fichier `domain_mapping.go` : `Service string`, `Param string`, `Convert func(float64) any`) ; défaut `{Param: "value"}`. `cover` : service `set_cover_position`, param `position`, `int(output)` ; `climate` : `set_temperature` / `temperature` ; `input_number` : `set_value` / `value` ; `light` : `brightness`.
  - Les cas existants de `translator_test.go` (custom, formules, domaines) prouvent la parité ; ajouter un cas par attribut d'entrée (priorité) si absent.
- Modify: `internal/domain/service/bridge.go` : `extractEntities` itère `nameAttributes = []string{"friendly_name", "name"}`. Cas de test : seul `name`, les deux, aucun, valeur non texte.

- [ ] Tests d'abord (rouges), puis implémentation (verts), un sous-lot par fichier.
- [ ] `go test -race -tags e2e ./...` vert, scan : `ocp` ne contient plus que les sites HTTP (9 restants), total attendu 121.
- [ ] Commit : `Replace switch and else-if chains with tables in non-HTTP code`.

---

### Task 3: OCP HTTP (9 findings)

**Files:**
- Modify: `internal/adapters/input/http/{server,admin_handlers,hue_handlers,logs_handlers}.go`
- Create: `internal/adapters/input/http/routing_characterization_test.go`, `test_action_status.go` (+ test)

- [ ] **Caractérisation d'abord, sur le code actuel** (doit être verte avant tout changement). Tableau `méthode x chemin -> statut + corps` via `httptest`, pour `/api`, `/api/`, `/api/u`, `/api/u/`, `/api/u/lights`, `/api/u/lights/`, `/api/u/lights/1`, `/api/u/lights/1/state` (GET, PUT), `/api/u/lights/1/x`, `/api/u/groups`, `POST /api`, `POST /api/`, `/admin/setup` (GET, POST, PUT), `/admin/config` (GET, POST, PUT). Consigner l'existant (par exemple chemin inconnu : 200 corps vide).
- [ ] **Routes `/api`** : `Mux()` enregistre `POST /api` et `POST /api/{$}` (enregistrement), `/api` et `/api/{$}` (état complet), `/api/{user}` et `/api/{user}/{$}` (état complet), `/api/{user}/lights[/]`, `/api/{user}/lights/{id}[/]`, `/api/{user}/lights/{id}/state`, plus `/api/` en capture pour reproduire la réponse actuelle des chemins inconnus (200 vide). `handleAPI` disparaît ; les handlers lisent `r.PathValue("id")`. Plus de `switch` ni de `else if`. `handleSetLightState` garde son contrôle PUT.
- [ ] **Setup et config** : `GET /admin/setup`, `POST /admin/setup` derrière un middleware `withSetupGuard` (déjà configuré : 403 + purge du limiteur ; limiteur de débit par IP) ; `GET /admin/config` -> `handleConfigGet`, `POST /admin/config` -> `handleConfigPost`. Les autres méthodes répondent 405 (changement de comportement n°1 : mettre à jour la caractérisation pour ces seules lignes, justifier dans le commit).
- [ ] **Statuts du test-action** : `test_action_status.go` définit `testActionStatuses` (table `{err error; status int}` : `ErrInvalidTestRequest` 400, `ErrNoOpAction` 409, `ErrTooManyRequests` 429) et `statusForTestError(err) int` (parcours `errors.Is`, repli 502, qui couvre `HAAPIError` et les erreurs réseau comme aujourd'hui). Tests de la table avant le handler.
- [ ] **JS des logs** : remplacer `if / else if / else` de `render()` par une fonction `message()` à retours anticipés. Les tests de la page (`logs_handlers_test.go`, `admin_page_test.go`) restent verts ; ajouter une assertion « la page ne contient pas `else if` » n'est pas nécessaire, le scan la porte.
- [ ] `go test -race -tags e2e ./...` vert ; scan : `ocp` = 0, total attendu 112.
- [ ] Commit : `Route Hue API and admin by ServeMux patterns, tables for error statuses`.

---

### Task 4: Demeter, violations de code (4 findings)

`bridge.go` : `GetTranslator(t).ToHA/ToHue/GetMetadata`.

**Files:**
- Modify: `internal/domain/translator/translator_factory.go` : l'interface devient

```go
type TranslatorFactory interface {
    ToHue(mappingType model.MappingType, haState model.HAEntityState, vd *model.VirtualDevice) *model.DeviceState
    ToHA(mappingType model.MappingType, hueState *model.DeviceState, vd *model.VirtualDevice) model.HomeAssistantCommand
    Metadata(mappingType model.MappingType) model.HueMetadata
}
```

- Modify: `internal/domain/translator/factory.go` : `Factory` implémente les 3 méthodes en déléguant à `f.GetTranslator(...)` (conservé, public, utilisé par les tests). Le repli vers `light` pour un type inconnu reste dans `GetTranslator`.
- Modify: `internal/domain/service/bridge.go` : `s.translatorFactory.ToHA(vd.Type, &stateCopy, vd)`, idem `ToHue`, `Metadata` (lignes 99, 149, 220, 265).
- Modify: tests du service qui fournissent une fabrique (adaptation mécanique du double s'il existe) ; `internal/architecture_test.go` doit rester vert (`TestTranslatorStrategiesMustImplementInterface` inchangé).

- [ ] Tests d'abord : `Factory.ToHA/ToHue/Metadata` pour chaque type et pour un type inconnu (repli light), à parité avec `GetTranslator`.
- [ ] Implémenter, adapter les appels, `go test -race -tags e2e ./...` vert.
- [ ] Scan : `demeter-chain` = 22 (framework seulement), total attendu 108.
- [ ] Commit : `Delegate translation to the factory port (Law of Demeter)`.

---

### Task 5: Logger injecté (86 findings : 60 + 26)

Ordre : du centre vers l'extérieur, un commit par couche, tests verts à chaque étape. Pour chaque type :
1. écrire d'abord un test de capture (handler en mémoire : `slog.New(slog.NewTextHandler(&buf, opts{Level: Debug}))`) qui vérifie **un** log sur la décision ciblée et l'**absence** de secret ;
2. ajouter le champ `logger *slog.Logger` et l'argument de constructeur ;
3. remplacer les `slog.*` globaux par `x.logger.*` ;
4. adapter les appels dans les tests existants avec `slog.New(slog.DiscardHandler)`.

**5a. Domaine : translator**
- `LightStrategy`, `CoverStrategy`, `ClimateStrategy`, `CustomStrategy` : `NewXxxStrategy(logger)`, champ `logger`, log `Debug` des surcharges `ActionConfig` appliquées (service, effet, payload) sans les valeurs de payload.
- `Factory` : `NewFactory(logger)` ; `slog.Warn("Unknown mapping type...")` devient `f.logger.Warn(...)`.
- `main.go`, `e2e/helpers_test.go`, `translator_test.go` : constructeurs mis à jour (`&LightStrategy{}` devient `NewLightStrategy(discard)`).

**5b. Domaine : model, service**
- `model.HAAPIError.Error` : construction sans branche (`strings.Join` des segments non vides). Test : `Body` vide, non vide, body se terminant par `: `.
- `model.HAEntityState.IsSupported` supprimée ; nouveau `service.EntityFilter` (`entity_filter.go`, `NewEntityFilter(ignoredDomains, logger)`, `Supports(entityID string) bool`, log `Debug` de l'entité ignorée). `BridgeService.SetIgnoredDomains` / `extractEntities` l'utilisent ; `TestHAEntityState_IsSupported` migre vers `entity_filter_test.go` à l'identique (déclaré dans les contraintes).
- `BridgeService` : `NewBridgeService(..., logger)` ; 7 `slog.*` globaux remplacés ; ajout de logs `Warn` sur `ErrNoOp` et `ErrTooManyRequests`.
- `AuthService` : `NewAuthService(repo, logger)` ; `Warn` sur échec de `Verify` (nom d'utilisateur oui, mot de passe jamais), `Info` sur `CreateCredentials`.

**5c. Adapters de sortie**
- `persistence` : `NewJSONConfigRepository(path, logger)`, `NewJSONAuthRepository(path, logger)` ; logs `Info` de migration d'ancien format, `Warn` de repli sur jeton en clair ou clé par défaut, `Debug` de lecture et d'écriture (chemin seulement). Jamais le jeton ni la clé.
- `homeassistant.Client` : `NewClient(logger)` ; les 6 `slog.*` du client passent par `c.logger` ; `slog.Log(...)` devient `c.logger.Log(...)`.
- `logging.LineRing` : `NewLineRing(maxLines, maxLineBytes, logger)` où le logger est **stdout seul** (spec D3.3) ; `Debug` « log line truncated ». Test : une ligne tronquée produit un log sur le logger stdout et rien dans le tampon. `main.go` construit ce logger avant le handler multi.

**5d. Adapters d'entrée**
- `ssdp.Server` : `NewServer(ip, logger)` ; les 7 `slog.*` passent par `s.logger`.
- `http.Server` : `NewServer(..., ip, logger)` ; `loggingMiddleware` utilise `s.logger` ; ajout de logs `Warn` sur 401 et 403 d'authentification (méthode, chemin, IP, jamais les identifiants), `Warn` sur limite de débit du setup, `Info` à la création des identifiants, `Error` sur les 500 de `handleConfig` / `handleHAEntities`.

**5e. Composition**
- `cmd/bridge/main.go` : `logger := slog.New(handler)` construit une fois, passé à tous les constructeurs et à `getLocalIP(logger, preferred)` ; suppression de `slog.SetDefault` et de tout `slog.*` global.

- [ ] Réaliser 5a à 5e dans l'ordre, un commit chacun (`Inject logger into ...`).
- [ ] `grep -rn 'slog\.\(Debug\|Info\|Warn\|Error\|Log\)' --include=*.go . | grep -v _test.go` ne retourne plus rien.
- [ ] Scan : `logger-*` = 0, total attendu 22 (demeter framework seulement, plus d'éventuelles nouvelles détections à classer).
- [ ] Vérifier à la main qu'un log d'erreur HA ne contient pas le jeton (test e2e existant sur le tampon + nouvelle assertion).

---

### Task 6: Demeter, chaînes imposées par le framework (`nosemgrep`)

Code intact ; ajout en fin de ligne de `// nosemgrep: demeter-chain <raison>`. Numéros de ligne **avant** refactor, à réactualiser en fin de Task 5.

| Fichier : ligne (état final) | Expression | Raison |
|---|---|---|
| `adapters/input/http/admin_handlers.go:78` | `w.Header().Set(...)` | net/http : en-têtes accessibles uniquement via `ResponseWriter.Header()` |
| `admin_handlers.go:107` | `w.Header().Set(...)` | idem |
| `admin_handlers.go:170` | `r.Header.Get("X-Real-IP")` | net/http : `Request.Header` est un `http.Header` |
| `admin_handlers.go:173` | `r.Header.Get("X-Forwarded-For")` | idem |
| `logs_handlers.go:32, 48, 49` | `w.Header().Set(...) (3)` | net/http |
| `server.go:91, 105` | `w.Header().Set("WWW-Authenticate", ...) (2)` | net/http |
| `server.go:131, 156` | `w.Header().Set("Content-Type", ...) (2)` | net/http |
| `adapters/output/homeassistant/client.go:72` | `req.URL.Redacted()` | net/url : `Request.URL` est un `*url.URL` du framework |
| `client.go:83` | `time.Since(start).Milliseconds()` | time : `Duration.Milliseconds` |
| `client.go:85, 114, 186, 214` | `resp.Body.Close() (4)` | net/http : fermeture obligatoire de `Response.Body` |
| `client.go:108, 179, 180, 208` | `req.Header.Set(...) (4)` | net/http |
| `domain/service/bridge.go:317` | `time.Now().Add(-5 * time.Second)` | time : `Time.Add` |
| `cmd/bridge/main.go:183, 186` | `ip.IsLoopback(), ip.To4() (2)` | net : méthodes de `net.IP` sur l'adresse analysée (ajoutées par Task 2) |

24 occurrences au total (22 prévues + 2 dans `main.go`).

- [x] Ajouter les 24 commentaires, relancer le scan, vérifier que chacun supprime bien sa détection (id court accepté, vérifié sur un cas de test).
- [ ] Toute nouvelle détection apparue en Task 4 ou 5 : si le code du projet est en cause, corriger par délégation ; sinon l'ajouter à cette table et à la PR.
- [ ] Commit : `Annotate framework-imposed call chains for demeter-chain`.

---

### Task 7: Preuve, CI, livraison

- [ ] Scan final : `opengrep scan -f .opengrep/architecture.yml --error --text-output=opengrep-report.txt --json-output=opengrep-report.json . ; echo "exit=$?"` -> `exit=0`, `errors` de `opengrep-report.json` = `[]`. Si `exit` vaut 2 ou 7, ou si `errors` n'est pas vide : s'arrêter et signaler.
- [ ] `go test -race -tags e2e ./...` et `make test` verts ; couverture conforme au `ci.yml`.
- [ ] **CI** : dans `.github/workflows/ci.yml`, job `test`, **juste après** `actions/checkout@v4` et avant `Set up Go`, insérer les deux étapes fournies (installation OpenGrep 1.33.1 et « Règles d'architecture (OpenGrep) »), sans modifier les codes de retour, sans SARIF.
- [ ] Vérifier le YAML (`python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`) et dérouler les étapes localement avec un `GITHUB_STEP_SUMMARY` temporaire.
- [ ] Pousser la branche, ouvrir la PR avec : le résumé initial (155 violations par règle et fichiers les plus touchés), le lien vers ce plan, la liste des `nosemgrep` avec leur raison, la preuve du scan à 0 (`exit=0`, `errors: []`), les limites de règles signalées (spec) et les changements de comportement.
- [ ] Ne pas commiter `opengrep-report.*`.

---

## Écarts constatés à l'exécution

1. **Handlers HTTP scindés** (Task 5d). `logger-missing-in-decision-type` exige le struct et ses méthodes dans le même fichier : les handlers de `Server` répartis dans `admin_handlers.go`, `hue_handlers.go`, `logs_handlers.go` restaient signalés. Ils sont devenus `AdminHandlers`, `HueHandlers` et `LogsHandlers` (logger injecté chacun), embarqués dans `Server`. Comportement HTTP inchangé.
2. **Méthodes `Factory` sans chaîne** (Task 4) : la recherche de stratégie passe par `f.strategies[f.resolve(t)]` pour ne pas créer de nouvelle chaîne Demeter. `Register` et `GetTranslator` sont conservés.
3. **Niveaux de log** : l'absence d'en-tête `Authorization` (défi Basic normal) et la création des identifiants sont en `Debug`, pour ne pas changer le contenu du tampon attendu par `TestAdminLogs_EmptyIsJSONArray`.
4. **Moteur OpenGrep** : une méthode à corps vide fait échouer `logger-missing-in-decision-type` avec une erreur interne (`lack range info for $...BODY`, présente dans `errors`). `handleAPIUnknown` écrit donc explicitement `w.WriteHeader(http.StatusOK)` (même réponse). Limite de la règle à signaler.
5. Tests nouveaux : caractérisation du routage, logs sans secret (mot de passe, jeton, payload), `EntityFilter`, `ParseLevel`, sélection de taille de clé, priorité des attributs, délégation de la fabrique.
