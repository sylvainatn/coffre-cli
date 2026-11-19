// Package agent détecte qu'une commande est lancée par un agent de code IA
// (Claude Code, Codex, Cursor, Gemini CLI…) pour appliquer des garde-fous :
// les valeurs des secrets ne doivent pas arriver dans la conversation du modèle.
package agent

import "os"

// Variables posées par les agents dans le shell où ils lancent leurs commandes.
// COFFRE_AGENT=1 permet de signaler n'importe quel autre agent.
var markers = []struct{ env, name string }{
	{"COFFRE_AGENT", "agent IA"},
	{"CLAUDECODE", "Claude Code"},
	{"CODEX_SANDBOX", "Codex"},
	{"CODEX_SANDBOX_NETWORK_DISABLED", "Codex"},
	{"CURSOR_AGENT", "Cursor"},
	{"GEMINI_CLI", "Gemini CLI"},
	{"AI_AGENT", "agent IA"},
}

// Detect renvoie le nom de l'agent qui a lancé la commande, ou "" pour un humain.
// La détection est volontairement sans échappatoire : un agent ne peut pas la désactiver
// par une variable d'environnement. Un humain lance simplement la commande dans son terminal.
func Detect() string {
	return detect(os.Getenv)
}

func detect(getenv func(string) string) string {
	for _, m := range markers {
		if v := getenv(m.env); v != "" && v != "0" && v != "false" {
			return m.name
		}
	}
	return ""
}
