//go:build !windows

package runner

import (
	"fmt"
	"os/exec"
	"syscall"
)

// Exec remplace le processus coffre par la commande cible (execve).
// Les signaux (Ctrl+C, SIGTERM...) et le code de sortie sont donc
// exactement ceux de la commande, sans processus intermédiaire.
func Exec(args []string, env []string) (int, error) {
	path, err := exec.LookPath(args[0])
	if err != nil {
		return 127, fmt.Errorf("commande introuvable : %s", args[0])
	}
	if err := syscall.Exec(path, args, env); err != nil {
		return 126, fmt.Errorf("impossible d'exécuter %s : %w", args[0], err)
	}
	return 0, nil // jamais atteint
}
