// Package mcp implements the `bifu-cli mcp` command group: run an MCP server
// exposing bifu trading tools, and register it with MCP clients.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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
(Claude Code, Codex, Cursor, VS Code, Claude Desktop, …) can read
balances/positions/orders and place or cancel orders using the active profile.

  bifu-cli mcp serve                  # run the stdio MCP server
  bifu-cli mcp setup --client claude  # register with Claude Code (claude mcp add)`,
	}
	cmd.AddCommand(newServeCmd(load))
	cmd.AddCommand(newSetupCmd())
	return cmd
}

func newServeCmd(load LoadFn) *cobra.Command {
	var httpAddr, httpPath string
	var stateless bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the MCP server — stdio (default) or Streamable HTTP (--http)",
		Long: `Run the bifu MCP server. Default transport is stdio (the client launches
this process). Use --http to instead serve the Streamable HTTP transport on a
TCP address, mounting the MCP endpoint at --path (default /mcp).

Every tool call uses the configured profile's logged-in session — the HTTP
transport has no per-request auth, so bind it to localhost unless the network
is trusted.`,
		Example: `  bifu-cli --profile dev mcp serve                        # stdio (default)
  bifu-cli --profile dev mcp serve --http 127.0.0.1:8080  # Streamable HTTP at http://127.0.0.1:8080/mcp
  bifu-cli --profile dev mcp serve --http :8080 --stateless`,
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, _, err := load()
			if err != nil {
				return err
			}
			// Keep stdout clean of the spinner (stdio uses it as the transport).
			client.ShowSpinner = false
			if httpAddr != "" {
				return mcpserver.ServeHTTP(profile, version, httpAddr, httpPath, stateless)
			}
			return mcpserver.Serve(profile, version)
		},
	}
	cmd.Flags().StringVar(&httpAddr, "http", "", "Serve Streamable HTTP on this address (e.g. 127.0.0.1:8080) instead of stdio")
	cmd.Flags().StringVar(&httpPath, "path", "/mcp", "HTTP endpoint path (used with --http)")
	cmd.Flags().BoolVar(&stateless, "stateless", false, "Stateless HTTP mode — no per-session state (used with --http)")
	return cmd
}

func newSetupCmd() *cobra.Command {
	var clientName string
	var profiles string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Register the bifu MCP server with an MCP client",
		Example: `  bifu-cli --profile dev mcp setup --client claude          # Claude Code, single env
  bifu-cli mcp setup --client claude-desktop --profiles dev,staging,prod  # 3 servers bifu-dev/staging/prod
  bifu-cli --profile dev mcp setup --client codex           # OpenAI Codex (codex mcp add)
  bifu-cli mcp setup --client cursor`,
		RunE: func(cmd *cobra.Command, args []string) error {
			pr := output.NewPrinter(output.FormatTable, false)

			exe, err := os.Executable()
			if err != nil {
				exe = "bifu-cli"
			}
			rootProfile, _ := cmd.Root().PersistentFlags().GetString("profile")
			servers := resolveServers(profiles, rootProfile)

			// CLI-managed clients own their own config — register via their
			// official `... mcp add` command (each falls back to a snippet).
			switch clientName {
			case "claude", "claude-code":
				return setupClaudeCode(pr, exe, servers) // Claude Code (~/.claude.json)
			case "codex":
				return setupCodex(pr, exe, servers) // OpenAI Codex (~/.codex/config.toml)
			}

			path, key, err := clientConfigPath(clientName)
			if err != nil {
				// Unknown client: print the snippet for manual setup.
				m := map[string]any{}
				for _, s := range servers {
					m[s.name] = map[string]any{"command": exe, "args": serveArgs(s.profile)}
				}
				snippet, _ := json.MarshalIndent(map[string]any{"mcpServers": m}, "", "  ")
				pr.Line("Add this to your MCP client config:\n%s", string(snippet))
				return nil
			}

			for _, s := range servers {
				entry := map[string]any{"command": exe, "args": serveArgs(s.profile)}
				if err := mergeMCPConfig(path, key, s.name, entry); err != nil {
					return fmt.Errorf("update %s: %w", path, err)
				}
			}
			pr.OK("Registered %d bifu MCP server(s) in %s", len(servers), path)
			pr.Line("  Restart %s to pick them up.", clientName)
			return nil
		},
	}
	cmd.Flags().StringVar(&clientName, "client", "", "MCP client: claude (Claude Code) | codex | cursor | vscode | claude-desktop (omit to print a snippet)")
	cmd.Flags().StringVar(&profiles, "profiles", "", "Comma-separated env profiles → one server each as bifu-<env> (e.g. dev,staging,prod). Default: a single 'bifu' using --profile/active.")
	_ = cmd.RegisterFlagCompletionFunc("client", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"claude", "codex", "cursor", "vscode", "claude-desktop"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

// serverSpec is one MCP server entry to register: a client-visible name and the
// bifu-cli profile it is pinned to ("" = follow the active profile).
type serverSpec struct {
	name    string
	profile string
}

// resolveServers turns the --profiles flag into the set of servers to register.
// With --profiles, one server per env named bifu-<env>; otherwise a single
// "bifu" pinned to --profile (or the active profile when empty).
func resolveServers(profilesCSV, rootProfile string) []serverSpec {
	if strings.TrimSpace(profilesCSV) != "" {
		var out []serverSpec
		for _, p := range strings.Split(profilesCSV, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, serverSpec{name: "bifu-" + p, profile: p})
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []serverSpec{{name: "bifu", profile: rootProfile}}
}

// serveArgs is the `mcp serve` argument list for a profile ("" = active).
func serveArgs(profile string) []string {
	a := []string{"mcp", "serve"}
	if profile != "" {
		a = append(a, "--profile", profile)
	}
	return a
}

// setupClaudeCode registers the bifu MCP server with Claude Code (the `claude`
// CLI / IDE extension) via the official `claude mcp add`, which owns
// ~/.claude.json. NOTE: this is the Claude Code agent, not the Claude Desktop
// app (that's --client claude-desktop). Falls back to a copy-paste command and
// a project .mcp.json snippet when the claude CLI isn't on PATH.
func setupClaudeCode(pr *output.Printer, exe string, servers []serverSpec) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		m := map[string]any{}
		for _, s := range servers {
			pr.Line("Claude Code CLI not found. Once installed, run:\n  claude mcp add %s -- %s %s", s.name, exe, strings.Join(serveArgs(s.profile), " "))
			m[s.name] = map[string]any{"command": exe, "args": serveArgs(s.profile)}
		}
		snippet, _ := json.MarshalIndent(map[string]any{"mcpServers": m}, "", "  ")
		pr.Line("Or add to a project .mcp.json:\n%s", string(snippet))
		return nil
	}

	// Re-register (remove first) so a changed --profile takes effect — `claude
	// mcp add` errors if the name already exists.
	for _, s := range servers {
		_ = exec.Command(claudePath, "mcp", "remove", s.name).Run() // #nosec G204 -- fixed args; s.name is bifu[-env]
		addArgs := append([]string{"mcp", "add", s.name, "--", exe}, serveArgs(s.profile)...)
		out, err := exec.Command(claudePath, addArgs...).CombinedOutput() // #nosec G204 -- exe is this binary's own path; args fixed
		if err != nil {
			return fmt.Errorf("claude mcp add %s failed: %w\n%s", s.name, err, strings.TrimSpace(string(out)))
		}
	}
	pr.OK("Registered %d bifu MCP server(s) with Claude Code", len(servers))
	pr.Line("  Verify with: claude mcp list  (or /mcp inside Claude Code). Add -s user to each for all projects.")
	return nil
}

// setupCodex registers the bifu MCP server with the OpenAI Codex CLI. It prefers
// the official `codex mcp add` (idempotent, owns ~/.codex/config.toml); if the
// codex binary is absent it prints the TOML snippet to add manually.
func setupCodex(pr *output.Printer, exe string, servers []serverSpec) error {
	if codexPath := lookCodex(); codexPath != "" {
		// Re-register so a changed --profile takes effect (add errors if it exists).
		for _, s := range servers {
			_ = exec.Command(codexPath, "mcp", "remove", s.name).Run() // #nosec G204 -- fixed args; s.name is bifu[-env]
			addArgs := append([]string{"mcp", "add", s.name, "--", exe}, serveArgs(s.profile)...)
			out, err := exec.Command(codexPath, addArgs...).CombinedOutput() // #nosec G204 -- exe is this binary's own path; args fixed
			if err != nil {
				return fmt.Errorf("codex mcp add %s failed: %w\n%s", s.name, err, strings.TrimSpace(string(out)))
			}
		}
		pr.OK("Registered %d bifu MCP server(s) with Codex", len(servers))
		pr.Line("  Verify with: codex mcp list  (or /mcp in the Codex TUI)")
		return nil
	}

	// codex not installed → print the TOML block(s) for ~/.codex/config.toml.
	var b strings.Builder
	for _, s := range servers {
		args := serveArgs(s.profile)
		quoted := make([]string, len(args))
		for i, a := range args {
			quoted[i] = fmt.Sprintf("%q", a)
		}
		fmt.Fprintf(&b, "[mcp_servers.%s]\ncommand = %q\nargs = [%s]\n\n", s.name, exe, strings.Join(quoted, ", "))
	}
	pr.Line("Codex CLI not found on PATH. Add this to ~/.codex/config.toml:\n\n%s", b.String())
	return nil
}

// lookCodex finds the codex CLI: first on PATH, then inside the desktop Codex
// app's install location (the app ships the same binary but may not add it to
// PATH). Linux desktop builds (AppImage) have no fixed path — rely on PATH.
func lookCodex() string {
	if p, err := exec.LookPath("codex"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Codex.app/Contents/Resources/codex",
			filepath.Join(home, "Applications", "Codex.app", "Contents", "Resources", "codex"),
		}
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			local = filepath.Join(home, "AppData", "Local")
		}
		candidates = []string{
			filepath.Join(local, "Programs", "Codex", "codex.exe"),
			filepath.Join(local, "Programs", "codex", "codex.exe"),
		}
	}
	for _, p := range candidates {
		// #nosec G703 -- candidates are fixed app-install locations; Stat only.
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// userConfigBase returns the per-user app-config base dir for the current OS:
// macOS ~/Library/Application Support, Windows %APPDATA%, else $XDG_CONFIG_HOME
// or ~/.config. Used to locate GUI clients' config across platforms.
func userConfigBase() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support")
	case "windows":
		if ad := os.Getenv("APPDATA"); ad != "" {
			return ad
		}
		return filepath.Join(home, "AppData", "Roaming")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return xdg
		}
		return filepath.Join(home, ".config")
	}
}

// clientConfigPath returns the config file path and the JSON key holding MCP
// servers for the named client (cross-platform: macOS / Windows / Linux).
func clientConfigPath(clientName string) (path, serversKey string, err error) {
	home, _ := os.UserHomeDir()
	switch clientName {
	case "claude-desktop":
		return filepath.Join(userConfigBase(), "Claude", "claude_desktop_config.json"), "mcpServers", nil
	case "cursor":
		return filepath.Join(home, ".cursor", "mcp.json"), "mcpServers", nil
	case "vscode":
		return filepath.Join(userConfigBase(), "Code", "User", "mcp.json"), "servers", nil
	default:
		return "", "", fmt.Errorf("unknown client %q", clientName)
	}
}

// mergeMCPConfig reads the client's JSON config (if any), adds/replaces the
// named server under serversKey, and writes it back, preserving other entries.
func mergeMCPConfig(path, serversKey, name string, entry map[string]any) error {
	cfg := map[string]any{}
	// #nosec G304 -- path is a known MCP-client config location, not untrusted input
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse existing config: %w", err)
		}
	}
	servers, ok := cfg[serversKey].(map[string]any)
	if !ok || servers == nil {
		servers = map[string]any{}
	}
	servers[name] = entry
	cfg[serversKey] = servers

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // #nosec G301 -- MCP-client config dir, not secret
		return err
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644) // #nosec G306 -- MCP-client config (no secrets), readable by the client app
}
