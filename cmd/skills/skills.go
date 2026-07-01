// Package skillscmd implements the `bifu-cli skills` command group: list, show,
// and install the embedded agent SKILL.md guides (à la OKX agent-trade-kit).
package skillscmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"bifu-cli/internal/output"
	skillsdata "bifu-cli/skills"
)

// NewSkillsCmd builds the `skills` command tree.
func NewSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Agent skills — SKILL.md guides that teach AI agents how to use bifu-cli",
		Long: `Agent skills are SKILL.md instruction files (one per task category) that tell
an AI agent when to activate and how to drive bifu-cli. Point your agent
(Claude Code, Cursor, …) at the installed files, or pipe ` + "`skills show`" + ` into context.`,
	}
	cmd.AddCommand(newListCmd(), newShowCmd(), newInstallCmd())
	return cmd
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available agent skills",
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := skillsdata.List()
			if err != nil {
				return err
			}
			jsonOut, _ := cmd.Root().PersistentFlags().GetBool("json")
			format, _ := cmd.Root().PersistentFlags().GetString("output")
			pr := output.NewPrinter(output.FormatTable, false)
			if jsonOut || format == string(output.FormatJSON) {
				pr.Format = output.FormatJSON
			}
			rows := make([][]string, 0, len(items))
			for _, s := range items {
				rows = append(rows, []string{s.Name, s.Auth, s.Description})
			}
			pr.PrintTable([]string{"SKILL", "AUTH", "DESCRIPTION"}, rows)
			return nil
		},
	}
}

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <skill>",
		Short: "Print a skill's SKILL.md",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			items, _ := skillsdata.List()
			names := make([]string, 0, len(items))
			for _, s := range items {
				names = append(names, s.Name)
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(_ *cobra.Command, args []string) error {
			s, err := skillsdata.Get(args[0])
			if err != nil {
				return err
			}
			fmt.Print(s.Content)
			return nil
		},
	}
}

func newInstallCmd() *cobra.Command {
	var client string
	var global bool
	cmd := &cobra.Command{
		Use:   "install [dir]",
		Short: "Install the skills into an agent's standard directory (or a custom dir)",
		Long: `Install the embedded skills so an AI agent can read them.

  --client claude   → .claude/skills/<name>/SKILL.md   (--global: ~/.claude/skills)
  --client cursor   → .cursor/rules/<name>.mdc         (--global: ~/.cursor/rules)
  [dir]             → <dir>/<name>/SKILL.md             (default: ./bifu-skills)

Cursor uses its Project Rules format (.mdc); Claude Code uses SKILL.md.`,
		Example: `  bifu-cli skills install --client claude
  bifu-cli skills install --client cursor --global
  bifu-cli skills install ./my-agent/skills`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			items, err := skillsdata.List()
			if err != nil {
				return err
			}

			dir, cursor, err := resolveTarget(client, global, args)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o755); err != nil { // #nosec G301 -- skill docs dir, not secret
				return fmt.Errorf("create %s: %w", dir, err)
			}

			pr := output.NewPrinter(output.FormatTable, false)
			for _, s := range items {
				var path string
				var data []byte
				if cursor {
					path = filepath.Join(dir, s.Name+".mdc")
					data = []byte(cursorMDC(s))
				} else {
					d := filepath.Join(dir, s.Name)
					if err := os.MkdirAll(d, 0o755); err != nil { // #nosec G301 -- skill docs dir, not secret
						return fmt.Errorf("create %s: %w", d, err)
					}
					path = filepath.Join(d, "SKILL.md")
					data = []byte(s.Content)
				}
				if err := os.WriteFile(path, data, 0o644); err != nil { // #nosec G306 -- skill doc (no secrets), meant to be readable by agents
					return fmt.Errorf("write %s: %w", path, err)
				}
				pr.Line("  %s", path)
			}
			pr.OK("Installed %d skills to %s", len(items), dir)
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "client", "", "Target agent: claude | cursor (writes to its standard dir)")
	cmd.Flags().BoolVar(&global, "global", false, "Install to the user-level dir (~/...) instead of the current project")
	_ = cmd.RegisterFlagCompletionFunc("client", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"claude", "cursor"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

// resolveTarget decides the output directory and whether to emit Cursor .mdc.
func resolveTarget(client string, global bool, args []string) (dir string, cursor bool, err error) {
	home, _ := os.UserHomeDir()
	switch client {
	case "claude":
		if global {
			return filepath.Join(home, ".claude", "skills"), false, nil
		}
		return filepath.Join(".claude", "skills"), false, nil
	case "cursor":
		if global {
			return filepath.Join(home, ".cursor", "rules"), true, nil
		}
		return filepath.Join(".cursor", "rules"), true, nil
	case "":
		dir = "bifu-skills"
		if len(args) == 1 {
			dir = filepath.Clean(expandHome(args[0], home))
			// Reject a symlinked target so a pre-planted symlink can't redirect the
			// write outside the intended directory (BIFU-CLI-202606-027).
			if fi, err := os.Lstat(dir); err == nil && fi.Mode()&os.ModeSymlink != 0 {
				return "", false, fmt.Errorf("refusing to install into a symlink: %s", dir)
			}
		}
		return dir, false, nil
	default:
		return "", false, fmt.Errorf("unknown --client %q (use: claude | cursor)", client)
	}
}

func expandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if len(p) >= 2 && p[:2] == "~/" {
		return filepath.Join(home, p[2:])
	}
	return p
}

// cursorMDC renders a skill as a Cursor Project Rule (.mdc).
func cursorMDC(s skillsdata.Skill) string {
	return fmt.Sprintf("---\ndescription: %s\nalwaysApply: false\n---\n\n%s", s.Description, s.Body)
}
