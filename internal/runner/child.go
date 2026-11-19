package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// RunChild lance la commande en processus enfant et attend sa fin, au lieu de remplacer
// le processus coffre. Utilisé quand coffre doit agir après la commande
// (supprimer le fichier temporaire de `coffre run --file`).
//
// Ctrl+C est envoyé par le terminal à tout le groupe de processus : l'enfant le reçoit
// directement, coffre se contente de ne pas s'arrêter avant lui. SIGTERM et SIGHUP
// (arrêt demandé à coffre lui-même) sont relayés à l'enfant.
func RunChild(args []string, env []string) (int, error) {
	path, err := exec.LookPath(args[0])
	if err != nil {
		return 127, fmt.Errorf("commande introuvable : %s", args[0])
	}
	cmd := exec.Command(path, args[1:]...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer func() {
		signal.Stop(signals)
		close(signals)
	}()

	if err := cmd.Start(); err != nil {
		return 126, fmt.Errorf("impossible d'exécuter %s : %w", args[0], err)
	}
	go func() {
		for s := range signals {
			if s != os.Interrupt {
				_ = cmd.Process.Signal(s)
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if code := exitErr.ExitCode(); code >= 0 {
				return code, nil
			}
			return 1, nil // arrêtée par un signal
		}
		return 1, err
	}
	return 0, nil
}
