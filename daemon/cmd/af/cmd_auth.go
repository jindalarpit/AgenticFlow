package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/agenticflow/agenticflow/daemon/internal/cli"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication state",
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE:  runAuthStatus,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored authentication token",
	RunE:  runAuthLogout,
}

func init() {
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
}

func runAuthStatus(_ *cobra.Command, _ []string) error {
	cfg := cli.LoadConfig()

	serverURL := cfg.ServerURL
	if serverURL == "" {
		serverURL = "(not configured)"
	}

	fmt.Fprintf(os.Stdout, "Server:  %s\n", serverURL)

	if cfg.Token == "" {
		fmt.Fprintln(os.Stdout, "Status:  Not authenticated")
		fmt.Fprintln(os.Stderr, "\nRun 'af login' to authenticate.")
		return nil
	}

	// Display user email if available.
	userEmail := cfg.UserEmail
	if userEmail == "" {
		userEmail = "(unknown)"
	}
	fmt.Fprintf(os.Stdout, "User:    %s\n", userEmail)

	// Display token info (masked).
	tokenPreview := cfg.Token
	if len(tokenPreview) > 12 {
		tokenPreview = tokenPreview[:12] + "..."
	}
	fmt.Fprintf(os.Stdout, "Token:   %s\n", tokenPreview)

	// Display token validity.
	if !cfg.TokenExpiresAt.IsZero() {
		remaining := time.Until(cfg.TokenExpiresAt)
		if remaining <= 0 {
			fmt.Fprintln(os.Stdout, "Expires: EXPIRED")
			fmt.Fprintln(os.Stderr, "\nToken has expired. Run 'af login' to re-authenticate.")
		} else {
			days := int(remaining.Hours() / 24)
			fmt.Fprintf(os.Stdout, "Expires: %s (%d days remaining)\n",
				cfg.TokenExpiresAt.Format(time.RFC3339), days)
		}
	}

	return nil
}

func runAuthLogout(_ *cobra.Command, _ []string) error {
	cfg := cli.LoadConfig()

	if cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "Not authenticated.")
		return nil
	}

	cfg.Token = ""
	cfg.TokenExpiresAt = time.Time{}
	cfg.UserEmail = ""

	if err := cli.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Token removed. You are now logged out.")
	return nil
}
