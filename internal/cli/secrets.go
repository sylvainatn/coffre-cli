package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/coffre-dev/coffre-cli/internal/api"
	"github.com/coffre-dev/coffre-cli/internal/envformat"
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

	var target targetFunc = func() (*api.Client, string, string, error) {
		projectID, envSlug, err := resolveTarget(project, env)
		if err != nil {
			return nil, "", "", err
		}
		client, _, err := authedClient()
		return client, projectID, envSlug, err
	}

	var reveal, listJSON bool
	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les secrets (valeurs masquées par défaut)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if reveal {
				if err := refuseForAgent("--reveal", "les valeurs finiraient dans la conversation du modèle"); err != nil {
					return err
				}
			}
			client, projectID, envSlug, err := target()
			if err != nil {
				return err
			}
			secrets, err := client.GetSecrets(cmd.Context(), projectID, envSlug)
			if err != nil {
				return err
			}
			if listJSON {
				// Noms seulement, sauf --reveal (refusé aux agents)
				items := make([]map[string]any, len(secrets))
				for i, s := range secrets {
					item := map[string]any{"key": s.Key, "empty": s.Value == ""}
					if reveal {
						item["value"] = s.Value
					}
					items[i] = item
				}
				return printJSON(map[string]any{"project": projectID, "environment": envSlug, "secrets": items})
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
	list.Flags().BoolVar(&reveal, "reveal", false, "afficher les valeurs en clair (refusé aux agents IA)")
	list.Flags().BoolVar(&listJSON, "json", false, "noms des variables en JSON (valeurs seulement avec --reveal)")

	set := &cobra.Command{
		Use:   "set KEY=VALUE [KEY=VALUE...]",
		Short: "Crée ou met à jour des secrets",
		Example: `  coffre secrets set STRIPE_KEY=sk_test_123 DEBUG=true
  coffre secrets set STRIPE_KEY=   # variable vide, à compléter par l'équipe`,
		Args: cobra.MinimumNArgs(1),
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
			if err := checkAgentWrite(envSlug); err != nil {
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
			if err := checkAgentWrite(envSlug); err != nil {
				return err
			}
			if err := client.DeleteSecret(cmd.Context(), projectID, envSlug, args[0]); err != nil {
				return err
			}
			fmt.Printf("✓ %s supprimé de %s/%s\n", args[0], projectID, envSlug)
			return nil
		},
	}

	var output, format, name string
	download := &cobra.Command{
		Use:     "download",
		Aliases: []string{"export"},
		Short:   "Exporte les secrets (.env, JSON, YAML, Docker, Kubernetes)",
		Long: `Écrit tous les secrets de l'environnement, sur la sortie standard par défaut.

Avec -o, le fichier est remplacé entièrement, écrit avec des droits 0600 et ajouté
au .gitignore s'il ne l'est pas. Pour compléter un .env existant sans l'écraser :
coffre secrets pull. Pour ne pas avoir de fichier du tout : coffre run -- <commande>.

Formats : dotenv (défaut), json, yaml, docker (docker run --env-file), k8s (Secret).
Désactivé pour les agents IA : les valeurs finiraient dans la conversation du modèle.`,
		Example: `  coffre secrets download --env production > /run/secrets/app.env
  coffre secrets export -o .env
  coffre secrets export --env production --format k8s --name mon-app | kubectl apply -f -`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := refuseForAgent("secrets download / export", "les valeurs finiraient dans la conversation du modèle ou dans un .env ; utilisez coffre run -- <commande>"); err != nil {
				return err
			}
			client, projectID, envSlug, err := target()
			if err != nil {
				return err
			}
			secrets, err := client.GetSecrets(cmd.Context(), projectID, envSlug)
			if err != nil {
				return err
			}
			vars := make([]envformat.Variable, len(secrets))
			for i, s := range secrets {
				vars[i] = envformat.Variable{Key: s.Key, Value: s.Value}
			}
			if name == "" {
				name = "coffre-" + envSlug
			}
			content, err := envformat.Render(format, vars, name)
			if err != nil {
				return err
			}
			if output == "" || output == "-" {
				fmt.Print(content)
				return nil
			}
			if err := writeLocal(output, content); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "✓ %d variable(s) de %s/%s écrite(s) dans %s\n", len(vars), projectID, envSlug, output)
			return nil
		},
	}
	download.Flags().StringVarP(&output, "output", "o", "", "fichier à écrire (défaut : sortie standard)")
	download.Flags().StringVar(&format, "format", "dotenv", "format : "+strings.Join(envformat.Formats, ", "))
	download.Flags().StringVar(&name, "name", "", "nom du Secret Kubernetes (défaut : coffre-<env>)")

	c.AddCommand(list, set, del, download, newDiffCmd(target), newPullCmd(target), newPushCmd(target))
	return c
}

func mask(v string) string {
	if len(v) <= 4 {
		return "••••"
	}
	return v[:2] + strings.Repeat("•", 6)
}
