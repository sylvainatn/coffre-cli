package cli

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/coffre-dev/coffre-cli/internal/api"
	"github.com/coffre-dev/coffre-cli/internal/dotenv"
	"github.com/coffre-dev/coffre-cli/internal/envdiff"
	"github.com/coffre-dev/coffre-cli/internal/localfile"
	"github.com/spf13/cobra"
)

// targetFunc renvoie le client et l'environnement visé (flags > COFFRE_* > .coffre.json).
type targetFunc func() (*api.Client, string, string, error)

// Noms acceptés par l'API (plus stricts que ceux tolérés dans un .env).
var apiKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

const fileFlagUsage = "fichier .env local"

// readLocal lit et analyse le fichier local. missing = le fichier n'existe pas.
func readLocal(path string) (f *dotenv.File, missing bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		f, _ = dotenv.Parse("")
		return f, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	f, err = dotenv.Parse(string(data))
	if err != nil {
		return nil, false, fmt.Errorf("%s : %w", path, err)
	}
	return f, false, nil
}

func remoteValues(cmd *cobra.Command, client *api.Client, projectID, env string) (map[string]string, error) {
	secrets, err := client.GetSecrets(cmd.Context(), projectID, env)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(secrets))
	for _, s := range secrets {
		out[s.Key] = s.Value
	}
	return out, nil
}

// writeLocal écrit le fichier (0600, atomique) et l'ajoute au .gitignore si besoin.
func writeLocal(path, content string) error {
	if err := localfile.Write(path, content); err != nil {
		return err
	}
	if gitignore, err := localfile.EnsureGitIgnored(path); err == nil && gitignore != "" {
		fmt.Fprintf(os.Stderr, "  %s ajouté à %s\n", path, gitignore)
	}
	return nil
}

func printKeys(symbol, label string, keys []string) {
	for _, k := range keys {
		fmt.Printf("  %s %-32s %s\n", symbol, k, label)
	}
}

/* --------------------------------- diff ---------------------------------- */

func newDiffCmd(target targetFunc) *cobra.Command {
	var file string
	var exitCode bool
	c := &cobra.Command{
		Use:   "diff",
		Short: "Compare le fichier .env local aux secrets de Coffre (valeurs jamais affichées)",
		Example: `  coffre secrets diff
  coffre secrets diff --env staging --file .env.staging
  coffre secrets diff --exit-code   # code 1 s'il y a des différences (CI)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, projectID, envSlug, err := target()
			if err != nil {
				return err
			}
			f, missing, err := readLocal(file)
			if err != nil {
				return err
			}
			if missing {
				return fmt.Errorf("%s introuvable (pour le créer : coffre secrets pull)", file)
			}
			remote, err := remoteValues(cmd, client, projectID, envSlug)
			if err != nil {
				return err
			}
			d := envdiff.Compare(f.Values(), remote)
			if d.Empty() {
				fmt.Printf("✓ %s identique à %s/%s (%d variable(s))\n", file, projectID, envSlug, len(remote))
				return nil
			}
			fmt.Printf("%s ↔ %s/%s\n", file, projectID, envSlug)
			printKeys("+", "dans Coffre, absente du fichier", d.OnlyRemote)
			printKeys("-", "dans le fichier, absente de Coffre", d.OnlyLocal)
			printKeys("~", "valeurs différentes", d.Changed)
			fmt.Println()
			if len(d.OnlyRemote)+len(d.Changed) > 0 {
				fmt.Println("  coffre secrets pull   récupère les variables de Coffre")
			}
			if len(d.OnlyLocal) > 0 {
				fmt.Println("  coffre secrets push   envoie les variables locales")
			}
			if exitCode {
				return &exitCodeError{code: 1}
			}
			return nil
		},
	}
	c.Flags().StringVarP(&file, "file", "f", ".env", fileFlagUsage)
	c.Flags().BoolVar(&exitCode, "exit-code", false, "terminer avec le code 1 s'il y a des différences")
	return c
}

/* --------------------------------- pull ---------------------------------- */

func newPullCmd(target targetFunc) *cobra.Command {
	var file string
	var overwrite, dryRun bool
	c := &cobra.Command{
		Use:   "pull",
		Short: "Ajoute au fichier .env les variables de Coffre qu'il n'a pas",
		Long: `Récupère les secrets de l'environnement et les ajoute au fichier .env local.

- Les variables absentes du fichier sont ajoutées à la fin, sans toucher au reste
  (commentaires, ordre, valeurs locales).
- Une variable dont la valeur locale diffère n'est remplacée qu'avec --overwrite,
  ou après confirmation, variable par variable, dans un terminal.
- Rien n'est jamais supprimé du fichier.

Le fichier est écrit avec des droits 0600 et ajouté au .gitignore s'il ne l'est pas.
Pour ne pas avoir de fichier du tout : coffre run -- <commande>.`,
		Example: `  coffre secrets pull
  coffre secrets pull --env staging --file .env.staging
  coffre secrets pull --overwrite   # Coffre fait foi pour toutes les valeurs
  coffre secrets pull --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, projectID, envSlug, err := target()
			if err != nil {
				return err
			}
			f, missing, err := readLocal(file)
			if err != nil {
				return err
			}
			remote, err := remoteValues(cmd, client, projectID, envSlug)
			if err != nil {
				return err
			}
			d := envdiff.Compare(f.Values(), remote)
			if len(d.OnlyRemote) == 0 && len(d.Changed) == 0 {
				fmt.Printf("✓ %s est à jour avec %s/%s\n", file, projectID, envSlug)
				hintPush(d.OnlyLocal)
				return nil
			}

			// Valeurs différentes : Coffre ne les impose qu'avec l'accord de l'utilisateur
			var replace, kept []string
			askEach := !overwrite && !dryRun && interactive() && len(d.Changed) > 0
			if askEach {
				fmt.Fprintf(os.Stderr, "%d variable(s) ont une valeur locale différente de celle de Coffre.\n", len(d.Changed))
			}
		changed:
			for i, k := range d.Changed {
				switch {
				case overwrite:
					replace = append(replace, k)
				case !askEach:
					kept = append(kept, k)
				default:
					switch ask(fmt.Sprintf("  %s : remplacer la valeur locale par celle de Coffre ? [o/N/t=tout/q=garder le reste] ", k)) {
					case "o", "oui", "y", "yes":
						replace = append(replace, k)
					case "t", "tout", "a", "all":
						replace = append(replace, d.Changed[i:]...)
						break changed
					case "q":
						kept = append(kept, d.Changed[i:]...)
						break changed
					default:
						kept = append(kept, k)
					}
				}
			}

			if dryRun {
				fmt.Printf("%s ← %s/%s (simulation, rien n'est écrit)\n", file, projectID, envSlug)
				printKeys("+", "serait ajoutée", d.OnlyRemote)
				printKeys("~", "serait remplacée", replace)
				printKeys("=", "valeur locale conservée", kept)
				return nil
			}
			if len(d.OnlyRemote) == 0 && len(replace) == 0 {
				fmt.Printf("Rien à écrire : valeurs locales conservées (%s).\n", strings.Join(kept, ", "))
				return nil
			}

			for _, k := range replace {
				f.Set(k, remote[k])
			}
			if len(d.OnlyRemote) > 0 {
				f.AppendComment(fmt.Sprintf("Ajoutées par coffre depuis %s/%s le %s", projectID, envSlug, time.Now().Format("02/01/2006 15:04")))
				for _, k := range d.OnlyRemote {
					f.Set(k, remote[k])
				}
			}
			if err := writeLocal(file, f.String()); err != nil {
				return err
			}

			verb := "mis à jour"
			if missing {
				verb = "créé"
			}
			fmt.Printf("✓ %s %s depuis %s/%s : %d ajoutée(s), %d remplacée(s)\n", file, verb, projectID, envSlug, len(d.OnlyRemote), len(replace))
			if len(kept) > 0 {
				fmt.Printf("  valeur locale conservée : %s (--overwrite pour prendre celle de Coffre)\n", strings.Join(kept, ", "))
			}
			hintPush(d.OnlyLocal)
			fmt.Fprintln(os.Stderr, "  astuce : coffre run -- <commande> injecte les secrets sans fichier .env")
			return nil
		},
	}
	c.Flags().StringVarP(&file, "file", "f", ".env", fileFlagUsage)
	c.Flags().BoolVar(&overwrite, "overwrite", false, "remplacer les valeurs locales qui diffèrent de Coffre")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "afficher ce qui serait fait, sans écrire")
	return c
}

func hintPush(onlyLocal []string) {
	if len(onlyLocal) > 0 {
		fmt.Printf("  %d variable(s) seulement en local (%s) : coffre secrets push pour les envoyer\n", len(onlyLocal), strings.Join(onlyLocal, ", "))
	}
}

/* --------------------------------- push ---------------------------------- */

func newPushCmd(target targetFunc) *cobra.Command {
	var file string
	var update, dryRun, yes bool
	c := &cobra.Command{
		Use:   "push",
		Short: "Envoie dans Coffre les variables du fichier .env qu'il n'a pas",
		Long: `Envoie les variables du fichier .env local qui n'existent pas encore dans Coffre.

- Une variable qui existe déjà dans Coffre n'est jamais écrasée, même si un collègue
  l'a créée entre-temps (vérification faite par l'API, dans une seule transaction).
- Avec --update, les variables dont la valeur diffère sont aussi mises à jour
  (l'ancienne valeur reste dans l'historique et peut être restaurée).
- Rien n'est jamais supprimé de Coffre.

En staging et en production, une confirmation est demandée (--yes pour la CI).`,
		Example: `  coffre secrets push
  coffre secrets push --env staging --file .env.staging
  coffre secrets push --update --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, projectID, envSlug, err := target()
			if err != nil {
				return err
			}
			f, missing, err := readLocal(file)
			if err != nil {
				return err
			}
			if missing {
				return fmt.Errorf("%s introuvable", file)
			}
			local := f.Values()

			var invalid []string
			for k := range local {
				if !apiKeyPattern.MatchString(k) {
					invalid = append(invalid, k)
					delete(local, k)
				}
			}
			sort.Strings(invalid)
			if len(invalid) > 0 {
				fmt.Fprintf(os.Stderr, "⚠ ignorée(s), nom non accepté par Coffre (lettres, chiffres et _) : %s\n", strings.Join(invalid, ", "))
			}

			remote, err := remoteValues(cmd, client, projectID, envSlug)
			if err != nil {
				return err
			}
			d := envdiff.Compare(local, remote)
			toUpdate := []string{}
			if update {
				toUpdate = d.Changed
			}
			if len(d.OnlyLocal) == 0 && len(toUpdate) == 0 {
				fmt.Printf("✓ rien à envoyer : %s/%s a déjà toutes les variables de %s\n", projectID, envSlug, file)
				if len(d.Changed) > 0 {
					fmt.Printf("  %d valeur(s) différente(s) (%s) : --update pour les envoyer\n", len(d.Changed), strings.Join(d.Changed, ", "))
				}
				return nil
			}

			fmt.Printf("%s → %s/%s\n", file, projectID, envSlug)
			printKeys("+", "nouvelle", d.OnlyLocal)
			printKeys("~", "mise à jour", toUpdate)
			if dryRun {
				fmt.Println("(simulation, rien n'est envoyé)")
				return nil
			}

			if !yes && (sensitiveEnv(envSlug) || len(toUpdate) > 0) {
				if !interactive() {
					return fmt.Errorf("écriture dans %s : relancez avec --yes pour confirmer", envSlug)
				}
				if !confirm(fmt.Sprintf("Envoyer ces %d variable(s) dans %s ?", len(d.OnlyLocal)+len(toUpdate), envSlug)) {
					return errors.New("annulé")
				}
			}

			created, skipped := 0, []string(nil)
			if len(d.OnlyLocal) > 0 {
				secrets := make([]api.Secret, 0, len(d.OnlyLocal))
				for _, k := range d.OnlyLocal {
					secrets = append(secrets, api.Secret{Key: k, Value: local[k]})
				}
				res, err := client.CreateMissingSecrets(cmd.Context(), projectID, envSlug, secrets)
				if err != nil {
					return err
				}
				created, skipped = len(res.Created), res.Skipped
			}
			if len(toUpdate) > 0 {
				secrets := make([]api.Secret, 0, len(toUpdate))
				for _, k := range toUpdate {
					secrets = append(secrets, api.Secret{Key: k, Value: local[k]})
				}
				if err := client.SetSecrets(cmd.Context(), projectID, envSlug, secrets); err != nil {
					return err
				}
			}

			fmt.Printf("✓ %s/%s : %d créée(s), %d mise(s) à jour\n", projectID, envSlug, created, len(toUpdate))
			if len(skipped) > 0 {
				fmt.Printf("  déjà créée(s) entre-temps par quelqu'un d'autre, non écrasée(s) : %s\n", strings.Join(skipped, ", "))
			}
			if len(d.Changed) > 0 && !update {
				fmt.Printf("  %d valeur(s) différente(s) non envoyée(s) (%s) : --update pour les envoyer\n", len(d.Changed), strings.Join(d.Changed, ", "))
			}
			return nil
		},
	}
	c.Flags().StringVarP(&file, "file", "f", ".env", fileFlagUsage)
	c.Flags().BoolVar(&update, "update", false, "mettre aussi à jour les variables dont la valeur diffère")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "afficher ce qui serait envoyé, sans rien envoyer")
	c.Flags().BoolVarP(&yes, "yes", "y", false, "ne pas demander de confirmation")
	return c
}
