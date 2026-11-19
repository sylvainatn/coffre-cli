package envformat

import (
	"encoding/json"
	"strings"
	"testing"
)

var vars = []Variable{{"API_KEY", "sk_test"}, {"CERT", "a\nb"}, {"MSG", `il a dit "ok" & <b>`}}

func TestJSON(t *testing.T) {
	out, err := Render("json", vars, "")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got["CERT"] != "a\nb" || got["MSG"] != `il a dit "ok" & <b>` {
		t.Fatalf("%v", got)
	}
}

func TestYAMLAndK8s(t *testing.T) {
	out, _ := Render("yaml", vars, "")
	if !strings.Contains(out, `CERT: "a\nb"`) || !strings.Contains(out, `MSG: "il a dit \"ok\" & <b>"`) {
		t.Fatalf("%s", out)
	}
	k8s, err := Render("k8s", vars, "coffre-production")
	if err != nil || !strings.HasPrefix(k8s, "apiVersion: v1\nkind: Secret\nmetadata:\n  name: coffre-production\n") {
		t.Fatalf("%v\n%s", err, k8s)
	}
	if _, err := Render("k8s", vars, "Pas_Valide"); err == nil {
		t.Fatal("nom invalide attendu")
	}
}

func TestDockerRefusesMultiline(t *testing.T) {
	if _, err := Render("docker", vars, ""); err == nil {
		t.Fatal("erreur attendue pour une valeur multiligne")
	}
	out, err := Render("docker", vars[:1], "")
	if err != nil || out != "API_KEY=sk_test\n" {
		t.Fatalf("%q %v", out, err)
	}
}

func TestUnknownFormat(t *testing.T) {
	if _, err := Render("toml", vars, ""); err == nil {
		t.Fatal("erreur attendue")
	}
}
