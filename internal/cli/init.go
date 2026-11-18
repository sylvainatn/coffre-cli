package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/coffre-dev/coffre-cli/internal/api"
	"github.com/coffre-dev/coffre-cli/internal/config"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var env string

	c := &cobra.Command{
		Use:     "init <projectId>",
		Short:   "Associe le dossier courant à un projet Coffre",
		Long:    "Crée un fichier .coffre.json (sans aucun secret, il peut être commité).",
		Example: "  coffre init proj_12345 --env development",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := authedClient()
			if err != nil {
				return err
			}
			project, err := client.GetProject(cmd.Context(), args[0])
			if err != nil {
				var apiErr *api.Error
				if errors.As(err, &apiErr) && apiErr.Status == 404 {
					return fmt.Errorf("projet %q introuvable (ou vous n'en êtes pas membre)", args[0])
				}
				return err
			}

			slug, err := matchEnvironment(project.Environments, env)
			if err != nil {
				return err
			}

			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			path, err := config.WriteProject(wd, &config.Project{ProjectID: project.ID, Environment: slug})
			if err != nil {
				return err
			}
			fmt.Printf("✓ %s relié au projet « %s » (%s)\n", config.ProjectFileName, project.Name, slug)
			fmt.Printf("  %s\n\n  Lancez votre application avec : coffre run -- <commande>\n", path)
			return nil
		},
	}
	c.Flags().StringVarP(&env, "env", "e", "development", "environnement par défaut")
	return c
}

var envAliases = map[string]string{
	"dev": "development", "develop": "development",
	"stg": "staging", "stage": "staging", "preprod": "staging",
	"prod": "production",
}

func matchEnvironment(envs []api.Environment, wanted string) (string, error) {
	wanted = strings.ToLower(wanted)
	for _, candidate := range []string{wanted, envAliases[wanted]} {
		for _, e := range envs {
			if e.Slug == candidate {
				return e.Slug, nil
			}
		}
	}
	slugs := make([]string, len(envs))
	for i, e := range envs {
		slugs[i] = e.Slug
	}
	return "", fmt.Errorf("environnement %q inconnu (disponibles : %s)", wanted, strings.Join(slugs, ", "))
}
