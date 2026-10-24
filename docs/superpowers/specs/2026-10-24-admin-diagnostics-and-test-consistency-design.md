# Diagnostic des appels HA et cohérence des boutons de test (design)

Date : 2026-10-24. Source : `echobridge-bug-reports.md` (bugs 1 et 2). Taille : petit changement, deux parties liées.

## Intention

Quand un appel vers Home Assistant (HA) échoue, l'utilisateur doit pouvoir le voir depuis l'admin, sans SSH. Et l'interface ne doit jamais afficher une chose et en faire une autre : ce que le tableau montre est ce que les boutons testent, et un succès n'est annoncé que si HA a accepté l'appel.

## Constats vérifiés

Vérifiés par un test jetable (supprimé) avec le vrai client HA et un faux HA :
- `POST /admin/test-action` avec `switch.intlumieresalleamanger` (types Light puis Custom) atteint HA sur `/api/services/switch/turn_on` avec la bonne entité.
- Si HA répond 400, l'interface reçoit quand même `200`. L'erreur n'apparaît que dans le log serveur, sans le corps de la réponse HA.

Lus dans le code, non exécutés :
- `applyDeviceChanges` ne modifie que la config en mémoire du navigateur. Les boutons du tableau appellent `PUT /api/admin/lights/{hue_id}/state`, qui agit sur l'appareil enregistré. Après Apply sans Save, le tableau affiche la nouvelle entité mais le bouton agit sur l'ancienne.
- `DeviceState.UpdatedByBri` est `json:"-"`. Via `/admin/test-action`, `{bri:127}` donne `On=false`, donc le bouton « Dim 50% » du formulaire envoie probablement un `turn_off`.
- `CustomStrategy.ToHA` ajoute `"value":0` au payload pour les domaines autres que light, cover, climate. HA peut refuser ce champ en trop.

Non reproduit : la cause exacte de l'absence de réaction du switch dans le bug 2 sur le formulaire. Les logs de la partie A doivent la révéler.

## Objectifs

- A. Page de logs admin : pour chaque appel à HA, URL, payload, code retour, corps de la réponse en cas d'erreur.
- B. Les boutons de test (formulaire et tableau) agissent sur ce qui est affiché, y compris non enregistré. Le succès n'est affiché que si HA a accepté l'appel. Une ligne non enregistrée est signalée.

## Hors périmètre

- Corriger le `"value":0` du Custom (à traiter après lecture des logs).
- Persistance des logs sur disque, filtres avancés, journalisation de `GET /api/states`.
- Modifier l'API Hue vue par Alexa (toujours asynchrone).

## Approches considérées (bug 2)

- **A (choisie)** : les boutons du tableau utilisent `/admin/test-action` avec l'appareil de la ligne telle qu'affichée, plus un repère « non enregistré ».
- B : boutons sur la config enregistrée, entité enregistrée affichée. Cohérent mais on ne peut pas tester avant d'enregistrer.
- C : Apply enregistre automatiquement. Écarté : modifie le pont réel vu par Alexa sans décision explicite.

## Décisions

Choisies par l'utilisateur : approche A pour le bug 2 ; tampon mémoire pour les logs (proposé, non contredit).

Mes choix, à relire :
- Tampon en mémoire, 200 entrées, perdu au redémarrage.
- Journalisation des appels `/api/services/...` (dont effets), pas du polling `GET /api/states`.
- Corps de réponse conservé seulement pour les codes >= 400, tronqué à 2 Ko. Jamais d'en-tête `Authorization`.
- L'endpoint `/admin/test-action` devient synchrone (timeout 10 s). `agents.md` impose l'asynchrone pour l'API Hue ; je l'applique à celle-ci, pas à l'admin.
- Erreur HA renvoyée au navigateur en `502` avec le texte `HA API error: <code>: <corps>`.
- Pages `/admin/logs` (HTML) et `/admin/logs/data` (JSON), derrière `withBasicAuth`. Rendu des valeurs via `textContent` (le corps vient de HA).
- Interface en anglais, comme l'existant.
- Le bug de luminosité (`UpdatedByBri`) est dans le périmètre : faire passer le tableau par `/admin/test-action` le casserait, car il fonctionne aujourd'hui via la route Hue. Correctif : un analyseur d'état partagé (`on`, `bri` impliquant `UpdatedByBri`), et `TestDeviceAction` met `On=true` quand `UpdatedByBri`, comme `UpdateDeviceState`. Le tag JSON du modèle ne change pas (l'API Hue est inchangée).
- Le bouton du tableau envoie l'appareil complet de la ligne tel qu'affiché dans la config du navigateur. L'ancienne route `PUT /api/admin/lights/{id}/state` reste en place (utilisée par les tests e2e), la page ne l'appelle plus. `/admin/test-action` n'actualise pas le cache d'état du pont ; le tableau n'affiche pas d'état on/off, rien à rafraîchir.
- Statuts HTTP de `/admin/test-action` : erreur HA, erreur réseau ou timeout 10 s en `502` ; trop de requêtes concurrentes en `429` ; corps invalide ou `virtual_device` / `state_update` absent en `400` ; le reste en `500`.
- Journal : tous les appels de service HA y passent, y compris ceux déclenchés par Alexa. Échec sans réponse HTTP : entrée avec `Error` et `Status` 0. Le payload enregistré est celui réellement envoyé (donc avec le `"value":0` du Custom), complet, sans rédaction (données de service définies par l'utilisateur ; le jeton n'y figure jamais).
- Repère « non enregistré » : par ligne, comparé à la version enregistrée du même `hue_id` ; toute ligne sans `hue_id` est non enregistrée ; il disparaît après un Save réussi.
- Tests : tests Go (service, handlers, e2e) plus une vérification navigateur jetable (Playwright), non commitée.

```mermaid
flowchart LR
  UI[Bouton de test] -->|POST /admin/test-action| H[handler admin]
  H --> S[BridgeService.TestDeviceAction sync]
  S --> C[HA client]
  C -->|enregistre| R[(Ring 200)]
  C -->|HAAPIError code+corps| S
  S --> H -->|200 ou 502 + corps| UI
  L[/admin/logs] -->|GET /admin/logs/data| R
```

## Laissé au développeur

Présentation exacte de la page de logs, libellés du repère « non enregistré », nom des fichiers de test.

## Plans

1. `docs/superpowers/plans/2026-10-24-ha-call-logs.md` (bug 1, à faire en premier : introduit `HAAPIError`).
2. `docs/superpowers/plans/2026-10-24-test-buttons-consistency.md` (bug 2, dépend du plan 1).
