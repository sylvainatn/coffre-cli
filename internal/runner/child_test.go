//go:build !windows

package runner

import "testing"

func TestRunChildExitCodeAndEnv(t *testing.T) {
	code, err := RunChild([]string{"sh", "-c", `test "$COFFRE_TEST" = ok && exit 3`}, []string{"COFFRE_TEST=ok"})
	if err != nil || code != 3 {
		t.Fatalf("code %d, err %v", code, err)
	}
	if code, _ := RunChild([]string{"commande-qui-n-existe-pas"}, nil); code != 127 {
		t.Fatalf("code %d, 127 attendu", code)
	}
}
