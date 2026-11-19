package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// interactive : un humain peut répondre (entrée et sortie d'erreur sur un terminal).
func interactive() bool {
	for _, f := range []*os.File{os.Stdin, os.Stderr} {
		info, err := f.Stat()
		if err != nil || info.Mode()&os.ModeCharDevice == 0 {
			return false
		}
	}
	return true
}

var stdinReader = bufio.NewReader(os.Stdin)

// ask pose une question sur la sortie d'erreur et renvoie la réponse en minuscules.
func ask(question string) string {
	fmt.Fprint(os.Stderr, question)
	answer, _ := stdinReader.ReadString('\n')
	return strings.ToLower(strings.TrimSpace(answer))
}

// confirm : oui / non, non par défaut.
func confirm(question string) bool {
	switch ask(question + " [o/N] ") {
	case "o", "oui", "y", "yes":
		return true
	}
	return false
}

// sensitiveEnv : environnements où une écriture mérite une confirmation explicite
// (staging, production, et leurs alias : stg, stage, preprod, prod…).
func sensitiveEnv(ref string) bool {
	ref = strings.ToLower(ref)
	if alias, ok := envAliases[ref]; ok {
		ref = alias
	}
	return ref == "staging" || strings.Contains(ref, "prod") || strings.Contains(ref, "stag")
}
