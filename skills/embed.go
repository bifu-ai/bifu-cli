// Package skills embeds the agent SKILL.md instruction files and exposes them
// for the `bifu-cli skills` command. Each subdirectory holds one SKILL.md whose
// YAML frontmatter declares name / description / auth.
package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed */SKILL.md
var fsys embed.FS

// Skill is one embedded agent skill.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Auth        string `json:"auth"` // none | required | partial
	Content     string `json:"-"`    // full SKILL.md text
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Auth        string `yaml:"auth"`
}

// List returns all embedded skills, sorted by name.
func List() ([]Skill, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var out []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := load(e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Get returns a single skill by its directory/name.
func Get(name string) (Skill, error) {
	return load(name)
}

func load(dir string) (Skill, error) {
	b, err := fsys.ReadFile(dir + "/SKILL.md")
	if err != nil {
		return Skill{}, fmt.Errorf("skill %q not found", dir)
	}
	content := string(b)
	fm := parseFrontmatter(content)
	name := fm.Name
	if name == "" {
		name = dir
	}
	auth := fm.Auth
	if auth == "" {
		auth = "none"
	}
	return Skill{Name: name, Description: fm.Description, Auth: auth, Content: content}, nil
}

func parseFrontmatter(s string) frontmatter {
	var fm frontmatter
	if !strings.HasPrefix(s, "---") {
		return fm
	}
	rest := s[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return fm
	}
	_ = yaml.Unmarshal([]byte(rest[:end]), &fm)
	return fm
}
