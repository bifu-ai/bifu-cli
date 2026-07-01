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

	"bifu-cli/internal/client"
	"bifu-cli/internal/clifconfig"
)

// newRegisterCmd builds the `auth register` subcommand: create an account with
// email + password, confirm the emailed code, and save the resulting session.
func newRegisterCmd(load LoadFn) *cobra.Command {
	var email, password, referrer string
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new BifuFX account (email + verification code)",
		Long: `Create a new BifuFX account. A verification code is emailed; enter it to
activate. On success the session cookie is saved to the active profile (you are
logged in). On dev/staging the universal code 123456 works.`,
		Example: `  bifu-cli --profile dev auth register --email you@example.com
  echo 123456 | bifu-cli --profile dev auth register --email you@example.com --password 'Pw123!@#'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, _, err := load()
			if err != nil {
				return err
			}
			baseURL := profile.BaseURL
			if baseURL == "" {
				return noBaseURLErr(profile.Name)
			}
			reader := bufio.NewReader(os.Stdin)

			if email == "" {
				fmt.Print("Email: ")
				line, _ := reader.ReadString('\n')
				email = strings.TrimSpace(line)
			}
			if password == "" {
				if term.IsTerminal(int(os.Stdin.Fd())) {
					fmt.Print("Password: ")
					b, err := term.ReadPassword(int(os.Stdin.Fd()))
					fmt.Println()
					if err != nil {
						return fmt.Errorf("read password: %w", err)
					}
					password = string(b)
				} else {
					fmt.Print("Password: ")
					line, _ := reader.ReadString('\n')
					password = strings.TrimSpace(line)
				}
			}
			if email == "" || password == "" {
				return fmt.Errorf("email and password are required")
			}

			termType := profile.Auth.TerminalType
			if termType == "" {
				termType = "API"
			}
			locale := profile.Auth.Locale
			if locale == "" {
				locale = "en"
			}

			// ── Step 1: POST /user/register → issueId + emailed code ──────────
			fmt.Printf("Registering %s...\n", email)
			issueID, err := doRegister(baseURL, email, password, locale, referrer, termType)
			if err != nil {
				return fmt.Errorf("register failed: %w", err)
			}
			fmt.Printf("✓ Verification code sent to %s\n", email)

			// ── Step 2: prompt for the code ───────────────────────────────────
			fmt.Print("Verification code: ")
			codeLine, _ := reader.ReadString('\n')
			code := strings.TrimSpace(codeLine)
			if code == "" {
				return fmt.Errorf("verification code is required")
			}

			// ── Step 3: POST /user/activate → session cookie ──────────────────
			cookieName, cookieVal, err := doActivate(baseURL, issueID, code, termType)
			if err != nil {
				return fmt.Errorf("activation failed: %w", err)
			}

			// ── Step 4: persist session to the resolved profile ───────────────
			cfg, err := clifconfig.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			p := cfg.EnsureProfile(profile.Name)
			p.Auth.AuthCookie = cookieVal
			p.Auth.AuthCookieName = cookieName
			cfg.ActiveProfile = profile.Name
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Printf("✓ Registered. Session saved and active profile set to %q\n", profile.Name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "Account email")
	cmd.Flags().StringVar(&password, "password", "", "Account password (omit to be prompted securely)")
	cmd.Flags().StringVar(&referrer, "referrer", "", "Referral code (optional)")
	return cmd
}

type registerReq struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
	Locale          string `json:"locale"`
	Referrer        string `json:"referrer,omitempty"`
}

type registerResp struct {
	RetCode string `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		IssueID string `json:"issueId"`
		Email   string `json:"email"`
	} `json:"result"`
}

func doRegister(baseURL, email, password, locale, referrer, terminalType string) (issueID string, err error) {
	body, _ := json.Marshal(registerReq{ // #nosec G117 -- registration request body sent to the auth API over TLS; not persisted/logged
		Email: email, Password: password, ConfirmPassword: password,
		Locale: locale, Referrer: referrer,
	})
	req, err := http.NewRequest("POST", baseURL+"/user/register", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	setLoginHeaders(req, terminalType)

	resp, err := client.NewSecureHTTPClient(30 * time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out registerResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if out.RetCode != "0" {
		return "", fmt.Errorf("[%s] %s", out.RetCode, out.RetMsg)
	}
	if out.Result.IssueID == "" {
		return "", fmt.Errorf("no issueId returned")
	}
	return out.Result.IssueID, nil
}

type activateResp struct {
	RetCode string `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		CookieStr string `json:"cookieStr"`
	} `json:"result"`
}

func doActivate(baseURL, issueID, code, terminalType string) (cookieName, cookieVal string, err error) {
	body, _ := json.Marshal(map[string]string{"issueId": issueID, "code": code})
	req, err := http.NewRequest("POST", baseURL+"/user/activate", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	setLoginHeaders(req, terminalType)

	resp, err := client.NewSecureHTTPClient(30 * time.Second).Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var out activateResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decode response: %w", err)
	}
	if out.RetCode != "0" {
		return "", "", fmt.Errorf("[%s] %s", out.RetCode, out.RetMsg)
	}
	if out.Result.CookieStr == "" {
		return "", "", fmt.Errorf("no session cookie returned")
	}
	name, val := extractCookie(out.Result.CookieStr)
	return name, val, nil
}
