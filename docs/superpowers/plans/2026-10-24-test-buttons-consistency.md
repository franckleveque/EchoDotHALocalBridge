# Cohérence des boutons de test Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Les boutons de test du formulaire et du tableau agissent sur l'appareil tel qu'affiché (même non enregistré), et annoncent un succès seulement si Home Assistant a accepté l'appel.

**Architecture:** `TestDeviceAction` devient synchrone et renvoie l'erreur HA (`HAAPIError`). Le handler `/admin/test-action` convertit l'état comme le fait l'API Hue (`bri` implique `UpdatedByBri`) et répond 502 avec le corps de l'erreur HA. Côté page admin, une fonction JS unique envoie le test, et le tableau l'utilise avec l'appareil de la ligne ; les lignes non enregistrées sont signalées.

**Tech Stack:** Go 1.25, JavaScript inline dans `admin_handlers.go`, testify, e2e avec tag `e2e`, Playwright (vérification jetable).

**Spec:** `docs/superpowers/specs/2026-10-24-admin-diagnostics-and-test-consistency-design.md`

**Prérequis:** le plan `2026-10-24-logging-foundation-and-admin-logs.md` est terminé (`model.HAAPIError` existe, les logs permettent de diagnostiquer).

## Global Constraints

- L'API Hue vue par Alexa reste asynchrone (`UpdateDeviceState` inchangé). Seul `TestDeviceAction` devient synchrone, timeout 10 s.
- Hexagonal et `make test` comme dans le plan 1 ; couverture > 80 % dans `internal/domain/service`.
- Statuts de `/admin/test-action` : erreur HA (`*model.HAAPIError`), erreur réseau ou timeout en `502` avec le corps texte de l'erreur (`HA API error: <code>: <corps>` pour HA) ; `model.ErrTooManyRequests` en `429` ; `model.ErrInvalidTestRequest` ou corps invalide en `400` ; le reste en `500`.
- `model.ErrTooManyRequests` et `model.ErrInvalidTestRequest` sont des erreurs sentinelles du domaine (`errors.Is`), créées dans la tâche 1 de ce plan (`internal/domain/model/hacall.go`) avec les messages `too many concurrent requests, please try again later` et `invalid test request`. Le message de concurrence existant est conservé.
- L'ancienne route `PUT /api/admin/lights/{id}/state` n'est pas supprimée.
- Message de succès : `Home Assistant accepted the call on <entity_id>`. Interface en anglais. Pas de tiret cadratin.
- Une ligne est « non enregistrée » si elle n'a pas de `hue_id` ou si son JSON diffère de la version enregistrée du même `hue_id`.

## Review Focus

- « Dim 50% » du formulaire : doit produire une action allumée avec luminosité, pas un `turn_off`.
- Corps de requête sans `virtual_device` ou sans `state_update` : 400, pas de panique.
- HA lent (> 10 s) : le bouton reçoit une erreur claire, la requête ne reste pas pendue.
- Ligne supprimée ou réordonnée dans le tableau : le repère « non enregistré » suit le `hue_id`, pas l'index.
- Après **Save Configuration** réussi : plus aucune ligne marquée non enregistrée.

---

### Task 1: TestDeviceAction synchrone avec erreur HA et règle de luminosité

**Files:**
- Modify: `internal/domain/service/bridge.go:60-88` (`TestDeviceAction`), `internal/domain/model/hacall.go` (ajout des deux sentinelles)
- Test: `internal/domain/service/bridge_test.go` (remplacer les cas async autour des l.489-580)

**Interfaces:**
- Produces: `TestDeviceAction(ctx, vd *model.VirtualDevice, state *model.DeviceState) error` : renvoie `ports`-level error du port HA tel quel (donc `*model.HAAPIError` via `errors.As`) ; `vd == nil` ou `state == nil` → `model.ErrInvalidTestRequest`. Si `state.UpdatedByBri`, l'état envoyé au traducteur a `On=true` (comme `UpdateDeviceState`). L'état passé en argument n'est pas modifié. Si le sémaphore est plein : `model.ErrTooManyRequests` (message `too many concurrent requests, please try again later`).

- [ ] **Step 1: Tests qui échouent** (mock `MockHAPort` existant) :
  - `TestTestDeviceAction_ReturnsHAError` : le mock `SetState` renvoie `&model.HAAPIError{Status:400, Body:"nope"}` → l'appel retourne cette erreur (`errors.As`), et `SetState` a été appelé avant le retour.
  - `TestTestDeviceAction_BriImpliesOn` : `state{Bri:127, UpdatedByBri:true, On:false}` → la commande transmise au mock est celle d'un `turn_on` avec `brightness` 127 (type light), et `state.On` reste `false` chez l'appelant.
  - `TestTestDeviceAction_NilInputs` : `nil` pour l'un ou l'autre → erreur, aucun appel au mock.
  - `TestTestDeviceAction_UsesFormEntity` : `vd.EntityID = "switch.new"` → `device.ExternalID == "switch.new"` dans l'appel `SetState`.
  - Adapter le test de limite de concurrence existant (l.575) : le sémaphore plein renvoie toujours l'erreur de concurrence.
- [ ] **Step 2: Lancer** `go test ./internal/domain/service -run TestDeviceAction` → FAIL.
- [ ] **Step 3: Implémenter** : copie de l'état, application de la règle luminosité, acquisition non bloquante du sémaphore (libération par `defer`), `context.WithTimeout(ctx, 10*time.Second)`, appel direct à `haPort.SetState`, retour de l'erreur sans l'envelopper.
- [ ] **Step 4: Lancer** `go test -race ./internal/domain/service` et `make coverage` (service > 80 %) → PASS.
- [ ] **Step 5: Commit** `fix: make admin test action synchronous and return HA errors`

### Task 2: Handler /admin/test-action

**Files:**
- Modify: `internal/adapters/input/http/admin_handlers.go:147-172` (`handleAdminTestAction`), `internal/adapters/input/http/hue_handlers.go:~138-155`
- Create: `internal/adapters/input/http/state_parse.go`
- Test: `internal/adapters/input/http/state_parse_test.go`, `internal/e2e/admin_api_test.go`

**Interfaces:**
- Produces: `parseStateUpdate(raw map[string]any) *model.DeviceState` : `on` (bool) → `On` ; `bri` (nombre) → `Bri` et `UpdatedByBri=true`. Utilisée par le handler Hue (remplace le bloc dupliqué) et par `handleAdminTestAction` (le champ `state_update` est décodé en `map[string]any`).

- [ ] **Step 1: Tests qui échouent** :
  - `TestParseStateUpdate` : `{"on":true}` → On ; `{"bri":127}` → Bri 127 et UpdatedByBri ; `{}` → état zéro.
  - e2e `TestAdminTestAction_HAError502` : faux HA répondant 400 + corps `{"message":"Service not found."}` → réponse `502`, corps contenant `400` et `Service not found.`.
  - e2e `TestAdminTestAction_Success200AfterHAAccepts` : faux HA 200 → `200` et `ha.callCount()==1` dès le retour (pas d'attente).
  - e2e `TestAdminTestAction_DimSendsTurnOn` : `{"bri":127}` sur une entité `light.x` → dernier appel HA `light/turn_on` avec `brightness` 127.
  - e2e `TestAdminTestAction_BadBody400` : JSON sans `virtual_device` → `400`.
- [ ] **Step 2: Lancer** `go test ./internal/adapters/input/http` et `go test -tags e2e ./internal/e2e -run AdminTestAction` → FAIL.
- [ ] **Step 3: Implémenter** `parseStateUpdate`, refactorer le handler Hue pour l'utiliser, adapter le handler admin avec la table de statuts des Global Constraints (`errors.As` sur `*model.HAAPIError` → 502 ; `errors.Is` pour les deux sentinelles ; toute autre erreur issue de `SetState` ou du timeout → 502).
- [ ] **Step 4: Lancer** `make test && go test -race -tags e2e ./...` → PASS (les tests Hue existants garantissent que l'API Alexa n'a pas changé).
- [ ] **Step 5: Commit** `fix: return HA errors from /admin/test-action and share state parsing`

### Task 3: Page admin, boutons cohérents et repère non enregistré

**Files:**
- Modify: `internal/adapters/input/http/admin_handlers.go` (JS inline : `testAction` l.433, `testCurrentForm` l.450, `applyDeviceChanges`, `renderDevices` l.~405, chargement de la config, `saveAll`)
- Test: `internal/adapters/input/http/admin_page_test.go` (nouveau)

**Interfaces:**
- Produces (JS, dans la page) :
  - `readDeviceForm()` → objet appareil ou `null` (alerte si JSON invalide) ; utilisé par `applyDeviceChanges` et `testCurrentForm` (supprime la duplication).
  - `sendTest(vd, stateUpdate)` → POST `/admin/test-action` ; succès : `showStatus('Home Assistant accepted the call on ' + vd.entity_id)` ; échec : `showStatus('Error: ' + texte de la réponse)`.
  - `testRow(index, stateUpdate)` → `sendTest(config.virtual_devices[index], stateUpdate)`.
  - `savedByHueId` : `Map<hue_id, JSON string>` rempli au chargement de la config et après un `saveAll` réussi ; `isUnsaved(vd)` selon la règle des Global Constraints.
  - `renderDevices` : boutons On / Off / Dim 50% toujours présents (plus de « Save config first »), appelant `testRow(index, ...)` ; repère `unsaved` à côté du nom si `isUnsaved`.

- [ ] **Step 1: Test qui échoue** `TestAdminPage_Markers` (GET `/admin` servi par le handler) : le HTML contient `function testRow(`, `function sendTest(`, `function readDeviceForm(`, `isUnsaved(`, et ne contient plus `Save config first` ni `/api/admin/lights/' + hueId`. Limite assumée : ce test ne prouve pas le comportement du navigateur, d'où l'étape 5.
- [ ] **Step 2: Lancer** `go test ./internal/adapters/input/http -run AdminPage` → FAIL.
- [ ] **Step 3: Implémenter** les fonctions JS ci-dessus ; `testCurrentForm(state)` devient `const vd = readDeviceForm(); if (vd) sendTest(vd, state)`.
- [ ] **Step 4: Lancer** `make test` → PASS.
- [ ] **Step 5: Vérification dans un vrai navigateur** (Playwright préinstallé, script jetable hors du dépôt, `/tmp/claude-0/.../scratchpad`) : démarrer le binaire avec un faux HA enregistrant les appels, puis dérouler le scénario du bug : Edit d'un appareil → choisir `switch.intlumieresalleamanger` → **Apply** sans Save → cliquer **On** dans le tableau → le faux HA doit voir `switch/turn_on` sur la nouvelle entité et la ligne doit porter le repère non enregistré ; refaire avec un faux HA en 400 → la barre de statut affiche l'erreur et son corps ; **Save Configuration** → le repère disparaît ; cliquer **Dim 50%** dans le formulaire → le faux HA voit `turn_on` avec luminosité. Rapporter ce qui est vérifié et ce qui ne l'est pas (HA réel).
- [ ] **Step 6: Commit** `fix: admin test buttons act on displayed device and flag unsaved rows`
