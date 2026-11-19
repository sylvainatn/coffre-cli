// Package localfile écrit les fichiers de secrets sur le poste avec les précautions d'usage :
// droits restreints, écriture atomique, exclusion de Git.
package localfile

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Write écrit le fichier de façon atomique (fichier temporaire puis renommage),
// lisible et modifiable par l'utilisateur seul (0600).
func Write(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".coffre-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // sans effet une fois renommé
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// EnsureGitIgnored ajoute le fichier au .gitignore du dépôt s'il n'est pas déjà ignoré.
// Renvoie le chemin du .gitignore modifié, ou "" si rien n'a été fait
// (fichier déjà ignoré, pas de dépôt Git, ou Git absent).
func EnsureGitIgnored(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := exec.LookPath("git"); err != nil {
		return "", nil
	}
	// Git renvoie des chemins réels (sans lien symbolique, ex : /var -> /private/var sur macOS)
	dir, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", err
	}
	abs = filepath.Join(dir, filepath.Base(abs))
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", nil // pas un dépôt Git
	}
	root := strings.TrimSpace(string(out))
	// check-ignore : code 0 = ignoré, 1 = non ignoré
	check := exec.Command("git", "-C", dir, "check-ignore", "-q", "--no-index", abs)
	if err := check.Run(); err == nil {
		return "", nil
	} else {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return "", nil
		}
	}

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	gitignore := filepath.Join(root, ".gitignore")
	existing, err := os.ReadFile(gitignore)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	entry := "/" + filepath.ToSlash(rel)
	prefix := ""
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		prefix = "\n"
	}
	f, err := os.OpenFile(gitignore, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s# Secrets locaux générés par coffre\n%s\n", prefix, entry); err != nil {
		return "", err
	}
	return gitignore, nil
}
