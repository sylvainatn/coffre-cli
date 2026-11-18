// Package runner lance la commande de l'utilisateur avec les secrets injectés
// dans son environnement. Les secrets ne sont jamais écrits sur disque.
package runner

import (
	"sort"
	"strings"
)

// MergeEnv fusionne les secrets dans l'environnement courant.
// Par défaut les secrets écrasent les variables existantes ; avec keepExisting,
// les variables déjà définies dans le shell sont prioritaires.
func MergeEnv(base []string, secrets map[string]string, keepExisting bool) []string {
	env := make(map[string]string, len(base)+len(secrets))
	order := make([]string, 0, len(base)+len(secrets))
	for _, kv := range base {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if _, seen := env[k]; !seen {
			order = append(order, k)
		}
		env[k] = v
	}
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, exists := env[k]; exists {
			if keepExisting {
				continue
			}
		} else {
			order = append(order, k)
		}
		env[k] = secrets[k]
	}
	out := make([]string, 0, len(order))
	for _, k := range order {
		out = append(out, k+"="+env[k])
	}
	return out
}
