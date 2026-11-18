package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/coffre-dev/coffre-cli/internal/api"
	"github.com/spf13/cobra"
)

func newSecretsCmd() *cobra.Command {
	var project, env string

	c := &cobra.Command{
		Use:     "secrets",
		Aliases: []string{"secret"},
		Short:   "Liste et modifie les secrets d'un environnement",
	}
	c.PersistentFlags().StringVarP(&project, "project", "p", "", "ID du projet (défaut : .coffre.json)")
	c.PersistentFlags().StringVarP(&env, "env", "e", "", "environnement (défaut : .coffre.json)")

	target := func() (*api.Client, string, string, error) {
		projectID, envSlug, err := resolveTarget(project, env)
		if err != nil {
			return nil, "", "", err
		}
		client, _, err := authedClient()
		return client, projectID, envSlug, err
	}

	var reveal bool
	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les secrets (valeurs masquées par défaut)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, projectID, envSlug, err := target()
			if err != nil {
				return err
			}
			secrets, err := client.GetSecrets(cmd.Context(), projectID, envSlug)
			if err != nil {
				return err
			}
			if len(secrets) == 0 {
				fmt.Printf("Aucun secret dans %s/%s\n", projectID, envSlug)
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			for _, s := range secrets {
				value := mask(s.Value)
				if reveal {
					value = s.Value
				}
				fmt.Fprintf(w, "%s\t%s\n", s.Key, value)
			}
			return w.Flush()
		},
	}
	list.Flags().BoolVar(&reveal, "reveal", false, "afficher les valeurs en clair")

	set := &cobra.Command{
		Use:     "set KEY=VALUE [KEY=VALUE...]",
		Short:   "Crée ou met à jour des secrets",
		Example: `  coffre secrets set STRIPE_KEY=sk_test_123 DEBUG=true`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			secrets := make([]api.Secret, 0, len(args))
			for _, arg := range args {
				k, v, ok := strings.Cut(arg, "=")
				if !ok || k == "" {
					return fmt.Errorf("format attendu KEY=VALUE, reçu %q", arg)
				}
				secrets = append(secrets, api.Secret{Key: k, Value: v})
			}
			client, projectID, envSlug, err := target()
			if err != nil {
				return err
			}
			if err := client.SetSecrets(cmd.Context(), projectID, envSlug, secrets); err != nil {
				return err
			}
			fmt.Printf("✓ %d secret(s) enregistré(s) dans %s/%s\n", len(secrets), projectID, envSlug)
			return nil
		},
	}

	del := &cobra.Command{
		Use:     "delete KEY",
		Aliases: []string{"rm"},
		Short:   "Supprime un secret",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, projectID, envSlug, err := target()
			if err != nil {
				return err
			}
			if err := client.DeleteSecret(cmd.Context(), projectID, envSlug, args[0]); err != nil {
				return err
			}
			fmt.Printf("✓ %s supprimé de %s/%s\n", args[0], projectID, envSlug)
			return nil
		},
	}

	download := &cobra.Command{
		Use:   "download",
		Short: "Affiche les secrets au format .env (pour Docker, CI…)",
		Long: `Écrit les secrets au format dotenv sur la sortie standard.
Attention : rediriger vers un fichier recrée un .env en clair sur le disque.`,
		Example: `  coffre secrets download --env production > /run/secrets/app.env`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, projectID, envSlug, err := target()
			if err != nil {
				return err
			}
			secrets, err := client.GetSecrets(cmd.Context(), projectID, envSlug)
			if err != nil {
				return err
			}
			for _, s := range secrets {
				fmt.Printf("%s=%s\n", s.Key, dotenvQuote(s.Value))
			}
			return nil
		},
	}

	c.AddCommand(list, set, del, download)
	return c
}

func mask(v string) string {
	if len(v) <= 4 {
		return "••••"
	}
	return v[:2] + strings.Repeat("•", 6)
}

func dotenvQuote(v string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, `$`, `\$`)
	return `"` + r.Replace(v) + `"`
}
