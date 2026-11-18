//go:build windows

package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
)

// Exec lance la commande en processus enfant (Windows n'a pas d'execve)
// et renvoie son code de sortie.
func Exec(args []string, env []string) (int, error) {
	path, err := exec.LookPath(args[0])
	if err != nil {
		return 127, fmt.Errorf("commande introuvable : %s", args[0])
	}
	cmd := exec.Command(path, args[1:]...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	// Ctrl+C est déjà transmis à l'enfant par la console : on l'ignore ici
	signal.Ignore(os.Interrupt)

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
