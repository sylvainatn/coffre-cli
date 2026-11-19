# coffre (CLI)

Injecte les variables d'environnement d'un projet Coffre dans votre application, **sans fichier `.env` sur le poste**. Les secrets sont récupérés via HTTPS, gardés en mémoire et transmis uniquement au processus lancé.

## Installation

```bash
npm install -g coffre-cli            # télécharge le binaire adapté à l'OS
brew install coffre-dev/tap/coffre   # macOS / Linux
go install github.com/coffre-dev/coffre-cli/cmd/coffre@latest
```

## Utilisation

```bash
coffre login                           # ouvre le navigateur pour relier la CLI à votre compte
coffre init                            # choisit (ou crée) le projet : crée .coffre.json (sans secret, à commiter)
coffre run -- npm run dev              # lance l'app avec les secrets injectés
```

| Commande | Rôle |
| --- | --- |
| `coffre login [--token cfc_…] [--no-browser]` | Connexion via le navigateur (device flow) ou un jeton existant |
| `coffre logout` | Révoque le jeton côté serveur et l'efface du poste |
| `coffre whoami` | Compte connecté |
| `coffre init [projectId] [-e env] [--create nom] [--push] [--agents]` | Associe le dossier à un projet : choix interactif sans argument, `--create` pour en créer un, `--push` pour y envoyer le `.env` du dossier |
| `coffre run [-e env] [-p projet] [--keep-existing] [--file f] -- <cmd>` | Lance une commande avec les secrets |
| `coffre secrets ls [--reveal]` | Liste les secrets (masqués par défaut) |
| `coffre secrets set K=V …` | Crée / met à jour |
| `coffre secrets delete K` | Supprime |
| `coffre secrets diff [-f .env] [--exit-code]` | Compare un `.env` local à Coffre (valeurs jamais affichées) |
| `coffre secrets pull [-f .env] [--overwrite] [--dry-run]` | Ajoute au `.env` les variables de Coffre qu'il n'a pas |
| `coffre secrets push [-f .env] [--update] [--dry-run] [-y]` | Envoie dans Coffre les variables du `.env` qu'il n'a pas |
| `coffre secrets download` / `export [-o f] [--format …]` | Exporte : `dotenv`, `json`, `yaml`, `docker`, `k8s` |
| `coffre agent setup` | Écrit les instructions pour les agents IA (`AGENTS.md`, skill Claude Code) |
| `coffre agent status` | Agent IA détecté, et ce qu'il a le droit de faire ici |

`--json` est disponible sur `init`, `whoami`, `secrets list`, `diff`, `pull`, `push` et `agent` : résultat lisible par une machine sur la sortie standard, erreurs sur la sortie d'erreur avec un code non nul. Les valeurs des secrets n'y figurent jamais (sauf `list --reveal --json`).

Les environnements acceptent des alias : `dev`, `stg`/`preprod`, `prod`.

### Variables d'environnement

| Variable | Rôle |
| --- | --- |
| `COFFRE_TOKEN` | Jeton à utiliser (CI/CD) à la place de celui enregistré par `login` |
| `COFFRE_API_URL` | URL de l'API (instance auto-hébergée, dev local) |
| `COFFRE_PROJECT` / `COFFRE_ENV` | Remplacent `.coffre.json` |
| `COFFRE_CONFIG` | Chemin du fichier d'identifiants (défaut : dossier de config utilisateur) |

Les identifiants sont stockés dans `~/Library/Application Support/coffre/config.json` (macOS) ou `~/.config/coffre/config.json` (Linux), en permissions `0600`.

### Sans fichier `.env`

L'objectif est de ne plus avoir de `.env` sur le poste :

1. **`coffre run -- <commande>`** suffit dans la plupart des projets. Les outils qui chargent un `.env` (dotenv, Next.js, Vite, Prisma, Laravel, Symfony…) n'écrasent pas une variable déjà définie : le fichier peut ne pas exister.
2. **Un outil exige un fichier** (`env_file` de Docker Compose, code qui lit `.env` lui-même) : `coffre run --file .env -- docker compose up` écrit le fichier (droits `0600`) pour la durée de la commande et le supprime à la fin, y compris sur Ctrl+C ou `SIGTERM`. Il refuse d'écraser un fichier existant.
3. **En dernier recours** (configuration de lancement de l'IDE…) : `coffre secrets pull` ou `export -o .env`.

Pour Docker Compose, on peut aussi se passer de fichier en listant les variables sans valeur (`environment: [DATABASE_URL]`) et en lançant `coffre run -- docker compose up`.

### Synchroniser un `.env` existant

| Situation | Commande |
| --- | --- |
| Migrer un projet qui a déjà un `.env` | `coffre init --create "Mon projet" --push` → supprimer le `.env` |
| Un collègue a ajouté une variable | `coffre secrets pull` (ou rien à faire avec `coffre run`) |
| J'ai ajouté une variable en local | `coffre secrets push` |
| Pourquoi ça marche chez moi et pas en staging ? | `coffre secrets diff --env staging` |
| Docker, Kubernetes | `coffre secrets export --format docker` / `--format k8s --name mon-app` |

Règles communes :

- **Rien n'est jamais supprimé**, ni du fichier ni de Coffre.
- **`pull`** ajoute les variables manquantes en fin de fichier (commentaires, ordre et valeurs locales conservés). Une valeur locale différente n'est remplacée qu'avec `--overwrite`, ou après confirmation variable par variable dans un terminal.
- **`push`** n'écrase jamais une variable existante, même créée entre-temps par un collègue (l'API vérifie et écrit dans une seule transaction). `--update` envoie aussi les valeurs différentes ; l'ancienne valeur reste dans l'historique. En staging et en production, une confirmation est demandée (`--yes` en CI).
- **Tout fichier écrit** (`pull`, `export -o`, `run --file`) l'est avec des droits `0600`, de façon atomique, et est ajouté au `.gitignore` du dépôt s'il n'est pas déjà ignoré.

### Agents de code IA (Claude Code, Codex, Cursor…)

Les agents ont le réflexe d'écrire les variables dans un `.env`. Deux mécanismes les en empêchent :

**1. Les instructions** : `coffre agent setup` (ou `coffre init --agents`, ou la question posée à la fin de `coffre init`) écrit dans le dossier du projet :

| Fichier | Lu par |
| --- | --- |
| `AGENTS.md` : une section Coffre, entre marqueurs, sans toucher au reste du fichier | Codex, Cursor, Copilot, Gemini… |
| `.claude/skills/coffre/SKILL.md` | Claude Code, dès qu'une tâche touche aux variables d'environnement |
| `CLAUDE.md` : un rappel, seulement si le fichier existe déjà | Claude Code |

Ces fichiers ne contiennent aucun secret : commitez-les pour que toute l'équipe et ses agents en profitent. Relancer la commande les met à jour.

**2. Les garde-fous de la CLI** : quand elle détecte un agent (variables `CLAUDECODE`, `CODEX_SANDBOX`, `CURSOR_AGENT`, `GEMINI_CLI`, `AI_AGENT`, ou `COFFRE_AGENT=1` pour tout autre agent) :

- `secrets list --reveal`, `secrets download` / `export` et `secrets pull` sont refusés : les valeurs n'arrivent pas dans la conversation du modèle et aucun `.env` n'est recréé ;
- l'écriture (`set`, `delete`, `push`) n'est autorisée que dans `development`, sauf si `.coffre.json` en autorise d'autres :

```json
{ "projectId": "proj_xxx", "environment": "development", "agents": { "write": ["development", "staging"] } }
```

Ces garde-fous évitent les erreurs et les fuites accidentelles. Ils ne remplacent pas les droits du compte : un agent qui dispose du jeton peut toujours lancer `coffre run -- sh -c 'echo $X'`. Pour un agent autonome, utilisez un compte ou un rôle limité (lecteur, accès restreint par environnement).

### Comportement de `run`

- Unix : `coffre` est **remplacé** par la commande (`execve`) : signaux et code de sortie sont ceux de votre application, sans processus intermédiaire.
- Windows, et `--file` sur toutes les plateformes : la commande est lancée en processus enfant (coffre doit lui survivre pour supprimer le fichier). Son code de sortie est propagé ; `SIGTERM` et `SIGHUP` lui sont relayés.
- Par défaut, les secrets écrasent les variables déjà présentes dans le shell (`--keep-existing` pour l'inverse).

## Développement

```bash
make build                          # ./bin/coffre
make test
COFFRE_API_URL=http://localhost:4000 ./bin/coffre login
```

L'URL d'API par défaut est injectée à la compilation (`-X main.defaultAPIURL=…`, voir `.goreleaser.yaml`) ; en build local elle vaut `http://localhost:4000`.

## Publier une version

1. Créer le dépôt `coffre-dev/homebrew-tap` et les secrets GitHub `HOMEBREW_TAP_TOKEN` et `NPM_TOKEN`.
2. (Optionnel) Modifier `defaultAPIURL` dans `.goreleaser.yaml` (actuellement `https://api.sylvainanton.fr`).
3. `git tag v0.1.0 && git push --tags` : GoReleaser publie les binaires et la formule Homebrew, puis le wrapper npm est publié.
