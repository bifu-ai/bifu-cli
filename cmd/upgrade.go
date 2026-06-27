package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"bifu-cli/internal/output"
)

// releasesRepo hosts the public release artifacts (same source install.sh uses).
const releasesRepo = "decodeex/bifu-cli-releases"

// latestVersion fetches the newest published release tag (e.g. "v1.1.2").
func latestVersion() (string, error) {
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Get("https://api.github.com/repos/" + releasesRepo + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release check failed: HTTP %d", resp.StatusCode)
	}
	var out struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.TagName == "" {
		return "", fmt.Errorf("no release tag in response")
	}
	return out.TagName, nil
}

// parseVer splits a dotted version (leading "v" and any -suffix ignored) into
// up to three numeric components.
func parseVer(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v = strings.SplitN(v, "-", 2)[0]
	var out [3]int
	for i, p := range strings.SplitN(v, ".", 3) {
		if i > 2 {
			break
		}
		out[i], _ = strconv.Atoi(p)
	}
	return out
}

// compareVersions returns -1, 0, 1 for a<b, a==b, a>b.
func compareVersions(a, b string) int {
	pa, pb := parseVer(a), parseVer(b)
	for i := 0; i < 3; i++ {
		switch {
		case pa[i] < pb[i]:
			return -1
		case pa[i] > pb[i]:
			return 1
		}
	}
	return 0
}

// installMethod guesses how this binary was installed (from its path) and
// returns a friendly label plus the command that upgrades it.
func installMethod() (label, command string) {
	exe, _ := os.Executable()
	switch {
	case strings.Contains(exe, "/Cellar/") || strings.Contains(exe, "/homebrew/") || strings.Contains(exe, "/Homebrew/"):
		return "Homebrew", "brew upgrade decodeex/tap/bifu-cli"
	case strings.Contains(exe, "/node_modules/") || strings.Contains(exe, "/.nvm/") || strings.Contains(exe, "/npm"):
		return "npm", "npm i -g @decodeex/bifu-cli"
	default:
		return "curl", "curl -fsSL https://cli.bifu.dev/install.sh | bash"
	}
}

// reportUpdateStatus checks the latest release and prints whether an upgrade is
// available, with the command appropriate to how this binary was installed.
func reportUpdateStatus(w io.Writer) {
	latest, err := latestVersion()
	if err != nil {
		fmt.Fprintln(w, output.Warn("⚠ could not check for updates: ")+err.Error())
		return
	}
	if version == "dev" {
		fmt.Fprintf(w, "dev build (latest release: %s)\n", latest)
		return
	}
	if compareVersions(version, latest) < 0 {
		label, command := installMethod()
		fmt.Fprintf(w, "%s update available: %s → %s\n  upgrade (%s): %s\n",
			output.Warn("⬆"), version, latest, label, command)
		return
	}
	fmt.Fprintf(w, "%s bifu-cli %s is up to date\n", output.Success("✓"), version)
}

func newUpgradeCmd() *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Check for and install the latest bifu-cli release",
		Long: `Check the latest published release and upgrade bifu-cli using the same
method it was installed with (Homebrew, npm, or the curl installer).

  bifu-cli upgrade           # upgrade to the latest release
  bifu-cli upgrade --check   # only report whether an update is available`,
		RunE: func(cmd *cobra.Command, args []string) error {
			latest, err := latestVersion()
			if err != nil {
				return fmt.Errorf("check latest version: %w", err)
			}
			label, command := installMethod()

			if version != "dev" && compareVersions(version, latest) >= 0 {
				fmt.Printf("%s bifu-cli %s is already the latest\n", output.Success("✓"), version)
				return nil
			}
			fmt.Printf("Update available: %s → %s (installed via %s)\n", version, latest, label)

			if checkOnly {
				fmt.Printf("  run: %s\n", command)
				return nil
			}
			if !globalYes {
				pr := output.NewPrinter(output.FormatPlain, globalVerbose)
				if !pr.Confirm(fmt.Sprintf("Run `%s`?", command)) {
					fmt.Println("Aborted.")
					return nil
				}
			}
			fmt.Printf("→ %s\n", command)
			c := exec.Command("sh", "-c", command)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("upgrade command failed: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only report whether an update is available (don't install)")
	return cmd
}
