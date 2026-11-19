// Package agentfiles écrit les instructions destinées aux agents de code IA :
// une section dans AGENTS.md (Codex, Cursor, Copilot, Gemini…), un skill Claude Code,
// et un rappel dans CLAUDE.md s'il existe.
package agentfiles

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	startMarker = "<!-- coffre:start (section gérée par `coffre agent setup`) -->"
	endMarker   = "<!-- coffre:end -->"
)

// Params : informations du projet reprises dans les instructions.
type Params struct {
	ProjectName  string
	DefaultEnv   string
	WritableEnvs []string
}

// Written : fichier écrit, et ce qui lui est arrivé.
type Written struct {
	Path   string `json:"path"`
	Action string `json:"action"` // created | updated | unchanged
}

// Setup écrit les instructions dans le dossier du projet (celui de .coffre.json).
func Setup(dir string, p Params) ([]Written, error) {
	var out []Written
	add := func(path string, action string, err error) error {
		if err != nil {
			return err
		}
		out = append(out, Written{Path: path, Action: action})
		return nil
	}

	agents := filepath.Join(dir, "AGENTS.md")
	action, err := upsertSection(agents, section(p), true)
	if err := add(agents, action, err); err != nil {
		return nil, err
	}

	skill := filepath.Join(dir, ".claude", "skills", "coffre", "SKILL.md")
	action, err = writeFile(skill, skillFile(p))
	if err := add(skill, action, err); err != nil {
		return nil, err
	}

	// CLAUDE.md : on ne le crée pas, mais s'il existe on y ajoute un rappel
	claude := filepath.Join(dir, "CLAUDE.md")
	if _, err := os.Stat(claude); err == nil {
		action, err = upsertSection(claude, claudeReminder(), false)
		if err := add(claude, action, err); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func envList(envs []string) string {
	quoted := make([]string, len(envs))
	for i, e := range envs {
		quoted[i] = "`" + e + "`"
	}
	return strings.Join(quoted, ", ")
}

// section : instructions communes à tous les agents (AGENTS.md).
func section(p Params) string {
	return fmt.Sprintf(`## Variables d'environnement : Coffre

Les variables d'environnement et secrets de ce projet sont gérés avec Coffre (projet « %s », environnement par défaut `+"`%s`"+`).
**Il n'y a pas de fichier .env : n'en crée pas et n'en modifie pas** (.env, .env.local, .env.development…).

- Lancer l'application, les tests ou un script : `+"`coffre run -- <commande>`"+` (ex : `+"`coffre run -- npm run dev`"+`).
- Variables disponibles : `+"`coffre secrets list --json`"+` (noms seulement, jamais les valeurs).
- Nouvelle variable nécessaire : `+"`coffre secrets set NOM=valeur`"+`. Autorisé uniquement en %s.
- Valeur inconnue (vraie clé d'API, mot de passe…) : crée la variable vide (`+"`coffre secrets set NOM=`"+`) et demande à l'utilisateur de la renseigner dans le dashboard Coffre. N'invente pas de valeur.
- Un fichier .env existe déjà : `+"`coffre secrets push --json`"+` pour envoyer ses variables (sans écraser), puis propose à l'utilisateur de supprimer le fichier.
- Un outil exige vraiment un fichier .env (env_file de Docker Compose…) : `+"`coffre run --file .env -- <commande>`"+` (fichier supprimé à la fin).
- N'affiche jamais les valeurs des secrets : pas de `+"`--reveal`"+`, pas de `+"`secrets download`"+`, pas de `+"`echo $SECRET`"+`. Elles finiraient dans la conversation.
- N'écris jamais en staging ni en production : demande à l'utilisateur.
- Documente chaque nouvelle variable (nom et rôle) dans le README ou le .env.example s'il existe, sans valeur réelle.
`, p.ProjectName, p.DefaultEnv, envList(p.WritableEnvs))
}

func skillFile(p Params) string {
	return `---
name: coffre
description: Variables d'environnement et secrets de ce projet, gérés avec Coffre (pas de fichier .env). À utiliser dès qu'il faut lire, ajouter ou modifier une variable d'environnement, une clé d'API ou un secret, créer ou éditer un fichier .env, ou lancer l'application, les tests ou un script qui en a besoin.
---

# Coffre : variables d'environnement sans fichier .env

` + section(p) + `
## Commandes utiles

| Besoin | Commande |
| --- | --- |
| Lancer l'app avec les secrets | ` + "`coffre run -- npm run dev`" + ` |
| Lister les variables (noms) | ` + "`coffre secrets list --json`" + ` |
| Ajouter / modifier une variable | ` + "`coffre secrets set NOM=valeur`" + ` |
| Comparer un .env existant à Coffre | ` + "`coffre secrets diff --json`" + ` |
| Envoyer un .env existant sans écraser | ` + "`coffre secrets push --json`" + ` |
| Vérifier le compte connecté | ` + "`coffre whoami --json`" + ` |

Toutes ces commandes acceptent ` + "`--env <environnement>`" + ` et renvoient un code de sortie non nul en cas d'erreur (message sur la sortie d'erreur).

## En cas d'erreur

- « non connecté » : demande à l'utilisateur de lancer ` + "`coffre login`" + ` dans son terminal (connexion par le navigateur).
- « aucun projet configuré » : demande à l'utilisateur de lancer ` + "`coffre init`" + `.
- « refusée aux agents IA » / « désactivé pour les agents IA » : c'est voulu. N'essaie pas de contourner : explique à l'utilisateur ce qu'il doit faire lui-même.
- 403 « lecture seule » : le compte n'a pas le droit d'écrire dans cet environnement ; demande à un propriétaire du projet.
`
}

func claudeReminder() string {
	return "Variables d'environnement : ce projet utilise Coffre, sans fichier .env. Voir la section Coffre de AGENTS.md (ou le skill `coffre`) avant de toucher à une variable d'environnement.\n"
}

// upsertSection remplace la section entre marqueurs, ou l'ajoute à la fin du fichier.
func upsertSection(path, body string, create bool) (string, error) {
	block := startMarker + "\n" + body + endMarker + "\n"
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if !create {
			return "", err
		}
		return "created", os.WriteFile(path, []byte(block), 0o644)
	}
	if err != nil {
		return "", err
	}
	content := string(data)
	start := strings.Index(content, startMarker)
	end := strings.Index(content, endMarker)
	var next string
	switch {
	case start >= 0 && end > start:
		next = content[:start] + block + strings.TrimPrefix(content[end+len(endMarker):], "\n")
	default:
		sep := "\n"
		if content != "" && !strings.HasSuffix(content, "\n") {
			sep = "\n\n"
		} else if content == "" {
			sep = ""
		}
		next = content + sep + block
	}
	if next == content {
		return "unchanged", nil
	}
	return "updated", os.WriteFile(path, []byte(next), 0o644)
}

func writeFile(path, content string) (string, error) {
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == content {
		return "unchanged", nil
	}
	action := "updated"
	if errors.Is(err, os.ErrNotExist) {
		action = "created"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return action, os.WriteFile(path, []byte(content), 0o644)
}
