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
coffre init proj_12345 --env development   # crée .coffre.json (sans secret, à commiter)
coffre run -- npm run dev              # lance l'app avec les secrets injectés
```

| Commande | Rôle |
| --- | --- |
| `coffre login [--token cfc_…] [--no-browser]` | Connexion via le navigateur (device flow) ou un jeton existant |
| `coffre logout` | Révoque le jeton côté serveur et l'efface du poste |
| `coffre whoami` | Compte connecté |
| `coffre init <projectId> [-e env]` | Associe le dossier à un projet |
| `coffre run [-e env] [-p projet] [--keep-existing] -- <cmd>` | Lance une commande avec les secrets |
| `coffre secrets ls [--reveal]` | Liste les secrets (masqués par défaut) |
| `coffre secrets set K=V …` | Crée / met à jour |
| `coffre secrets delete K` | Supprime |
| `coffre secrets download` | Sortie au format `.env` (Docker, CI) |

Les environnements acceptent des alias : `dev`, `stg`/`preprod`, `prod`.

### Variables d'environnement

| Variable | Rôle |
| --- | --- |
| `COFFRE_TOKEN` | Jeton à utiliser (CI/CD) à la place de celui enregistré par `login` |
| `COFFRE_API_URL` | URL de l'API (instance auto-hébergée, dev local) |
| `COFFRE_PROJECT` / `COFFRE_ENV` | Remplacent `.coffre.json` |
| `COFFRE_CONFIG` | Chemin du fichier d'identifiants (défaut : dossier de config utilisateur) |

Les identifiants sont stockés dans `~/Library/Application Support/coffre/config.json` (macOS) ou `~/.config/coffre/config.json` (Linux), en permissions `0600`.

### Comportement de `run`

- Unix : `coffre` est **remplacé** par la commande (`execve`) : signaux et code de sortie sont ceux de votre application, sans processus intermédiaire.
- Windows : la commande est lancée en processus enfant et son code de sortie est propagé.
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
2. Adapter `defaultAPIURL` dans `.goreleaser.yaml`.
3. `git tag v0.1.0 && git push --tags` : GoReleaser publie les binaires et la formule Homebrew, puis le wrapper npm est publié.
