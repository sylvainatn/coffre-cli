package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"

	"github.com/coffre-dev/coffre-cli/internal/api"
	"github.com/coffre-dev/coffre-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	appVersion     string
	builtinAPIURL  string
	flagAPIURL     string
	errNotLoggedIn = errors.New("non connecté : lancez `coffre login` (ou définissez COFFRE_TOKEN)")
)

// exitCodeError permet à `coffre run` de propager le code de sortie de la commande.
type exitCodeError struct{ code int }

func (e *exitCodeError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "coffre",
		Short: "Vos variables d'environnement, chiffrées et hébergées en France",
		Long: `coffre injecte les secrets de votre projet dans vos applications,
sans fichier .env sur le poste.

  coffre login                         # relie la CLI à votre compte
  coffre init proj_xxx --env development
  coffre run -- npm run dev`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "URL de l'API (défaut : COFFRE_API_URL ou valeur enregistrée)")

	root.AddCommand(
		newLoginCmd(),
		newLogoutCmd(),
		newWhoamiCmd(),
		newInitCmd(),
		newRunCmd(),
		newSecretsCmd(),
		newVersionCmd(),
	)
	return root
}

func Execute(version, defaultAPIURL string) int {
	appVersion = version
	builtinAPIURL = defaultAPIURL
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		var exitErr *exitCodeError
		if errors.As(err, &exitErr) {
			return exitErr.code
		}
		fmt.Fprintln(os.Stderr, "✗", err)
		return 1
	}
	return 0
}

/* ------------------------------ Helpers ---------------------------------- */

func userAgent() string {
	return fmt.Sprintf("coffre-cli/%s (%s/%s)", appVersion, runtime.GOOS, runtime.GOARCH)
}

// resolveAPIURL : --api-url > COFFRE_API_URL > config > valeur compilée.
func resolveAPIURL(g *config.Global) string {
	switch {
	case flagAPIURL != "":
		return flagAPIURL
	case os.Getenv("COFFRE_API_URL") != "":
		return os.Getenv("COFFRE_API_URL")
	case g != nil && g.APIURL != "":
		return g.APIURL
	default:
		return builtinAPIURL
	}
}

// authedClient renvoie un client authentifié : COFFRE_TOKEN (CI) > config.
func authedClient() (*api.Client, *config.Global, error) {
	g, err := config.LoadGlobal()
	if err != nil {
		return nil, nil, err
	}
	token := os.Getenv("COFFRE_TOKEN")
	if token == "" {
		token = g.Token
	}
	if token == "" {
		return nil, nil, errNotLoggedIn
	}
	return api.New(resolveAPIURL(g), token, userAgent()), g, nil
}

// resolveTarget détermine le projet et l'environnement :
// flags > variables COFFRE_PROJECT / COFFRE_ENV > .coffre.json.
func resolveTarget(projectFlag, envFlag string) (string, string, error) {
	project, env := projectFlag, envFlag
	if project == "" {
		project = os.Getenv("COFFRE_PROJECT")
	}
	if env == "" {
		env = os.Getenv("COFFRE_ENV")
	}
	if project == "" || env == "" {
		local, _, err := config.FindProject()
		if err != nil {
			return "", "", err
		}
		if local != nil {
			if project == "" {
				project = local.ProjectID
			}
			if env == "" {
				env = local.Environment
			}
		}
	}
	if project == "" {
		return "", "", errors.New("aucun projet configuré ici : lancez `coffre init <projectId>` (ou utilisez --project)")
	}
	if env == "" {
		env = "development"
	}
	return project, env, nil
}
