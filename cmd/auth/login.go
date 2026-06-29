package auth

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"bifu-cli/internal/client"
	"bifu-cli/internal/clifconfig"
)

// noBaseURLErr explains that the selected profile has no endpoint configured,
// naming the profile and showing how to initialise it for the intended
// environment — rather than hardcoding `--env dev`, which is misleading when
// the user asked for another profile/env.
func noBaseURLErr(name string) error {
	if name == "" {
		name = "default"
	}
	return fmt.Errorf("profile %q has no base_url — initialise it first:\n  bifu-cli config init --profile %s --env <dev|staging|prod>", name, name)
}

// newLoginCmd builds the `auth login` subcommand.
func newLoginCmd(load LoadFn) *cobra.Command {
	var username, password string
	var device bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login and save session cookie (email/password, or --device)",
		Long: `Login to BifuFX and save the session cookie to the active profile.

Default: email/password — a verification code is sent to your email and you
are prompted to enter it.

--device: scan-to-login (like ` + "`gh auth login`" + `). The CLI prints a QR code;
scan it with the Bifu app (already logged in) to approve, and the CLI polls
until it receives the session cookie. No password is typed in the terminal.
Backed by the existing scan-to-login endpoints
(/user/login/qr_code_get and /user/login/qr_code_check).`,
		Example: `  bifu-cli auth login
  bifu-cli auth login --username user@example.com
  bifu-cli --profile dev auth login
  bifu-cli auth login --device`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if device {
				return runDeviceLogin(load)
			}

			profile, _, err := load()
			if err != nil {
				return err
			}
			baseURL := profile.BaseURL
			if baseURL == "" {
				return noBaseURLErr(profile.Name)
			}

			// ── Step 0: collect credentials ──────────────────────────────────
			reader := bufio.NewReader(os.Stdin)

			if username == "" {
				fmt.Print("Username (email): ")
				line, _ := reader.ReadString('\n')
				username = strings.TrimSpace(line)
			}

			// Prefer an env var over the --password flag for non-interactive use:
			// a flag value lands in the process list and shell history
			// (BIFU-CLI-202606-019).
			if password == "" {
				password = os.Getenv("BIFU_PASSWORD")
			}

			if password == "" {
				if term.IsTerminal(int(os.Stdin.Fd())) {
					fmt.Print("Password: ")
					bytePass, err := term.ReadPassword(int(os.Stdin.Fd()))
					fmt.Println() // newline after hidden input
					if err != nil {
						return fmt.Errorf("read password: %w", err)
					}
					password = string(bytePass)
				} else {
					// non-interactive (piped input)
					fmt.Print("Password: ")
					line, _ := reader.ReadString('\n')
					password = strings.TrimSpace(line)
				}
			}

			if username == "" || password == "" {
				return fmt.Errorf("username and password are required")
			}

			// ── Step 1: POST /user/login ──────────────────────────────────────
			termType := profile.Auth.TerminalType
			if termType == "" {
				termType = "API"
			}
			fmt.Printf("Logging in as %s...\n", username)
			issueID, err := doLogin(baseURL, username, password, termType)
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			var cookieName, cookieVal, userID string

			// If issueID has COOKIE: prefix, no 2FA needed — cookie is inline (JSON-encoded)
			if strings.HasPrefix(issueID, "COOKIE:") {
				cookieName, cookieVal = extractCookie(strings.TrimPrefix(issueID, "COOKIE:"))
			} else {
				fmt.Println("✓ Password accepted, verification code sent to email")

				// ── Step 2: prompt for verification code ──────────────────────────
				fmt.Print("Verification code: ")
				codeLine, _ := reader.ReadString('\n')
				code := strings.TrimSpace(codeLine)
				if code == "" {
					return fmt.Errorf("verification code is required")
				}

				// ── Step 3: POST /user/login_check ────────────────────────────────
				cookieName, cookieVal, userID, err = doLoginCheck(baseURL, issueID, code, termType)
				if err != nil {
					return fmt.Errorf("verification failed: %w", err)
				}
			}

			// ── Step 4: persist cookie to the profile we logged in as ─────────
			// Save to the profile resolved by load() (which honours --profile),
			// not cfg.Active() — otherwise `--profile X auth login` would
			// authenticate against X but write the cookie to the on-disk
			// active profile.
			cfg, err := clifconfig.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			p := cfg.EnsureProfile(profile.Name)
			p.Auth.AuthCookie = cookieVal
			p.Auth.AuthCookieName = cookieName
			if userID != "" {
				p.Auth.UserID = userID
			}
			// Logging into a profile is a clear "use this" signal — make it the
			// active profile so subsequent commands without --profile target it.
			cfg.ActiveProfile = profile.Name
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			fmt.Printf("✓ Logged in. Session saved and active profile set to %q\n", profile.Name)
			if userID != "" {
				fmt.Printf("  user_id : %s\n", userID)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&username, "username", "u", "", "Email / username")
	cmd.Flags().StringVar(&password, "password", "", "Password (omit to be prompted securely; or set BIFU_PASSWORD for CI — avoid passing on the command line)")
	cmd.Flags().BoolVar(&device, "device", false, "Scan-to-login (like gh auth login): print a QR, scan with the Bifu app, poll for the cookie")
	return cmd
}

// ── Device login (gh auth login style, backed by the QR-login endpoints) ─────
//
// Reuses the backend's existing scan-to-login (QR) flow — no dedicated device
// endpoints needed:
//
//   GET  /user/login/qr_code_get    → issue an issueId + an approval URL
//   POST /user/login/qr_code_check  → poll until a logged-in browser approves
//
// The CLI opens the approval URL in the browser; the user (already logged in on
// the web) approves, and the CLI polls until it receives the session cookie.

type qrGetResp struct {
	RetCode string `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		URL     string `json:"url"`
		IssueID string `json:"issueId"`
	} `json:"result"`
}

type qrCheckResp struct {
	RetCode string `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		// pending | processing | refused | expired | success
		IssueStatus string `json:"issueStatus"`
		CookieStr   string `json:"cookieStr"`
		User        struct {
			UserID string `json:"userId"`
		} `json:"user"`
	} `json:"result"`
}

// runDeviceLogin drives browser-approved login end to end via the QR endpoints.
func runDeviceLogin(load LoadFn) error {
	profile, _, err := load()
	if err != nil {
		return err
	}
	baseURL := profile.BaseURL
	if baseURL == "" {
		return noBaseURLErr(profile.Name)
	}
	termType := profile.Auth.TerminalType
	if termType == "" {
		termType = "API"
	}

	// ── Step 1: request an approval issue ─────────────────────────────────────
	issueID, qrURL, err := qrCodeGet(baseURL, termType)
	if err != nil {
		return fmt.Errorf("request login code failed: %w", err)
	}

	// Prefer the profile's web host so the QR points at the right environment
	// (qr_code_get returns a hard-coded prod URL). Require https for the approval
	// URL so the issueID isn't carried over cleartext (BIFU-CLI-202606-012).
	scanURL := qrURL
	if profile.WebURL != "" {
		if !strings.HasPrefix(profile.WebURL, "https://") {
			return fmt.Errorf("web_url must be https:// for device login (got %q) — fix it with `bifu-cli config set --web-url https://...`", profile.WebURL)
		}
		scanURL = strings.TrimRight(profile.WebURL, "/") + "/x/" + issueID
	}

	// ── Step 2: render a QR for the user to scan with the Bifu app ────────────
	fmt.Println("\nScan this QR code with the Bifu app (already logged in) to approve:")
	fmt.Println()
	printApprovalQR(scanURL)
	fmt.Printf("\nSame target as the QR (scanning with the Bifu app is easiest).\n"+
		"Opening it in a browser only works if that browser is already signed in to Bifu:\n  %s\n", scanURL)
	fmt.Println("\nWaiting for approval (expires in 2 min)...")

	// ── Step 3: poll until approved / rejected / expired ──────────────────────
	// Backend expires the issue after 120s (IssueExpiredTime); match it so we
	// stop polling right as it lapses instead of hitting a stale-key error.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("login not approved in time — run `bifu-cli auth login --device` again")
		}
		time.Sleep(devicePollInterval)

		status, cookieName, cookieVal, userID, err := qrCodeCheck(baseURL, issueID, termType)
		if err != nil {
			return fmt.Errorf("login status check failed: %w", err)
		}
		switch status {
		case "success":
			cfg, err := clifconfig.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			p := cfg.EnsureProfile(profile.Name)
			p.Auth.AuthCookie = cookieVal
			p.Auth.AuthCookieName = cookieName
			if userID != "" {
				p.Auth.UserID = userID
			}
			// Make the just-authenticated profile active (see email path above).
			cfg.ActiveProfile = profile.Name
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Printf("✓ Authentication complete. Session saved and active profile set to %q\n", profile.Name)
			if userID != "" {
				fmt.Printf("  user_id : %s\n", userID)
			}
			return nil
		case "refused":
			return fmt.Errorf("login was rejected in the browser")
		case "expired":
			return fmt.Errorf("login code expired — run `bifu-cli auth login --device` again")
		default:
			// pending / processing — keep waiting
		}
	}
}

// printApprovalQR renders the approval QR, adapting to where stdout goes:
//
//   - A real terminal (TTY) gets the compact half-block form (▀▄), which is
//     crisp because each text row is drawn at the exact character-cell height.
//   - When stdout is captured/piped — e.g. shown back through an AI tool or
//     redirected to a file — half-blocks distort (line-spacing pulls the
//     stacked half-modules apart) and the full-block ANSI form collapses if
//     colors are stripped. So use glyph full-blocks (██ / spaces): no ANSI, one
//     module per cell, which survives reflow far better. The printed link is the
//     reliable fallback either way.
func printApprovalQR(url string) {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		qrterminal.GenerateHalfBlock(url, qrterminal.L, os.Stdout)
		return
	}
	qrterminal.GenerateWithConfig(url, qrterminal.Config{
		Level:     qrterminal.M, // more error correction → more robust after reflow
		Writer:    os.Stdout,
		BlackChar: "██",
		WhiteChar: "  ",
		QuietZone: 2,
	})
}

// qrCodeGet requests an approval issue and returns its id plus the approval URL.
func qrCodeGet(baseURL, terminalType string) (issueID, url string, err error) {
	req, err := http.NewRequest("GET", baseURL+"/user/login/qr_code_get", nil)
	if err != nil {
		return "", "", err
	}
	setLoginHeaders(req, terminalType)

	c := client.NewSecureHTTPClient(30 * time.Second)
	resp, err := c.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var out qrGetResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decode response: %w", err)
	}
	if out.RetCode != "0" {
		return "", "", fmt.Errorf("[%s] %s", out.RetCode, out.RetMsg)
	}
	if out.Result.IssueID == "" {
		return "", "", fmt.Errorf("backend did not return an issue id")
	}
	return out.Result.IssueID, out.Result.URL, nil
}

// qrCodeCheck makes one poll. status is one of:
// pending | processing | refused | expired | success.
func qrCodeCheck(baseURL, issueID, terminalType string) (status, cookieName, cookieVal, userID string, err error) {
	body, _ := json.Marshal(map[string]string{"issueId": issueID})
	req, err := http.NewRequest("POST", baseURL+"/user/login/qr_code_check", bytes.NewReader(body))
	if err != nil {
		return "", "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	setLoginHeaders(req, terminalType)

	c := client.NewSecureHTTPClient(30 * time.Second)
	resp, err := c.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()

	var out qrCheckResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", "", "", fmt.Errorf("decode response: %w", err)
	}
	if out.RetCode != "0" {
		return "", "", "", "", fmt.Errorf("[%s] %s", out.RetCode, out.RetMsg)
	}
	if out.Result.IssueStatus == "success" {
		name, val := extractCookie(out.Result.CookieStr)
		return "success", name, val, out.Result.User.UserID, nil
	}
	return out.Result.IssueStatus, "", "", "", nil
}

func setLoginHeaders(req *http.Request, terminalType string) {
	req.Header.Set("terminalType", terminalType)
	req.Header.Set("locale", "en")
	req.Header.Set("appVersion", "1.0.0")
}

// extractCookie pulls the cookie name and value from a backend cookieStr, which
// is a JSON-serialised http.Cookie ({"Name":"...","Value":"..."}). The name is
// environment-specific (dev=user_auth_name, staging/prod differ), so it must be
// preserved and sent under that exact name. Falls back to treating the whole
// string as the value (under the dev/local name) when it is not JSON.
func extractCookie(cookieStr string) (name, value string) {
	if cookieStr == "" {
		return "", ""
	}
	var ck struct {
		Name  string `json:"Name"`
		Value string `json:"Value"`
	}
	if err := json.Unmarshal([]byte(cookieStr), &ck); err == nil && ck.Value != "" {
		// Don't blindly trust the server-supplied cookie name
		// (BIFU-CLI-202606-026): a malformed/hostile name could later be
		// interpolated into a Cookie header. Accept only a safe token charset;
		// otherwise fall back to the known default name.
		if isValidCookieName(ck.Name) {
			return ck.Name, ck.Value
		}
		return "user_auth_name", ck.Value
	}
	return "user_auth_name", cookieStr
}

// isValidCookieName reports whether name is a safe RFC 6265 cookie token:
// non-empty, bounded length, and free of separators/control characters that
// could break or inject into a Cookie header.
func isValidCookieName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.':
		default:
			return false
		}
	}
	return true
}

// devicePollInterval is how long the CLI waits between approval polls. It is a
// variable so tests can shorten it.
var devicePollInterval = 3 * time.Second

// ── API helpers ───────────────────────────────────────────────────────────────

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResp struct {
	RetCode string `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		CookieStr string `json:"cookieStr"`
		User      struct {
			UserID string `json:"userId"`
		} `json:"user"`
		DoubleCheck struct {
			IssueID string `json:"issueId"`
		} `json:"doubleCheck"`
	} `json:"result"`
}

func doLogin(baseURL, username, password, terminalType string) (issueID string, err error) {
	body, _ := json.Marshal(loginReq{Username: username, Password: password}) // #nosec G117 -- login request body sent to the auth API over TLS; not persisted/logged
	req, err := http.NewRequest("POST", baseURL+"/user/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("terminalType", terminalType)
	req.Header.Set("locale", "en")
	req.Header.Set("appVersion", "1.0.0")

	c := client.NewSecureHTTPClient(30 * time.Second)
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out loginResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if out.RetCode != "0" {
		return "", fmt.Errorf("[%s] %s", out.RetCode, out.RetMsg)
	}
	// Direct cookie (no 2FA required)
	if out.Result.CookieStr != "" {
		return "COOKIE:" + out.Result.CookieStr, nil
	}
	if out.Result.DoubleCheck.IssueID == "" {
		return "", fmt.Errorf("no issueId returned — check credentials")
	}
	return out.Result.DoubleCheck.IssueID, nil
}

type loginCheckReq struct {
	IssueID string `json:"issueId"`
	Code    string `json:"code"`
}

type loginCheckResp struct {
	RetCode string `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		CookieStr string `json:"cookieStr"`
		User      struct {
			UID   string `json:"userId"`
			Email string `json:"email"`
		} `json:"user"`
	} `json:"result"`
}

func doLoginCheck(baseURL, issueID, code, terminalType string) (cookieName, cookieVal, userID string, err error) {
	body, _ := json.Marshal(loginCheckReq{
		IssueID: issueID,
		Code:    code,
	})
	req, err := http.NewRequest("POST", baseURL+"/user/login_check", bytes.NewReader(body))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("terminalType", terminalType)
	req.Header.Set("locale", "en")
	req.Header.Set("appVersion", "1.0.0")

	c := client.NewSecureHTTPClient(30 * time.Second)
	resp, err := c.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	var out loginCheckResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", "", fmt.Errorf("decode response: %w", err)
	}
	if out.RetCode != "0" {
		return "", "", "", fmt.Errorf("[%s] %s", out.RetCode, out.RetMsg)
	}

	uid := out.Result.User.UID

	// cookieStr is a JSON-serialised http.Cookie — preserve name + value.
	if out.Result.CookieStr != "" {
		name, val := extractCookie(out.Result.CookieStr)
		return name, val, uid, nil
	}
	// Fallback: take the session cookie from the Set-Cookie header.
	for _, ck := range resp.Cookies() {
		if ck.Value != "" {
			return ck.Name, ck.Value, uid, nil
		}
	}
	return "", "", uid, fmt.Errorf("no cookie returned in response")
}
