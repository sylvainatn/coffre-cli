package cli

import (
	"fmt"
	"os"

	"github.com/coffre-dev/coffre-cli/internal/runner"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var project, env string
	var keepExisting, quiet bool

	c := &cobra.Command{
		Use:   "run -- <commande> [args...]",
		Short: "Lance une commande avec les secrets injectés en mémoire",
		Long: `Récupère les secrets de l'environnement configuré et lance la commande
avec ces variables. Rien n'est écrit sur le disque.`,
		Example: `  coffre run -- npm run dev
  coffre run --env staging -- node server.js
  coffre run -- sh -c 'echo $DATABASE_URL'`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, envSlug, err := resolveTarget(project, env)
			if err != nil {
				return err
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

			code, err := runner.Exec(args, runner.MergeEnv(os.Environ(), values, keepExisting))
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
	return c
}
