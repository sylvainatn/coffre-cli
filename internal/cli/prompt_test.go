package cli

import "testing"

func TestSensitiveEnv(t *testing.T) {
	for ref, want := range map[string]bool{
		"production": true, "prod": true, "PROD": true, "staging": true, "stg": true, "stage": true,
		"preprod": true, "prod-eu": true, "development": false, "dev": false, "recette": false,
	} {
		if got := sensitiveEnv(ref); got != want {
			t.Errorf("sensitiveEnv(%q) = %v, attendu %v", ref, got, want)
		}
	}
}
