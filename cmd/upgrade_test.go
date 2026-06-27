package cmd

import "testing"

func TestParseVer(t *testing.T) {
	cases := map[string][3]int{
		"v1.1.2":        {1, 1, 2},
		"1.1.2":         {1, 1, 2},
		"v1.2":          {1, 2, 0},
		"v1.1.2-1-gabc": {1, 1, 2},
		"  v2.0.0  ":    {2, 0, 0},
		"dev":           {0, 0, 0},
	}
	for in, want := range cases {
		if got := parseVer(in); got != want {
			t.Errorf("parseVer(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.1.1", "v1.1.2", -1},
		{"v1.1.2", "v1.1.2", 0},
		{"1.1.2", "v1.1.2", 0}, // leading v ignored
		{"v1.2.0", "v1.1.9", 1},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.1.2-dirty", "v1.1.2", 0}, // suffix ignored
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestInstallMethod(t *testing.T) {
	// Path-based detection is exercised against the real executable path here;
	// just assert it returns a known label and a non-empty command.
	label, command := installMethod()
	switch label {
	case "Homebrew", "npm", "curl":
	default:
		t.Errorf("unexpected install label %q", label)
	}
	if command == "" {
		t.Error("upgrade command is empty")
	}
}
