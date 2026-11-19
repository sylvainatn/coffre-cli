package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/coffre-dev/coffre-cli/internal/api"
	"github.com/coffre-dev/coffre-cli/internal/envformat"
	"github.com/coffre-dev/coffre-cli/internal/runner"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var project, env string
	var keepExisting, quiet bool
	var file string

	c := &cobra.Command{
		Use:   "run -- <commande> [args...]",
		Short: "Lance une commande avec les secrets injectés en mémoire",
		Long: `Récupère les secrets de l'environnement configuré et lance la commande
avec ces variables. Rien n'est écrit sur le disque.

Les outils qui chargent un .env (dotenv, Next.js, Vite, Prisma, Laravel, Symfony…)
n'écrasent pas une variable déjà définie : coffre run suffit, sans fichier.

Si un outil exige vraiment un fichier (env_file de Docker Compose…), --file l'écrit
pendant la durée de la commande (droits 0600) et le supprime à la fin.`,
		Example: `  coffre run -- npm run dev
  coffre run --env staging -- node server.js
  coffre run -- sh -c 'echo $DATABASE_URL'
  coffre run --file .env -- docker compose up`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, envSlug, err := resolveTarget(project, env)
			if err != nil {
				return err
			}
			if file != "" {
				if _, err := os.Lstat(file); err == nil {
					return fmt.Errorf("%s existe déjà : coffre ne l'écrasera pas (supprimez-le, ou utilisez coffre secrets pull)", file)
				}
			}
			client, _, err := authedClient()
			if err != nil {
				return err
			}
			secrets, err := client.GetSecrets(cmd.Context(), projectID, envSlug)
			if err != nil {
				return err
			}

			values := make(map[string]string, len(secrets))
			for _, s := range secrets {
				values[s.Key] = s.Value
			}
			if !quiet {
				fmt.Fprintf(os.Stderr, "🔐 coffre : %d variable(s) injectée(s) depuis %s/%s\n", len(values), projectID, envSlug)
			}
			env := runner.MergeEnv(os.Environ(), values, keepExisting)

			if file != "" {
				return runWithFile(file, secrets, args, env, quiet)
			}
			code, err := runner.Exec(args, env)
			if err != nil {
				fmt.Fprintln(os.Stderr, "✗", err)
			}
			if code != 0 {
				return &exitCodeError{code: code}
			}
			return nil
		},
	}
	// Tout ce qui suit la commande lui appartient (ex : `coffre run npm run dev --port 3000`)
	c.Flags().SetInterspersed(false)
	c.Flags().StringVarP(&project, "project", "p", "", "ID du projet (défaut : .coffre.json)")
	c.Flags().StringVarP(&env, "env", "e", "", "environnement (défaut : .coffre.json)")
	c.Flags().BoolVar(&keepExisting, "keep-existing", false, "ne pas écraser les variables déjà définies dans le shell")
	c.Flags().BoolVarP(&quiet, "quiet", "q", false, "ne rien afficher avant de lancer la commande")
	c.Flags().StringVar(&file, "file", "", "écrire aussi les secrets dans ce fichier .env, supprimé à la fin de la commande")
	return c
}

// runWithFile écrit le fichier temporaire, lance la commande en processus enfant
// (coffre doit survivre à la commande pour supprimer le fichier), puis le supprime.
func runWithFile(path string, secrets []api.Secret, args, env []string, quiet bool) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("%s existe déjà : coffre ne l'écrasera pas (supprimez-le, ou utilisez coffre secrets pull)", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	vars := make([]envformat.Variable, len(secrets))
	for i, s := range secrets {
		vars[i] = envformat.Variable{Key: s.Key, Value: s.Value}
	}
	content, err := envformat.Render("dotenv", vars, "")
	if err != nil {
		return err
	}
	if err := writeLocal(path, "# Fichier temporaire écrit par coffre run : supprimé à la fin de la commande\n"+content); err != nil {
		return err
	}
	removed := false
	remove := func() {
		if !removed {
			removed = true
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(os.Stderr, "✗ impossible de supprimer %s : %v\n", path, err)
			}
		}
	}
	defer remove()
	if !quiet {
		fmt.Fprintf(os.Stderr, "🔐 coffre : %s écrit pour la durée de la commande\n", path)
	}

	code, err := runner.RunChild(args, env)
	remove()
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗", err)
	}
	if code != 0 {
		return &exitCodeError{code: code}
	}
	return nil
}
