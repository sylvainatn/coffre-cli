package localfile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := Write(path, "A=1\n"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("droits %v, 0600 attendus", info.Mode().Perm())
	}
	// Réécriture : le contenu est remplacé, les droits restent stricts
	if err := Write(path, "A=2\n"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "A=2\n" {
		t.Fatalf("contenu %q", data)
	}
}

func TestEnsureGitIgnored(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git absent")
	}
	repo := t.TempDir()
	if err := exec.Command("git", "-C", repo, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "api", ".env")

	changed, err := EnsureGitIgnored(path)
	if err != nil || changed == "" {
		t.Fatalf("ajout attendu : %q, %v", changed, err)
	}
	data, _ := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if !strings.Contains(string(data), "/api/.env") {
		t.Fatalf(".gitignore : %q", data)
	}
	// Deuxième appel : déjà ignoré, rien à faire
	if changed, _ := EnsureGitIgnored(path); changed != "" {
		t.Fatal("aucun ajout attendu la deuxième fois")
	}
}
