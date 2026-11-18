package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/coffre-dev/coffre-cli/internal/api"
	"github.com/coffre-dev/coffre-cli/internal/config"
	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	var token string
	var noBrowser bool

	c := &cobra.Command{
		Use:   "login",
		Short: "Relie la CLI à votre compte (via le navigateur)",
		Example: `  coffre login
  coffre login --api-url https://api.mon-instance.fr
  coffre login --token cfc_xxx   # jeton créé depuis le dashboard`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			g, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			apiURL := resolveAPIURL(g)
			ctx := cmd.Context()

			if token == "" {
				token, err = deviceLogin(ctx, apiURL, noBrowser)
				if err != nil {
					return err
				}
			}

			client := api.New(apiURL, token, userAgent())
			user, err := client.Me(ctx)
			if err != nil {
				return fmt.Errorf("jeton refusé : %w", err)
			}

			g.APIURL, g.Token, g.Email = apiURL, token, user.Email
			if err := config.SaveGlobal(g); err != nil {
				return err
			}
			fmt.Printf("✓ Connecté en tant que %s\n", user.Email)
			return nil
		},
	}
	c.Flags().StringVar(&token, "token", "", "utiliser un jeton existant au lieu du navigateur")
	c.Flags().BoolVar(&noBrowser, "no-browser", false, "ne pas ouvrir le navigateur automatiquement")
	return c
}

func deviceLogin(ctx context.Context, apiURL string, noBrowser bool) (string, error) {
	client := api.New(apiURL, "", userAgent())
	hostname, _ := os.Hostname()
	device, err := client.StartDeviceAuth(ctx, hostname)
	if err != nil {
		return "", err
	}

	fmt.Printf("\n  Code de confirmation : %s\n\n", device.UserCode)
	fmt.Printf("  Validez la connexion dans votre navigateur :\n  %s\n\n", device.VerificationURLComplete)
	if !noBrowser {
		_ = openBrowser(device.VerificationURLComplete)
	}
	fmt.Println("  En attente de validation… (Ctrl+C pour annuler)")

	interval := time.Duration(max(device.Interval, 1)) * time.Second
	deadline := time.Now().Add(time.Duration(device.ExpiresIn) * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
		token, _, err := client.PollDeviceToken(ctx, device.DeviceCode)
		if err == nil {
			return token, nil
		}
		var apiErr *api.Error
		if errors.As(err, &apiErr) && apiErr.Code == "authorization_pending" {
			continue
		}
		return "", err
	}
	return "", errors.New("la demande de connexion a expiré, relancez `coffre login`")
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Révoque le jeton de la CLI et l'efface du poste",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			g, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			if g.Token == "" {
				fmt.Println("Déjà déconnecté.")
				return nil
			}
			// Révocation côté serveur (best effort : on efface localement quoi qu'il arrive)
			_ = api.New(resolveAPIURL(g), g.Token, userAgent()).Logout(cmd.Context())
			g.Token, g.Email = "", ""
			if err := config.SaveGlobal(g); err != nil {
				return err
			}
			fmt.Println("✓ Déconnecté")
			return nil
		},
	}
}

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Affiche le compte connecté",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := authedClient()
			if err != nil {
				return err
			}
			user, err := client.Me(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Printf("%s  (%s)\n", user.Email, strings.TrimRight(client.BaseURL, "/"))
			return nil
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Affiche la version",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			fmt.Printf("coffre %s (%s/%s)\n", appVersion, runtime.GOOS, runtime.GOARCH)
		},
	}
}
