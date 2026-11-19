package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coffre-dev/coffre-cli/internal/agentfiles"
	"github.com/coffre-dev/coffre-cli/internal/api"
	"github.com/coffre-dev/coffre-cli/internal/config"
	"github.com/spf13/cobra"
)

var roleLabels = map[string]string{"OWNER": "propriétaire", "MEMBER": "membre", "VIEWER": "lecteur"}

func newInitCmd() *cobra.Command {
	var env, create, file string
	var push, yes, agents, jsonOut bool

	c := &cobra.Command{
		Use:   "init [projectId]",
		Short: "Associe le dossier courant à un projet Coffre",
		Long: `Crée un fichier .coffre.json (sans aucun secret, il peut être commité).

Sans argument, dans un terminal : choix parmi vos projets, ou création d'un nouveau.
Avec --push, les variables du .env présent dans le dossier sont ensuite envoyées
dans Coffre (sans écraser celles qui existent) : migrer un projet tient en une commande.`,
		Example: `  coffre init                              # choix interactif
  coffre init proj_12345 --env development  # scripts, CI
  coffre init --create "Site client"         # crée le projet
  coffre init --create "Site client" --push  # crée le projet et y envoie le .env`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := authedClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			tty := interactive() && !jsonOut
			if jsonOut && push {
				return errors.New("--json et --push ne se combinent pas : lancez ensuite coffre secrets push --json")
			}

			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if existing, err := os.ReadFile(filepath.Join(wd, config.ProjectFileName)); err == nil && tty && !yes {
				fmt.Fprintf(os.Stderr, "Ce dossier est déjà relié à un projet (%s).\n", strings.TrimSpace(string(existing)))
				if !confirm("Le remplacer ?") {
					return errors.New("annulé")
				}
			}

			var projectID string
			switch {
			case len(args) == 1 && create != "":
				return errors.New("précisez soit un ID de projet, soit --create, pas les deux")
			case len(args) == 1:
				projectID = args[0]
			case create != "":
				if projectID, err = client.CreateProject(ctx, create); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "✓ projet « %s » créé (%s)\n", create, projectID)
			case tty:
				if projectID, err = pickProject(ctx, client, filepath.Base(wd)); err != nil {
					return err
				}
			default:
				return errors.New("précisez le projet : coffre init <projectId>, ou coffre init --create \"Nom\"")
			}

			project, err := client.GetProject(ctx, projectID)
			if err != nil {
				var apiErr *api.Error
				if errors.As(err, &apiErr) && apiErr.Status == 404 {
					return fmt.Errorf("projet %q introuvable (ou vous n'en êtes pas membre)", projectID)
				}
				return err
			}
			if len(project.Environments) == 0 {
				return errors.New("ce projet n'a aucun environnement accessible pour vous")
			}

			// Environnement : --env s'il est précisé, sinon choix dans un terminal, sinon development
			wanted := env
			if !cmd.Flags().Changed("env") && tty {
				names := make([]string, len(project.Environments))
				def := 0
				for i, e := range project.Environments {
					names[i] = e.Slug
					if e.Slug == "development" {
						def = i
					}
				}
				wanted = project.Environments[choose("Environnement par défaut de ce dossier :", names, def)].Slug
			}
			slug, err := matchEnvironment(project.Environments, wanted)
			if err != nil {
				return err
			}

			// Relancer init ne doit pas effacer la politique des agents déjà définie
			next := &config.Project{ProjectID: project.ID, Environment: slug}
			if previous, err := config.ReadProject(wd); err == nil && previous != nil {
				next.Agents = previous.Agents
			}
			path, err := config.WriteProject(wd, next)
			if err != nil {
				return err
			}
			if !jsonOut {
				fmt.Printf("✓ %s relié au projet « %s » (%s)\n", config.ProjectFileName, project.Name, slug)
				fmt.Printf("  %s\n", path)
			}

			// Instructions pour les agents IA (AGENTS.md, skill Claude Code)
			if !agents && tty && !yes {
				agents = confirm("Ajouter les instructions pour les agents IA (AGENTS.md, skill Claude Code) ?")
			}
			var agentFiles any = []any{}
			if agents {
				written, err := agentfiles.Setup(wd, agentfiles.Params{
					ProjectName:  project.Name,
					DefaultEnv:   slug,
					WritableEnvs: next.AgentWritableEnvironments(),
				})
				if err != nil {
					return err
				}
				agentFiles = written
				if !jsonOut {
					printAgentFiles(written)
				}
			}

			// Un .env dans le dossier : proposer de l'envoyer dans Coffre
			envFilePresent := false
			if _, err := os.Stat(file); err == nil {
				envFilePresent = true
				if !push && tty && !yes {
					push = confirm(fmt.Sprintf("Un fichier %s est présent : envoyer ses variables dans %s ?", file, slug))
				}
				if push && !jsonOut {
					fmt.Println()
					if err := runPush(ctx, client, project.ID, slug, file, yes); err != nil {
						return err
					}
					fmt.Printf("\n  Une fois vérifié, vous pouvez supprimer %s.\n", file)
				}
			} else if push {
				return fmt.Errorf("--push : %s introuvable", file)
			}

			if jsonOut {
				return printJSON(map[string]any{
					"projectId":      project.ID,
					"projectName":    project.Name,
					"environment":    slug,
					"path":           path,
					"agentFiles":     agentFiles,
					"envFilePresent": envFilePresent,
				})
			}

			fmt.Println("\n  Lancez votre application avec : coffre run -- <commande>")
			return nil
		},
	}
	c.Flags().StringVarP(&env, "env", "e", "development", "environnement par défaut")
	c.Flags().StringVar(&create, "create", "", "créer un projet avec ce nom et y relier le dossier")
	c.Flags().BoolVar(&push, "push", false, "envoyer ensuite les variables du .env dans Coffre (coffre secrets push)")
	c.Flags().StringVarP(&file, "file", "f", ".env", "fichier .env à envoyer avec --push")
	c.Flags().BoolVarP(&yes, "yes", "y", false, "ne pas demander de confirmation")
	c.Flags().BoolVar(&agents, "agents", false, "écrire aussi les instructions pour les agents IA (voir coffre agent setup)")
	c.Flags().BoolVar(&jsonOut, "json", false, "résultat en JSON (aucune question posée)")
	return c
}

// pickProject : choix parmi les projets de l'utilisateur, ou création d'un nouveau.
func pickProject(ctx context.Context, client *api.Client, folder string) (string, error) {
	projects, err := client.ListProjects(ctx)
	if err != nil {
		return "", err
	}
	options := make([]string, 0, len(projects)+1)
	for _, p := range projects {
		options = append(options, fmt.Sprintf("%s  (%s, %s)", p.Name, p.ID, roleLabelOf(p.Role)))
	}
	options = append(options, "+ Créer un nouveau projet")

	index := choose("Projet :", options, 0)
	if index < len(projects) {
		return projects[index].ID, nil
	}
	name := askDefault("Nom du projet", folder)
	id, err := client.CreateProject(ctx, name)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "✓ projet « %s » créé (%s)\n", name, id)
	return id, nil
}

func roleLabelOf(role string) string {
	if label, ok := roleLabels[role]; ok {
		return label
	}
	return strings.ToLower(role)
}

// runPush réutilise `coffre secrets push` sur l'environnement qui vient d'être choisi.
func runPush(ctx context.Context, client *api.Client, projectID, env, file string, yes bool) error {
	push := newPushCmd(func() (*api.Client, string, string, error) { return client, projectID, env, nil })
	args := []string{"--file", file}
	if yes {
		args = append(args, "--yes")
	}
	push.SetArgs(args)
	push.SilenceUsage, push.SilenceErrors = true, true
	return push.ExecuteContext(ctx)
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
