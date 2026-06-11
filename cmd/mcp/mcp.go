// Package mcp implements the `bifu-cli mcp` command group: run an MCP server
// exposing bifu trading tools, and register it with MCP clients.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"bifu-cli/internal/client"
	"bifu-cli/internal/clifconfig"
	mcpserver "bifu-cli/internal/mcp"
	"bifu-cli/internal/output"
)

const version = "1.0.0"

// LoadFn resolves the active profile and printer.
type LoadFn func() (*clifconfig.Profile, *output.Printer, error)

// NewMCPCmd builds the `mcp` command tree.
func NewMCPCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Model Context Protocol server (let AI agents trade via bifu-cli)",
		Long: `Expose bifu-cli's trading tools over the Model Context Protocol so AI agents
(Claude Desktop, Cursor, VS Code, …) can read balances/positions/orders and
place or cancel orders using the active profile.

  bifu-cli mcp serve                  # run the stdio MCP server
  bifu-cli mcp setup --client cursor  # register the server with a client`,
	}
	cmd.AddCommand(newServeCmd(load))
	cmd.AddCommand(newSetupCmd())
	return cmd
}

func newServeCmd(load LoadFn) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the MCP server over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, _, err := load()
			if err != nil {
				return err
			}
			// stdio is the MCP transport — keep it clean (no spinner on stderr).
			client.ShowSpinner = false
			return mcpserver.Serve(profile, version)
		},
	}
}

func newSetupCmd() *cobra.Command {
	var clientName string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Register the bifu MCP server with an MCP client",
		Example: `  bifu-cli mcp setup --client cursor
  bifu-cli --profile dev mcp setup --client claude`,
		RunE: func(cmd *cobra.Command, args []string) error {
			pr := output.NewPrinter(output.FormatTable, false)

			exe, err := os.Executable()
			if err != nil {
				exe = "bifu-cli"
			}
			profile, _ := cmd.Root().PersistentFlags().GetString("profile")

			serverArgs := []any{"mcp", "serve"}
			if profile != "" {
				serverArgs = append(serverArgs, "--profile", profile)
			}
			entry := map[string]any{"command": exe, "args": serverArgs}

			path, key, err := clientConfigPath(clientName)
			if err != nil {
				// Unknown client: print the snippet for manual setup.
				snippet, _ := json.MarshalIndent(map[string]any{
					"mcpServers": map[string]any{"bifu": entry},
				}, "", "  ")
				pr.Line("Add this to your MCP client config:\n%s", string(snippet))
				return nil
			}

			if err := mergeMCPConfig(path, key, entry); err != nil {
				return fmt.Errorf("update %s: %w", path, err)
			}
			pr.OK("Registered bifu MCP server in %s", path)
			pr.Line("  Restart %s to pick it up. Server: %s mcp serve", clientName, exe)
			return nil
		},
	}
	cmd.Flags().StringVar(&clientName, "client", "", "MCP client: claude | cursor | vscode (omit to print a snippet)")
	return cmd
}

// clientConfigPath returns the config file path and the JSON key holding MCP
// servers for the named client.
func clientConfigPath(clientName string) (path, serversKey string, err error) {
	home, _ := os.UserHomeDir()
	switch clientName {
	case "claude":
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), "mcpServers", nil
		}
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), "mcpServers", nil
	case "cursor":
		return filepath.Join(home, ".cursor", "mcp.json"), "mcpServers", nil
	case "vscode":
		return filepath.Join(home, ".config", "Code", "User", "mcp.json"), "servers", nil
	default:
		return "", "", fmt.Errorf("unknown client %q", clientName)
	}
}

// mergeMCPConfig reads the client's JSON config (if any), adds/replaces the
// "bifu" server under serversKey, and writes it back, preserving other entries.
func mergeMCPConfig(path, serversKey string, entry map[string]any) error {
	cfg := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse existing config: %w", err)
		}
	}
	servers, ok := cfg[serversKey].(map[string]any)
	if !ok || servers == nil {
		servers = map[string]any{}
	}
	servers["bifu"] = entry
	cfg[serversKey] = servers

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
