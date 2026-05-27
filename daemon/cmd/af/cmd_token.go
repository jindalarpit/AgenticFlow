package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/agenticflow/agenticflow/daemon/internal/cli"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Display and verify stored authentication token",
	Long: `Show the stored authentication token (truncated) and verify it against the server.

If no token is stored, suggests running 'af login' to authenticate.
If the token is expired or invalid, suggests re-authentication.`,
	Args: cobra.NoArgs,
	RunE: runToken,
}

// TruncateToken returns a display-safe version of the token.
// If the token is 12 or more characters, it returns the first 12 characters
// followed by "...". Otherwise, it returns the full token.
func TruncateToken(token string) string {
	if len(token) >= 12 {
		return token[:12] + "..."
	}
	return token
}

func runToken(cmd *cobra.Command, _ []string) error {
	cfg := cli.LoadConfig()

	if cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "Not authenticated.")
		fmt.Fprintln(os.Stderr, "Run 'af login' to authenticate.")
		return nil
	}

	serverURL := cfg.ServerURL
	if serverURL == "" {
		serverURL = resolveServerURL(cmd)
	}

	// Display token (truncated) and server URL.
	fmt.Fprintf(os.Stdout, "Token:   %s\n", TruncateToken(cfg.Token))
	fmt.Fprintf(os.Stdout, "Server:  %s\n", serverURL)

	// Verify token validity via GET /api/me.
	if serverURL == "" {
		fmt.Fprintln(os.Stderr, "\nNo server URL configured — cannot verify token.")
		return nil
	}

	client := cli.NewAPIClient(serverURL, cfg.Token)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var me struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := client.GetJSON(ctx, "/api/me", &me); err != nil {
		var httpErr *cli.HTTPError
		if errors.As(err, &httpErr) && httpErr.IsUnauthorized() {
			fmt.Fprintln(os.Stderr, "\nToken is invalid or expired.")
			fmt.Fprintln(os.Stderr, "Run 'af login' to re-authenticate.")
			return nil
		}
		// Network or other error — still report but don't fail hard.
		fmt.Fprintf(os.Stderr, "\nCould not verify token: %v\n", err)
		return nil
	}

	fmt.Fprintf(os.Stdout, "Status:  Valid\n")
	fmt.Fprintf(os.Stdout, "User:    %s (%s)\n", me.Name, me.Email)
	return nil
}
