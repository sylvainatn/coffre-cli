package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/coffre-dev/coffre-cli/internal/agent"
	"github.com/coffre-dev/coffre-cli/internal/config"
)

// Garde-fous appliqués quand un agent de code IA lance la commande.
// Ils évitent les erreurs et les fuites accidentelles (valeurs recopiées dans la conversation
// du modèle, .env recréé, écriture en production). Ils ne remplacent pas les droits du compte :
// un agent qui dispose d'un jeton peut toujours lancer `coffre run -- sh -c 'echo $X'`.

// refuseForAgent bloque une commande qui exposerait des valeurs ou recréerait un .env.
func refuseForAgent(what, why string) error {
	name := agent.Detect()
	if name == "" {
		return nil
	}
	return fmt.Errorf("%s détecté : %s est désactivé pour les agents IA (%s). "+
		"Si c'est vraiment nécessaire, l'utilisateur doit lancer la commande lui-même dans son terminal", name, what, why)
}

// checkAgentWrite n'autorise un agent à écrire que dans les environnements prévus par .coffre.json
// (development par défaut).
func checkAgentWrite(env string) error {
	name := agent.Detect()
	if name == "" {
		return nil
	}
	local, _, _ := config.FindProject()
	allowed := local.AgentWritableEnvironments()
	if slices.Contains(allowed, canonicalEnv(env)) {
		return nil
	}
	return fmt.Errorf("%s détecté : écriture dans %q refusée aux agents IA (autorisé : %s). "+
		"Demandez à l'utilisateur de le faire, ou d'ajouter l'environnement à \"agents.write\" dans .coffre.json",
		name, env, strings.Join(allowed, ", "))
}

// canonicalEnv : slug d'un environnement désigné par un alias (prod -> production).
func canonicalEnv(ref string) string {
	ref = strings.ToLower(ref)
	if alias, ok := envAliases[ref]; ok {
		return alias
	}
	return ref
}
