package agent

import "testing"

func TestDetect(t *testing.T) {
	env := func(vars map[string]string) func(string) string {
		return func(k string) string { return vars[k] }
	}
	cases := []struct {
		vars map[string]string
		want string
	}{
		{map[string]string{}, ""},
		{map[string]string{"CLAUDECODE": "1"}, "Claude Code"},
		{map[string]string{"CODEX_SANDBOX": "seatbelt"}, "Codex"},
		{map[string]string{"CURSOR_AGENT": "1"}, "Cursor"},
		{map[string]string{"COFFRE_AGENT": "1"}, "agent IA"},
		{map[string]string{"CLAUDECODE": "0"}, ""},
	}
	for _, c := range cases {
		if got := detect(env(c.vars)); got != c.want {
			t.Errorf("%v : %q, attendu %q", c.vars, got, c.want)
		}
	}
}
