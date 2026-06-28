package auth

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"bifu-cli/internal/clifconfig"
)

// newLogoutCmd builds the `auth logout` subcommand: invalidate the session
// server-side (best effort) and clear it from the resolved profile.
func newLogoutCmd(load LoadFn) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out: invalidate the session and clear it from the profile",
		Example: `  bifu-cli auth logout
  bifu-cli --profile prod auth logout`,
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, pr, err := load()
			if err != nil {
				return err
			}
			if profile.Auth.AuthCookie == "" {
				pr.Line("Already logged out (no session on profile %q).", profile.Name)
				return nil
			}

			// Best-effort server-side logout; clear locally regardless of result.
			if profile.BaseURL != "" {
				if err := doLogout(profile); err != nil {
					pr.Line("%s", "warning: server logout failed ("+err.Error()+"); clearing local session anyway")
				}
			}

			cfg, err := clifconfig.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			p := cfg.EnsureProfile(profile.Name)
			p.Auth.AuthCookie = ""
			p.Auth.AuthCookieName = ""
			p.Auth.UserID = ""
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			pr.OK("Logged out of profile %q", profile.Name)
			return nil
		},
	}
}

// doLogout calls POST /user/logout with the profile's session cookie.
func doLogout(profile *clifconfig.Profile) error {
	termType := profile.Auth.TerminalType
	if termType == "" {
		termType = "API"
	}
	req, err := http.NewRequest("POST", profile.BaseURL+"/user/logout", bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setLoginHeaders(req, termType)
	req.Header.Set("Cookie", profile.Auth.AuthCookieName+"="+profile.Auth.AuthCookie)

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
