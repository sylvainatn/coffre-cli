package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/coffre-dev/coffre-cli/internal/agent"
	"github.com/coffre-dev/coffre-cli/internal/agentfiles"
	"github.com/coffre-dev/coffre-cli/internal/config"
	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "agent",
		Short: "Intégration avec les agents de code IA (Claude Code, Codex, Cursor…)",
		Long: `Les agents de code IA ont le réflexe d'écrire les variables dans un fichier .env.
coffre agent setup leur apprend à utiliser Coffre à la place, et la CLI applique des
garde-fous quand elle détecte un agent : pas d'affichage des valeurs, pas de .env,
écriture limitée aux environnements autorisés (development par défaut).`,
	}
	c.AddCommand(newAgentSetupCmd(), newAgentStatusCmd())
	return c
}

func newAgentSetupCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "setup",
		Short: "Écrit les instructions pour les agents IA (AGENTS.md, skill Claude Code)",
		Long: `Écrit, dans le dossier du projet (celui de .coffre.json) :

- AGENTS.md : une section Coffre, lue par Codex, Cursor, Copilot, Gemini… Le reste du
  fichier n'est pas touché ; relancer la commande met la section à jour ;
- .claude/skills/coffre/SKILL.md : un skill chargé par Claude Code dès qu'une tâche
  touche aux variables d'environnement ;
- CLAUDE.md : un rappel, seulement si le fichier existe déjà.

Ces fichiers ne contiennent aucun secret : ils peuvent (et doivent) être commités.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			local, path, err := config.FindProject()
			if err != nil {
				return err
			}
			if local == nil {
				return errors.New("aucun projet configuré ici : lancez d'abord `coffre init`")
			}
			written, err := setupAgentFiles(cmd.Context(), local, filepath.Dir(path))
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(map[string]any{"files": written})
			}
			printAgentFiles(written)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "résultat en JSON")
	return c
}

func newAgentStatusCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Indique si un agent IA est détecté et ce qu'il peut faire ici",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			local, _, _ := config.FindProject()
			name := agent.Detect()
			writable := local.AgentWritableEnvironments()
			if jsonOut {
				return printJSON(map[string]any{"agent": name, "detected": name != "", "writableEnvironments": writable})
			}
			if name == "" {
				fmt.Println("Aucun agent IA détecté : pas de restriction.")
			} else {
				fmt.Printf("Agent détecté : %s\n", name)
			}
			fmt.Printf("Écriture autorisée aux agents : %s (\"agents.write\" dans .coffre.json)\n", fmt.Sprint(writable))
			fmt.Println("Toujours refusé aux agents : --reveal, secrets download / export, secrets pull.")
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "résultat en JSON")
	return c
}

// setupAgentFiles écrit les instructions ; le nom du projet est demandé à l'API si possible.
func setupAgentFiles(ctx context.Context, local *config.Project, dir string) ([]agentfiles.Written, error) {
	name := local.ProjectID
	if client, _, err := authedClient(); err == nil {
		if project, err := client.GetProject(ctx, local.ProjectID); err == nil {
			name = project.Name
		}
	}
	return agentfiles.Setup(dir, agentfiles.Params{
		ProjectName:  name,
		DefaultEnv:   local.Environment,
		WritableEnvs: local.AgentWritableEnvironments(),
	})
}

func printAgentFiles(written []agentfiles.Written) {
	labels := map[string]string{"created": "créé", "updated": "mis à jour", "unchanged": "inchangé"}
	fmt.Println("✓ instructions pour les agents IA :")
	for _, w := range written {
		rel := w.Path
		if wd, err := filepath.Abs("."); err == nil {
			if r, err := filepath.Rel(wd, w.Path); err == nil {
				rel = r
			}
		}
		fmt.Printf("  %-36s %s\n", rel, labels[w.Action])
	}
	fmt.Println("  À commiter : ces fichiers ne contiennent aucun secret.")
}
