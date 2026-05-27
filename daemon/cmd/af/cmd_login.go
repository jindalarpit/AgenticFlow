package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/agenticflow/agenticflow/daemon/internal/cli"
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

// GenerateState generates a cryptographically random CSRF state token.
// It returns a 32-character hex string (16 random bytes encoded as hex).
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// BuildLoginURL constructs the login URL with cli_callback and cli_state query parameters.
// serverURL is the base server URL, callbackURL is the local callback endpoint,
// and state is the CSRF token.
func BuildLoginURL(serverURL, callbackURL, state string) string {
	base := strings.TrimRight(serverURL, "/") + "/login"
	return fmt.Sprintf("%s?cli_callback=%s&cli_state=%s",
		base,
		url.QueryEscape(callbackURL),
		url.QueryEscape(state),
	)
}

// ValidateTokenFormat checks whether a token string has the required "af_" prefix.
// Returns true if the token starts with "af_", false otherwise.
func ValidateTokenFormat(token string) bool {
	return strings.HasPrefix(token, "af_")
}

func runLoginBrowser(cmd *cobra.Command) error {
	serverURL := resolveServerURL(cmd)
	if serverURL == "" {
		return fmt.Errorf("no server URL configured. Run 'af setup' first")
	}

	// Start local HTTP server on a random available port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start local callback server: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// Generate CSRF state token (32 hex chars from crypto/rand).
	state, err := GenerateState()
	if err != nil {
		return err
	}

	// Construct login URL with cli_callback and cli_state query parameters.
	loginURL := BuildLoginURL(serverURL, callbackURL, state)

	// Channel to receive the JWT from the browser callback.
	jwtCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Validate state parameter matches the generated CSRF token.
		returnedState := r.URL.Query().Get("state")
		if returnedState != state {
			http.Error(w, "invalid state parameter", http.StatusBadRequest)
			return
		}

		// Extract JWT from query params.
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(callbackSuccessHTML))
		jwtCh <- token
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer srv.Close()

	// Open browser to login URL; print URL to stderr if browser fails to open.
	fmt.Fprintln(os.Stderr, "Opening browser to authenticate...")
	if err := openBrowser(loginURL); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open browser automatically.\n")
	}
	fmt.Fprintf(os.Stderr, "If the browser didn't open, visit:\n  %s\n\nWaiting for authentication...\n", loginURL)

	// Wait for the JWT from the callback (timeout: 5 minutes).
	var jwtToken string
	select {
	case jwtToken = <-jwtCh:
	case err := <-errCh:
		return fmt.Errorf("local server error: %w", err)
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("timed out waiting for authentication (5 minutes)")
	}

	// Exchange JWT for PAT via POST /api/tokens with name "CLI (hostname)" and 90-day expiry.
	client := cli.NewAPIClient(serverURL, jwtToken)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	patName := fmt.Sprintf("CLI (%s)", hostname)

	var patResp struct {
		Token string `json:"token"`
	}
	err = client.PostJSON(ctx, "/api/tokens", map[string]any{
		"name":            patName,
		"expires_in_days": 90,
	}, &patResp)
	if err != nil {
		return fmt.Errorf("failed to create access token: %w", err)
	}

	// Verify PAT via GET /api/me.
	patClient := cli.NewAPIClient(serverURL, patResp.Token)
	var me struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := patClient.GetJSON(ctx, "/api/me", &me); err != nil {
		return fmt.Errorf("token verification failed: %w", err)
	}

	// Save PAT and server URL to config atomically.
	cfg := cli.LoadConfig()
	cfg.ServerURL = serverURL
	cfg.Token = patResp.Token
	cfg.UserEmail = me.Email
	cfg.TokenExpiresAt = time.Now().Add(90 * 24 * time.Hour)

	if err := cli.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Authenticated as %s (%s)\n", me.Name, me.Email)
	fmt.Fprintf(os.Stderr, "Token saved to ~/.agenticflow/config.json\n")
	return nil
}

func runLoginToken(cmd *cobra.Command, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required (use --token af_...)")
	}

	if !ValidateTokenFormat(token) {
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

// callbackSuccessHTML is the HTML page shown in the browser after successful authentication.
const callbackSuccessHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>AgenticFlow — Authenticated</title>
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  @media (prefers-color-scheme: dark) {
    :root { --bg: #0b0b0f; --card-bg: #16161d; --border: rgba(255,255,255,0.10); --fg: #f5f5f5; --fg2: #a1a1aa; --accent: #22c55e; --accent-bg: rgba(34,197,94,0.12); }
  }
  @media (prefers-color-scheme: light) {
    :root { --bg: #f8f8fa; --card-bg: #ffffff; --border: rgba(0,0,0,0.08); --fg: #0f0f12; --fg2: #71717a; --accent: #16a34a; --accent-bg: rgba(22,163,74,0.08); }
  }
  body { font-family: -apple-system, "Segoe UI", Helvetica, Arial, sans-serif; background: var(--bg); color: var(--fg); display: flex; align-items: center; justify-content: center; min-height: 100vh; }
  .card { width: 100%; max-width: 380px; border: 1px solid var(--border); border-radius: 12px; background: var(--card-bg); padding: 40px 32px; text-align: center; }
  .icon-wrap { width: 48px; height: 48px; margin: 0 auto 24px; background: var(--accent-bg); border-radius: 50%; display: flex; align-items: center; justify-content: center; }
  .icon-wrap svg { width: 24px; height: 24px; color: var(--accent); }
  h1 { font-size: 20px; font-weight: 600; margin-bottom: 8px; }
  p { font-size: 14px; color: var(--fg2); line-height: 1.5; }
  .hint { margin-top: 24px; font-size: 13px; color: var(--fg2); opacity: 0.7; }
</style>
</head>
<body>
  <div class="card">
    <div class="icon-wrap">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/></svg>
    </div>
    <h1>Authentication successful</h1>
    <p>You can close this tab and return to the terminal.</p>
    <p class="hint">Your CLI session is now authenticated.</p>
  </div>
  <script>setTimeout(function(){window.close()},3000)</script>
</body>
</html>`
