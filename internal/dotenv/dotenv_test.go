package dotenv

import (
	"reflect"
	"testing"
)

const sample = `# Base de données
DATABASE_URL=postgres://localhost:5432/app
export API_KEY="sk_test_\"abc\"" # clé de test

EMPTY=
SPACED = valeur avec espaces   # commentaire
SINGLE='$NE_PAS_INTERPOLER'
CERT="-----BEGIN-----
ligne 2
-----END-----"
ceci n'est pas une variable
DUP=1
DUP=2
`

func TestParseValues(t *testing.T) {
	f, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"DATABASE_URL": "postgres://localhost:5432/app",
		"API_KEY":      `sk_test_"abc"`,
		"EMPTY":        "",
		"SPACED":       "valeur avec espaces",
		"SINGLE":       "$NE_PAS_INTERPOLER",
		"CERT":         "-----BEGIN-----\nligne 2\n-----END-----",
		"DUP":          "2",
	}
	if got := f.Values(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
	if got := f.Keys(); !reflect.DeepEqual(got, []string{"DATABASE_URL", "API_KEY", "EMPTY", "SPACED", "SINGLE", "CERT", "DUP"}) {
		t.Fatalf("keys: %v", got)
	}
}

func TestRoundTripPreservesFile(t *testing.T) {
	f, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	if f.String() != sample {
		t.Fatalf("le fichier relu doit être identique :\n%s", f.String())
	}
}

func TestSetReplacesInPlaceAndAppends(t *testing.T) {
	f, _ := Parse("# tête\nexport A=1\nB=2\n")
	f.Set("A", "nouvelle valeur")
	f.AppendComment("Ajoutées par coffre")
	f.Set("C", "3")
	want := "# tête\nexport A='nouvelle valeur'\nB=2\n\n# Ajoutées par coffre\nC=3\n"
	if f.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", f.String(), want)
	}
}

func TestLineRoundTrip(t *testing.T) {
	values := []string{"", "simple", "avec espace", "$HOME", "it's", "multi\nligne", `back\slash "q"`, "a#b", "x=y"}
	for _, v := range values {
		f, err := Parse(Line("K", v) + "\n")
		if err != nil {
			t.Fatalf("%q : %v", v, err)
		}
		if got := f.Values()["K"]; got != v {
			t.Fatalf("Line(%q) = %s, relu %q", v, Line("K", v), got)
		}
	}
}

func TestUnclosedQuote(t *testing.T) {
	if _, err := Parse("A=\"pas fermé\nB=1\n"); err == nil {
		t.Fatal("erreur attendue")
	}
}
