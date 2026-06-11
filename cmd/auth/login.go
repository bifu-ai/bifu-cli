package auth

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"bifu-cli/internal/clifconfig"
)

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

--device: OAuth-style device login (like ` + "`gh auth login`" + `). The CLI shows a
one-time code, opens the browser to the verification page, and polls until you
approve. No password is typed in the terminal. Requires backend device
endpoints (/user/device_code and /user/device_token).`,
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
				return fmt.Errorf("no base_url configured for this profile (run: bifu-cli config init --env dev)")
			}

			// ── Step 0: collect credentials ──────────────────────────────────
			reader := bufio.NewReader(os.Stdin)

			if username == "" {
				fmt.Print("Username (email): ")
				line, _ := reader.ReadString('\n')
				username = strings.TrimSpace(line)
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

			var cookieVal, userID string

			// If issueID has COOKIE: prefix, no 2FA needed — cookie is inline (JSON-encoded)
			if strings.HasPrefix(issueID, "COOKIE:") {
				raw := strings.TrimPrefix(issueID, "COOKIE:")
				var ck struct {
					Value string `json:"Value"`
				}
				if err := json.Unmarshal([]byte(raw), &ck); err == nil && ck.Value != "" {
					cookieVal = ck.Value
				} else {
					cookieVal = raw
				}
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
				cookieVal, userID, err = doLoginCheck(baseURL, issueID, code, termType)
				if err != nil {
					return fmt.Errorf("verification failed: %w", err)
				}
			}

			// ── Step 4: persist cookie to active profile ──────────────────────
			cfg, err := clifconfig.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			p := cfg.Active()
			p.Auth.AuthCookie = cookieVal
			if userID != "" {
				p.Auth.UserID = userID
			}
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			fmt.Printf("✓ Cookie saved to profile %q\n", cfg.ActiveProfile)
			if userID != "" {
				fmt.Printf("  user_id : %s\n", userID)
			}
			fmt.Printf("  cookie  : %s\n", cookieVal)
			return nil
		},
	}

	cmd.Flags().StringVarP(&username, "username", "u", "", "Email / username")
	cmd.Flags().StringVar(&password, "password", "", "Password (omit to be prompted securely)")
	cmd.Flags().BoolVar(&device, "device", false, "OAuth-style device login (like gh auth login): show a code, open the browser, poll for approval")
	return cmd
}

// ── Device flow (gh auth login style) ───────────────────────────────────────
//
// Implements the OAuth 2.0 Device Authorization Grant (RFC 8628) against the
// bifu backend. Requires two server endpoints:
//
//   POST /user/device_code   → issue a (deviceCode, userCode) pair
//   POST /user/device_token  → poll until the user approves in the browser
//
// See docs/device-flow.md for the full request/response contract.

type deviceCodeResp struct {
	RetCode string `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		DeviceCode              string `json:"deviceCode"`
		UserCode                string `json:"userCode"`
		VerificationURI         string `json:"verificationUri"`
		VerificationURIComplete string `json:"verificationUriComplete"`
		ExpiresIn               int    `json:"expiresIn"` // seconds
		Interval                int    `json:"interval"`  // poll seconds
	} `json:"result"`
}

type deviceTokenResp struct {
	RetCode string `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		// status: pending | slow_down | success | denied | expired
		Status    string `json:"status"`
		CookieStr string `json:"cookieStr"`
		User      struct {
			UserID string `json:"userId"`
		} `json:"user"`
	} `json:"result"`
}

// runDeviceLogin drives the device authorization grant end to end.
func runDeviceLogin(load LoadFn) error {
	profile, _, err := load()
	if err != nil {
		return err
	}
	baseURL := profile.BaseURL
	if baseURL == "" {
		return fmt.Errorf("no base_url configured for this profile (run: bifu-cli config init --env dev)")
	}
	termType := profile.Auth.TerminalType
	if termType == "" {
		termType = "API"
	}

	// ── Step 1: request a device + user code ──────────────────────────────────
	dc, err := requestDeviceCode(baseURL, termType)
	if err != nil {
		return fmt.Errorf("device authorization request failed: %w", err)
	}

	verifyURL := dc.Result.VerificationURIComplete
	if verifyURL == "" {
		verifyURL = dc.Result.VerificationURI
	}

	// ── Step 2: show the code and open the browser ────────────────────────────
	fmt.Printf("\n! First copy your one-time code: %s\n\n", dc.Result.UserCode)
	fmt.Printf("Opening %s in your browser to authorize...\n", verifyURL)
	if err := openBrowser(verifyURL); err != nil {
		fmt.Printf("⚠ could not open the browser automatically (%v)\n  Open this URL manually:\n  %s\n", err, verifyURL)
	}

	// ── Step 3: poll until approved / denied / expired ────────────────────────
	interval := dc.Result.Interval
	if interval <= 0 {
		interval = 5
	}
	expiresIn := dc.Result.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 600
	}
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)

	fmt.Println("\nWaiting for authorization...")
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("device code expired before authorization — run `bifu-cli auth login --device` again")
		}
		time.Sleep(time.Duration(interval) * time.Second)

		cookieVal, userID, status, err := pollDeviceToken(baseURL, dc.Result.DeviceCode, termType)
		if err != nil {
			return fmt.Errorf("device token poll failed: %w", err)
		}
		switch status {
		case "success":
			cfg, err := clifconfig.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			p := cfg.EnsureProfile(profile.Name)
			p.Auth.AuthCookie = cookieVal
			if userID != "" {
				p.Auth.UserID = userID
			}
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Printf("✓ Authentication complete. Cookie saved to profile %q\n", profile.Name)
			if userID != "" {
				fmt.Printf("  user_id : %s\n", userID)
			}
			return nil
		case "slow_down":
			interval += 5 // RFC 8628: back off on slow_down
		case "pending", "":
			// keep polling
		case "denied":
			return fmt.Errorf("authorization was denied in the browser")
		case "expired":
			return fmt.Errorf("device code expired — run `bifu-cli auth login --device` again")
		default:
			return fmt.Errorf("unexpected authorization status %q", status)
		}
	}
}

func requestDeviceCode(baseURL, terminalType string) (*deviceCodeResp, error) {
	body, _ := json.Marshal(map[string]string{"terminalType": terminalType})
	req, err := http.NewRequest("POST", baseURL+"/user/device_code", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("terminalType", terminalType)
	req.Header.Set("locale", "en")
	req.Header.Set("appVersion", "1.0.0")

	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out deviceCodeResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if out.RetCode != "0" {
		return nil, fmt.Errorf("[%s] %s", out.RetCode, out.RetMsg)
	}
	if out.Result.DeviceCode == "" || out.Result.UserCode == "" {
		return nil, fmt.Errorf("backend did not return a device/user code")
	}
	return &out, nil
}

// pollDeviceToken makes one poll request. status is one of:
// success | pending | slow_down | denied | expired.
func pollDeviceToken(baseURL, deviceCode, terminalType string) (cookieVal, userID, status string, err error) {
	body, _ := json.Marshal(map[string]string{"deviceCode": deviceCode})
	req, err := http.NewRequest("POST", baseURL+"/user/device_token", bytes.NewReader(body))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("terminalType", terminalType)
	req.Header.Set("locale", "en")
	req.Header.Set("appVersion", "1.0.0")

	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	var out deviceTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", "", fmt.Errorf("decode response: %w", err)
	}
	if out.RetCode != "0" {
		return "", "", "", fmt.Errorf("[%s] %s", out.RetCode, out.RetMsg)
	}
	if out.Result.Status == "success" {
		return extractCookieValue(out.Result.CookieStr), out.Result.User.UserID, "success", nil
	}
	return "", "", out.Result.Status, nil
}

// extractCookieValue pulls the bare cookie value from a backend cookieStr, which
// is a JSON-serialised http.Cookie ({"Value":"..."}). Falls back to the raw
// string when it is not JSON.
func extractCookieValue(cookieStr string) string {
	if cookieStr == "" {
		return ""
	}
	var ck struct {
		Value string `json:"Value"`
	}
	if err := json.Unmarshal([]byte(cookieStr), &ck); err == nil && ck.Value != "" {
		return ck.Value
	}
	return cookieStr
}

// openBrowser launches the OS default browser at url. It is a variable so tests
// can stub it out instead of spawning a real browser.
var openBrowser = func(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

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
	body, _ := json.Marshal(loginReq{Username: username, Password: password})
	req, err := http.NewRequest("POST", baseURL+"/user/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("terminalType", terminalType)
	req.Header.Set("locale", "en")
	req.Header.Set("appVersion", "1.0.0")

	c := &http.Client{Timeout: 30 * time.Second}
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

func doLoginCheck(baseURL, issueID, code, terminalType string) (cookieVal, userID string, err error) {
	body, _ := json.Marshal(loginCheckReq{
		IssueID: issueID,
		Code:    code,
	})
	req, err := http.NewRequest("POST", baseURL+"/user/login_check", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("terminalType", terminalType)
	req.Header.Set("locale", "en")
	req.Header.Set("appVersion", "1.0.0")

	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var out loginCheckResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decode response: %w", err)
	}
	if out.RetCode != "0" {
		return "", "", fmt.Errorf("[%s] %s", out.RetCode, out.RetMsg)
	}

	uid := out.Result.User.UID

	// cookieStr is a JSON-serialised http.Cookie — extract Value
	if out.Result.CookieStr != "" {
		var ck struct {
			Value string `json:"Value"`
		}
		if err := json.Unmarshal([]byte(out.Result.CookieStr), &ck); err == nil && ck.Value != "" {
			return ck.Value, uid, nil
		}
		// If not JSON or Value empty, use raw string as-is
		return out.Result.CookieStr, uid, nil
	}
	// Fallback: check Set-Cookie header
	for _, ck := range resp.Cookies() {
		if ck.Name == "user_auth_name" {
			return ck.Value, uid, nil
		}
	}
	return "", uid, fmt.Errorf("no cookie returned in response")
}
