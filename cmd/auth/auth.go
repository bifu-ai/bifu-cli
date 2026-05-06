// Package auth implements the `bifu-cli auth` command group.
package auth

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/cookie"
	"bifu-cli/internal/output"
)

// LoadFn resolves the active profile and printer.
type LoadFn func() (*clifconfig.Profile, *output.Printer, error)

// NewAuthCmd builds the `auth` command tree.
func NewAuthCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication utilities (login, cookie encode/decode/set)",
	}
	cmd.AddCommand(newLoginCmd(load))
	cmd.AddCommand(newCookieCmd(load))
	return cmd
}

// ── auth cookie ───────────────────────────────────────────────────────────────

func newCookieCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cookie",
		Short: "Cookie encode / decode / set for user_auth_name",
	}
	cmd.AddCommand(newCookieEncodeCmd())
	cmd.AddCommand(newCookieDecodeCmd())
	cmd.AddCommand(newCookieSetCmd(load))
	return cmd
}

// auth cookie encode ──────────────────────────────────────────────────────────

func newCookieEncodeCmd() *cobra.Command {
	var env string
	cmd := &cobra.Command{
		Use:   "encode <uid>",
		Short: "Generate a user_auth_name cookie for a given UID",
		Long: `Generate and print the user_auth_name cookie value for the specified UID.

The cookie uses AES-CBC encryption (same key as the bifu backend).
Use ` + "`" + `auth cookie set` + "`" + ` to generate AND save to your active profile.`,
		Example: `  bifu-cli auth cookie encode 109150807
  bifu-cli auth cookie encode 109150807 --env dev`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uid, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid uid %q: %w", args[0], err)
			}
			val := cookie.Generate(uid, env)
			fmt.Println(val)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "dev",
		"Cookie environment tag: dev | staging | prod")
	return cmd
}

// auth cookie decode ──────────────────────────────────────────────────────────

func newCookieDecodeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "decode <cookie-value>",
		Short: "Decode a user_auth_name cookie and show uid / env",
		Example: `  bifu-cli auth cookie decode "yHjCFUQ2jF..."`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uid, env, raw, err := cookie.Decode(args[0])
			if err != nil {
				return fmt.Errorf("decode: %w", err)
			}
			fmt.Printf("uid : %d\n", uid)
			fmt.Printf("env : %s\n", env)
			fmt.Printf("raw : %s\n", raw)
			return nil
		},
	}
}

// auth cookie set ─────────────────────────────────────────────────────────────

func newCookieSetCmd(load LoadFn) *cobra.Command {
	var env string
	cmd := &cobra.Command{
		Use:   "set <uid>",
		Short: "Generate a cookie and save it to the active profile",
		Long: `Generate a user_auth_name cookie for the given UID and persist it in the
active profile's auth_cookie field.

The --env flag defaults to the profile name (dev/staging/prod map directly,
anything else defaults to "dev").`,
		Example: `  bifu-cli auth cookie set 109150807
  bifu-cli auth cookie set 109150807 --env dev
  bifu-cli --profile dev auth cookie set 109150807`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uid, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid uid %q: %w", args[0], err)
			}

			// Load config
			cfg, err := clifconfig.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			profile, _, err := load()
			if err != nil {
				return fmt.Errorf("load profile: %w", err)
			}

			// Infer env from profile name if not explicitly set
			if !cmd.Flags().Changed("env") {
				env = cookie.EnvFromProfileName(strings.ToLower(profile.Name))
			}

			val := cookie.Generate(uid, env)

			// Update auth_cookie in profile and save
			p := cfg.Active()
			p.Auth.AuthCookie = val
			p.Auth.UserID = strconv.FormatInt(uid, 10)
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			fmt.Printf("✓ Cookie saved to profile %q\n", cfg.ActiveProfile)
			fmt.Printf("  uid : %d\n", uid)
			fmt.Printf("  env : %s\n", env)
			fmt.Printf("  cookie: %s\n", val)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "dev",
		"Cookie environment tag: dev | staging | prod")
	return cmd
}
