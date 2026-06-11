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
	var web bool
	var loginURL string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login and save session cookie (email/password, or --web)",
		Long: `Login to BifuFX and save the session cookie to the active profile.

Default: email/password — a verification code is sent to your email and you
are prompted to enter it.

--web: open the web login page in your browser, then paste back the
user_auth_name cookie. The page URL comes from --url, else the profile's
web_url (set it once with: bifu-cli config set --web-url https://<webapp>).`,
		Example: `  bifu-cli auth login
  bifu-cli auth login --username user@example.com
  bifu-cli --profile dev auth login
  bifu-cli auth login --web
  bifu-cli auth login --web --url https://app.bifu.dev/login`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if web {
				return runWebLogin(load, loginURL)
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
	cmd.Flags().BoolVar(&web, "web", false, "Login via browser: open the web login page and capture the session cookie")
	cmd.Flags().StringVar(&loginURL, "url", "", "Web login page URL (used with --web; defaults to profile web_url)")
	return cmd
}

// runWebLogin opens the web login page in the browser and captures the
// user_auth_name cookie pasted back by the user. Works against the existing
// backend without any server-side changes.
func runWebLogin(load LoadFn, explicitURL string) error {
	profile, _, err := load()
	if err != nil {
		return err
	}

	url := explicitURL
	if url == "" {
		url = profile.WebURL
	}
	if url == "" {
		return fmt.Errorf("no web login URL — pass --url <web-login-page>, or set it once:\n" +
			"  bifu-cli config set --web-url https://<webapp>")
	}

	fmt.Printf("Opening %s in your browser...\n", url)
	if err := openBrowser(url); err != nil {
		fmt.Printf("⚠ could not open the browser automatically (%v)\n  Open this URL manually:\n  %s\n", err, url)
	}

	fmt.Println()
	fmt.Println("After logging in, copy the session cookie value:")
	fmt.Println("  DevTools → Application → Cookies → user_auth_name → Value")
	fmt.Print("\nPaste user_auth_name cookie: ")

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	cookieVal := normalizeCookie(line)
	if cookieVal == "" {
		return fmt.Errorf("no cookie provided")
	}

	cfg, err := clifconfig.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	p := cfg.EnsureProfile(profile.Name)
	p.Auth.AuthCookie = cookieVal
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("✓ Cookie saved to profile %q\n", profile.Name)
	fmt.Printf("  cookie : %s\n", cookieVal)
	return nil
}

// normalizeCookie extracts the bare user_auth_name value from whatever the user
// pasted: a raw value, a `user_auth_name=...` pair, or a full Cookie header.
func normalizeCookie(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	if i := strings.Index(s, "user_auth_name="); i >= 0 {
		s = s[i+len("user_auth_name="):]
	}
	// A pasted Cookie header may contain other cookies after a separator.
	if i := strings.IndexByte(s, ';'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(strings.Trim(s, `"'`))
}

// openBrowser launches the OS default browser at url.
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
