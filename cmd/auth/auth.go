// Package auth implements the `bifu-cli auth` command group.
package auth

import (
	"github.com/spf13/cobra"

	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
)

// LoadFn resolves the active profile and printer.
type LoadFn func() (*clifconfig.Profile, *output.Printer, error)

// NewAuthCmd builds the `auth` command tree.
func NewAuthCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication (email/password or --device scan-to-login)",
	}
	cmd.AddCommand(newLoginCmd(load))
	return cmd
}
