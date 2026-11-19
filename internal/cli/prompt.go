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

// choose affiche une liste numérotée et renvoie l'index choisi (0 à len(options)-1).
// Entrée vide : choix par défaut (def). Redemande tant que la réponse est invalide.
func choose(question string, options []string, def int) int {
	fmt.Fprintln(os.Stderr, question)
	for i, o := range options {
		marker := " "
		if i == def {
			marker = "›"
		}
		fmt.Fprintf(os.Stderr, "  %s %d. %s\n", marker, i+1, o)
	}
	for {
		answer := ask(fmt.Sprintf("Votre choix [%d] : ", def+1))
		if answer == "" {
			return def
		}
		var n int
		if _, err := fmt.Sscanf(answer, "%d", &n); err == nil && n >= 1 && n <= len(options) {
			return n - 1
		}
		fmt.Fprintf(os.Stderr, "  Entrez un nombre entre 1 et %d.\n", len(options))
	}
}

// askDefault pose une question ouverte ; une réponse vide renvoie def. La casse est conservée.
func askDefault(question, def string) string {
	fmt.Fprintf(os.Stderr, "%s [%s] : ", question, def)
	answer, _ := stdinReader.ReadString('\n')
	if answer = strings.TrimSpace(answer); answer == "" {
		return def
	}
	return answer
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
