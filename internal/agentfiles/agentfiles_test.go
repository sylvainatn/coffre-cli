package agentfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var params = Params{ProjectName: "Site client", DefaultEnv: "development", WritableEnvs: []string{"development"}}

func TestSetupCreatesThenUpdatesInPlace(t *testing.T) {
	dir := t.TempDir()
	agents := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("# Mon projet\n\nRègles existantes.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	written, err := Setup(dir, params)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 || written[0].Action != "updated" || written[1].Action != "created" {
		t.Fatalf("%+v", written)
	}
	data, _ := os.ReadFile(agents)
	if !strings.HasPrefix(string(data), "# Mon projet\n\nRègles existantes.\n\n"+startMarker) {
		t.Fatalf("le contenu existant doit être conservé :\n%s", data)
	}

	// Relancer ne duplique rien ; changer les paramètres remplace la section
	if written, _ := Setup(dir, params); written[0].Action != "unchanged" || written[1].Action != "unchanged" {
		t.Fatalf("%+v", written)
	}
	other := params
	other.WritableEnvs = []string{"development", "staging"}
	if _, err := Setup(dir, other); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(agents)
	if strings.Count(string(data), startMarker) != 1 || !strings.Contains(string(data), "`staging`") {
		t.Fatalf("section non remplacée :\n%s", data)
	}

	skill, _ := os.ReadFile(filepath.Join(dir, ".claude", "skills", "coffre", "SKILL.md"))
	if !strings.HasPrefix(string(skill), "---\nname: coffre\ndescription: ") {
		t.Fatalf("skill :\n%s", skill)
	}
}

func TestClaudeMdOnlyIfPresent(t *testing.T) {
	dir := t.TempDir()
	if _, err := Setup(dir, params); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err == nil {
		t.Fatal("CLAUDE.md ne doit pas être créé")
	}
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("Contexte."), 0o644); err != nil {
		t.Fatal(err)
	}
	written, _ := Setup(dir, params)
	if len(written) != 3 || written[2].Action != "updated" {
		t.Fatalf("%+v", written)
	}
}
