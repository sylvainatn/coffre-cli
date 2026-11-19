package cli

import (
	"encoding/json"
	"os"
)

// printJSON écrit un résultat lisible par une machine (agents IA, scripts) sur la sortie standard.
// Les erreurs restent sur la sortie d'erreur, avec un code de sortie non nul.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// orEmpty : une liste vide s'écrit [] et non null en JSON.
func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
