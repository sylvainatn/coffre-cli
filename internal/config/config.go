// Package config gère la configuration globale (identifiants, ~/.config/coffre)
// et la configuration locale d'un dépôt (.coffre.json).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const ProjectFileName = ".coffre.json"

// Global est stocké dans le dossier de configuration utilisateur, en 0600.
type Global struct {
	APIURL string `json:"apiUrl,omitempty"`
	Token  string `json:"token,omitempty"`
	Email  string `json:"email,omitempty"`
}

// Project est le fichier .coffre.json, versionnable : il ne contient aucun secret.
type Project struct {
	ProjectID   string        `json:"projectId"`
	Environment string        `json:"environment"`
	Agents      *AgentsPolicy `json:"agents,omitempty"`
}

// AgentsPolicy : ce que les agents de code IA ont le droit de faire dans ce dépôt.
type AgentsPolicy struct {
	// Environnements où un agent peut écrire (défaut : development).
	Write []string `json:"write,omitempty"`
}

// AgentWritableEnvironments : environnements où un agent peut créer ou modifier des secrets.
func (p *Project) AgentWritableEnvironments() []string {
	if p != nil && p.Agents != nil && len(p.Agents.Write) > 0 {
		return p.Agents.Write
	}
	return []string{"development"}
}

func globalPath() (string, error) {
	if p := os.Getenv("COFFRE_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "coffre", "config.json"), nil
}

func LoadGlobal() (*Global, error) {
	path, err := globalPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Global{}, nil
	}
	if err != nil {
		return nil, err
	}
	var g Global
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("configuration illisible (%s) : %w", path, err)
	}
	return &g, nil
}

func SaveGlobal(g *Global) error {
	path, err := globalPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	// Écriture atomique, fichier lisible uniquement par l'utilisateur
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// FindProject cherche .coffre.json dans le dossier courant puis dans les parents.
func FindProject() (*Project, string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	for {
		path := filepath.Join(dir, ProjectFileName)
		data, err := os.ReadFile(path)
		if err == nil {
			var p Project
			if err := json.Unmarshal(data, &p); err != nil {
				return nil, path, fmt.Errorf("%s invalide : %w", path, err)
			}
			return &p, path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, "", nil
		}
		dir = parent
	}
}

// ReadProject lit .coffre.json dans ce dossier précis (nil s'il n'existe pas).
func ReadProject(dir string) (*Project, error) {
	data, err := os.ReadFile(filepath.Join(dir, ProjectFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("%s invalide : %w", ProjectFileName, err)
	}
	return &p, nil
}

func WriteProject(dir string, p *Project) (string, error) {
	path := filepath.Join(dir, ProjectFileName)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(data, '\n'), 0o644)
}
