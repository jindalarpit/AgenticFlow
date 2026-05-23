package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/agenticflow/agenticflow/internal/cli"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the AgenticFlow server",
	Long: `Log in to AgenticFlow via browser OAuth or a personal access token.

Examples:
  af login                    # Open browser for OAuth
  af login --token af_...     # Authenticate with a PAT directly`,
	Args: cobra.NoArgs,
	RunE: runLogin,
}

func init() {
	loginCmd.Flags().String("token", "", "Authenticate using a personal access token (e.g. --token af_...)")
}

func runLogin(cmd *cobra.Command, _ []string) error {
	tokenFlag, _ := cmd.Flags().GetString("token")
	if cmd.Flags().Changed("token") || tokenFlag != "" {
		return runLoginToken(cmd, tokenFlag)
	}
	return runLoginBrowser(cmd)
}

func runLoginBrowser(cmd *cobra.Command) error {
	cfg := cli.LoadConfig()
	serverURL := resolveServerURL(cmd)

	if serverURL == "" {
		return fmt.Errorf("no server URL configured. Run 'af setup' first")
	}

	loginURL := strings.TrimRight(serverURL, "/") + "/login"

	fmt.Fprintln(os.Stderr, "Opening browser to authenticate...")
	if err := openBrowser(loginURL); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open browser automatically.\n")
	}
	fmt.Fprintf(os.Stderr, "If the browser didn't open, visit:\n  %s\n", loginURL)
	fmt.Fprintf(os.Stderr, "\nWaiting for authentication (timeout: 120s)...\n")

	// In a full implementation, this would start a local HTTP server to receive
	// the OAuth callback. For now, print instructions.
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "After authenticating in the browser, copy your token and run:")
	fmt.Fprintf(os.Stderr, "  af login --token <your-token>\n")

	_ = cfg
	_ = time.After(120 * time.Second)

	return nil
}

func runLoginToken(cmd *cobra.Command, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required (use --token af_...)")
	}

	if !strings.HasPrefix(token, "af_") {
		return fmt.Errorf("invalid token format: must start with 'af_'")
	}

	serverURL := resolveServerURL(cmd)
	if serverURL == "" {
		return fmt.Errorf("no server URL configured. Run 'af setup' first")
	}

	// Validate the token against the server via GET /api/me.
	client := cli.NewAPIClient(serverURL, token)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var me struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := client.GetJSON(ctx, "/api/me", &me); err != nil {
		var httpErr *cli.HTTPError
		if errors.As(err, &httpErr) && httpErr.IsUnauthorized() {
			return fmt.Errorf("invalid token: server rejected authentication")
		}
		return fmt.Errorf("failed to validate token: %w", err)
	}

	// Token is valid — store it in config.
	cfg := cli.LoadConfig()
	cfg.ServerURL = serverURL
	cfg.Token = token
	cfg.UserEmail = me.Email

	if err := cli.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Authenticated as %s (%s)\n", me.Name, me.Email)
	fmt.Fprintf(os.Stderr, "Token saved to ~/.agenticflow/config.json\n")
	return nil
}

// resolveServerURL returns the server URL from flags, env, or config.
func resolveServerURL(cmd *cobra.Command) string {
	// 1. CLI flag
	if v, _ := cmd.Flags().GetString("server-url"); v != "" {
		return v
	}
	// 2. Environment variable
	if v := strings.TrimSpace(os.Getenv("AF_SERVER_URL")); v != "" {
		return v
	}
	// 3. Config file
	cfg := cli.LoadConfig()
	return cfg.ServerURL
}

// openBrowser opens the given URL in the default browser.
func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return exec.Command(cmd, args...).Start()
}
