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

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"bifu-cli/internal/clifconfig"
)

// newLoginCmd builds the `auth login` subcommand.
func newLoginCmd(load LoadFn) *cobra.Command {
	var username, password string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login with email/password and save session cookie",
		Long: `Login to BifuFX using your email and password.

An email verification code will be sent and you will be prompted to enter it.
On success, the session cookie is automatically saved to the active profile.`,
		Example: `  bifu-cli auth login
  bifu-cli auth login --username user@example.com
  bifu-cli --profile dev auth login`,
		RunE: func(cmd *cobra.Command, args []string) error {
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
			fmt.Printf("Logging in as %s...\n", username)
			issueID, err := doLogin(baseURL, username, password)
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			var cookieVal, userID string

			// If issueID has COOKIE: prefix, no 2FA needed — cookie is inline
			if strings.HasPrefix(issueID, "COOKIE:") {
				cookieVal = strings.TrimPrefix(issueID, "COOKIE:")
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
				cookieVal, userID, err = doLoginCheck(baseURL, issueID, code)
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
	return cmd
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

func doLogin(baseURL, username, password string) (issueID string, err error) {
	body, _ := json.Marshal(loginReq{Username: username, Password: password})
	req, err := http.NewRequest("POST", baseURL+"/user/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("terminalType", "web")
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
			UID   int64  `json:"userId"`
			Email string `json:"email"`
		} `json:"user"`
	} `json:"result"`
}

func doLoginCheck(baseURL, issueID, code string) (cookieVal, userID string, err error) {
	body, _ := json.Marshal(loginCheckReq{
		IssueID: issueID,
		Code:    code,
	})
	req, err := http.NewRequest("POST", baseURL+"/user/login_check", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("terminalType", "web")
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

	// Cookie is in response body cookieStr field
	if out.Result.CookieStr != "" {
		return out.Result.CookieStr, fmt.Sprintf("%d", out.Result.User.UID), nil
	}
	// Fallback: check Set-Cookie header
	for _, ck := range resp.Cookies() {
		if ck.Name == "user_auth_name" {
			return ck.Value, fmt.Sprintf("%d", out.Result.User.UID), nil
		}
	}
	return "", fmt.Sprintf("%d", out.Result.User.UID), fmt.Errorf("no cookie returned in response")
}
