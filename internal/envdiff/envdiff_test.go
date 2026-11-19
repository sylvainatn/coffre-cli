package envdiff

import (
	"reflect"
	"testing"
)

func TestCompare(t *testing.T) {
	local := map[string]string{"A": "1", "B": "local", "D": "4"}
	remote := map[string]string{"A": "1", "B": "remote", "C": "3"}
	got := Compare(local, remote)
	want := Diff{OnlyRemote: []string{"C"}, OnlyLocal: []string{"D"}, Changed: []string{"B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if !Compare(local, local).Empty() {
		t.Fatal("aucune différence attendue")
	}
}
