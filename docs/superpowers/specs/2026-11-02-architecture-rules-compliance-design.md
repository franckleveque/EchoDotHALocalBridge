# Conformité aux règles d'architecture OpenGrep : design

## Contexte

Le fichier `.opengrep/architecture.yml` (22 règles, OpenGrep 1.33.1, fourni tel quel et non modifiable) encode les règles d'architecture du dépôt : hexagonal, DDD, SOLID, Loi de Demeter, logger injecté. Le premier scan (`opengrep scan -f .opengrep/architecture.yml --error .`) remonte **155 violations**, `errors` vide, exit 1.

| Règle | Violations | Traitement |
|---|---:|---|
| `logger-missing-in-decision-type` | 60 | injecter un `*slog.Logger` par constructeur |
| `one-type-per-file` | 27 | scinder les fichiers (déplacements purs) |
| `demeter-chain` | 26 | 4 corrections de code, 22 `nosemgrep` motivés |
| `logger-global-not-injected` | 26 | remplacer `slog.*` par le logger injecté |
| `ocp-no-switch-or-else-if` | 16 | tables, itération, routes `ServeMux` |
| Hexagonal, DDD, DIP, ISP, LSP, `no-utility-package`, ports | 0 | rien à corriger, la règle reste un garde-fou |

## Objectifs

1. `opengrep scan ... --error` termine avec `exit=0` et `errors` vide.
2. Aucun changement de comportement observable (HTTP, SSDP, persistance, appels HA), sauf les points listés dans « Changements de comportement ».
3. Tous les tests existants restent verts. Seuls les appels de constructeurs sont adaptés mécaniquement (argument logger en plus, `slog.New(slog.DiscardHandler)` dans les tests), jamais les assertions.
4. La règle n'est jamais contournée : aucun `nosemgrep` hors `demeter-chain` framework, fichier de règles intact.

## Décisions de conception

### D1. Un type par fichier
Déplacements purs, un fichier par type, nommé en snake_case d'après le type. Les alias `type X = Y` ne comptent pas pour la règle et restent groupés.

### D2. Logger injecté
- Chaque struct portant du code décisionnel (`if`, `for`, `switch`, `&&`, `||`) reçoit un champ `logger *slog.Logger` renseigné par `NewXxx(..., logger)`.
- Le logger est utilisé sur les décisions utiles : refus, repli, erreur, saut d'élément, migration. Niveaux : `Debug` pour le détail, `Info` pour les événements de cycle de vie, `Warn` pour un repli ou un refus, `Error` pour un échec.
- Ne jamais journaliser un secret : jeton HA, mot de passe, en-tête `Authorization`, clé de chiffrement.
- `cmd/bridge/main.go` construit le logger (handler stdout + tampon) et le passe à tous les constructeurs. `slog.SetDefault` et les appels `slog.*` globaux disparaissent, y compris dans `main.go` et `getLocalIP` (qui reçoit le logger en paramètre).
- Les tests injectent `slog.New(slog.DiscardHandler)`, ou un handler de capture quand la sortie est vérifiée.

### D3. Cas où la règle logger n'a pas de sens naturel (à valider)
La règle ne connaît pas la nature du type. Trois cas méritent un arbitrage, sans toucher à la règle :

1. **`model.HAEntityState.IsSupported`** : une donnée désérialisée n'a pas à porter un logger. La décision (entité ignorée par domaine) est déplacée vers le domaine service, dans un type `EntityFilter` (logger injecté, journalise les entités ignorées en `Debug`). `BridgeService.SetIgnoredDomains` et `extractEntities` s'appuient dessus.
2. **`model.HAAPIError.Error`** : une erreur est une valeur. Le `if e.Body == ""` est remplacé par une construction sans branche (jointure des segments non vides avec `": "`), strictement équivalente.
3. **`logging.LineRing`** : c'est le puits des logs. Lui injecter le logger principal créerait une récursion (écrire dans le tampon déclenche un log qui écrit dans le tampon) et un interblocage sur son mutex. Il reçoit un logger **stdout seul** (sans le tampon), utilisé pour signaler en `Debug` la troncature d'une ligne. Signalé comme limite de la règle : elle suppose que tout type décisionnel peut se journaliser lui-même.

### D4. OCP : switch et `else if`
Aucun nouveau cas métier ne doit modifier un site de décision :

| Site | Remplacement |
|---|---|
| `main.go` `switch LOG_LEVEL` | `logging.ParseLevel(string) slog.Level`, table `map[string]slog.Level` |
| `main.go` `switch addr.(type)` | extraction de l'IP depuis `addr.String()` (`net.ParseIP` sur la partie avant `/`), sans switch de type |
| `json_repo.go` `if/else if` sur la longueur de clé | itération sur les tailles AES `[32, 24, 16]`, un seul avertissement paramétré |
| `custom.go` `else if` sur les attributs HA | itération sur `[brightness, current_position, temperature, value]` |
| `custom.go` `switch domain` | table `domain -> {service, paramètre, conversion}` (type `DomainMapping`) |
| `bridge.go` `else if` sur `friendly_name` / `name` | itération sur `[friendly_name, name]` |
| `admin_handlers.go` `if GET / else if POST` (setup, config) | patterns `ServeMux` avec méthode (`GET /admin/config`, `POST /admin/config`), garde de setup en middleware |
| `admin_handlers.go` `switch` d'erreurs de test | table `[]{erreur, statut}` parcourue avec `errors.Is`, repli 502 |
| `hue_handlers.go` `switch` + `else if` de sous-chemin | patterns `ServeMux` à jokers (`/api/{user}/lights/{id}/state`) et capture `/api/` conservée |
| `logs_handlers.go` JS `else if` | fonction JS à retours anticipés (la règle lit le texte du `.go`) |

### D5. Demeter
- **Violations de code (4)** : `s.translatorFactory.GetTranslator(t).ToHA(...)` navigue dans l'objet renvoyé par la fabrique. Application de *tell, don't ask* : le port `TranslatorFactory` expose `ToHue`, `ToHA` et `Metadata` (3 méthodes, ISP respecté) qui délèguent à la stratégie. `GetTranslator` reste sur `translator.Factory` (usage interne et tests), hors du port.
- **Appels imposés (22)** : `net/http` (`Header().Set`, `Header.Get`, `Body.Close`), `net/url` (`URL.Redacted`), `time` (`time.Since(...).Milliseconds()`, `time.Now().Add`). Code intact, `// nosemgrep: demeter-chain <raison>` sur la ligne. Liste complète dans le plan.
- Le refactor peut faire apparaître d'autres détections. Elles sont classées avec les mêmes critères et la liste finale figure dans la PR.

## Changements de comportement (à valider)

1. `/admin/setup` et `/admin/config` : une méthode autre que GET/POST répond désormais **405** (avec `Allow`) au lieu de 200 vide.
2. Messages d'avertissement de `HUE_ENCRYPTION_KEY` : un message paramétré remplace les trois variantes. Le niveau (`Warn`) et les clés effectives sont inchangés.
3. Nouveaux logs (`Warn` sur 401/403, `Info` sur création des identifiants, etc.). Aucun secret, pas de changement de format.

Tout le reste est un refactor à comportement constant, protégé par des tests de caractérisation écrits **avant** la modification.

## Hors périmètre

- Modification de `architecture.yml`.
- Ajout de fonctionnalités, upload SARIF, nouvelle dépendance.
- Dédoublonnage du code `ActionConfig` entre stratégies (réfacto utile mais sans règle associée).

## Limites connues des règles (signalées, non contournées)

- `demeter-chain` ne connaît pas les types : 22 faux positifs framework.
- `logger-missing-in-decision-type` impose un logger à des types où il n'a pas de sens (D3).
- `one-type-per-file` et les règles logger exigent struct et méthodes dans le même fichier : la découpe des fichiers en tient compte.
- OpenGrep ne scanne que les fichiers suivis par git : tout nouveau fichier doit être `git add` avant un scan local.
