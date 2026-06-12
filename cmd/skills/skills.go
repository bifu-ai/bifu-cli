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
	return &cobra.Command{
		Use:   "install [dir]",
		Short: "Write the SKILL.md files to a directory for your agent to read",
		Long: `Write each skill to <dir>/<name>/SKILL.md (default dir: ./bifu-skills).
Point your agent at that directory (e.g. an agent's skills/rules folder).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir := "bifu-skills"
			if len(args) == 1 {
				dir = args[0]
			}
			if dir == "~" || len(dir) >= 2 && dir[:2] == "~/" {
				home, _ := os.UserHomeDir()
				dir = filepath.Join(home, dir[1:])
			}
			items, err := skillsdata.List()
			if err != nil {
				return err
			}
			pr := output.NewPrinter(output.FormatTable, false)
			for _, s := range items {
				d := filepath.Join(dir, s.Name)
				if err := os.MkdirAll(d, 0o755); err != nil {
					return fmt.Errorf("create %s: %w", d, err)
				}
				p := filepath.Join(d, "SKILL.md")
				if err := os.WriteFile(p, []byte(s.Content), 0o644); err != nil {
					return fmt.Errorf("write %s: %w", p, err)
				}
				pr.Line("  %s", p)
			}
			pr.OK("Installed %d skills to %s", len(items), dir)
			return nil
		},
	}
}
