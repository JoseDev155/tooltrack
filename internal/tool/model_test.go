package tool

import (
	"strings"
	"testing"

	"github.com/JoseDev155/tooltrack/assets"
)

func TestDefaultCatalogIsValid(t *testing.T) {
	tools, err := ParseCatalog(assets.DefaultTools())
	if err != nil {
		t.Fatalf("ParseCatalog(defaults): %v", err)
	}
	want := []string{"uv", "bun", "deno", "rustup", "fnm", "anaconda", "gh"}
	if len(tools) != len(want) {
		t.Fatalf("got %d tools, want %d", len(tools), len(want))
	}
	for i, id := range want {
		if tools[i].ID != id {
			t.Errorf("tool %d = %q, want %q", i, tools[i].ID, id)
		}
		if !tools[i].Enabled {
			t.Errorf("tool %q should be enabled by default", id)
		}
	}
}

func TestDefaultVersionRegexes(t *testing.T) {
	tools, err := ParseCatalog(assets.DefaultTools())
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string]struct{ out, want string }{
		"uv":       {"uv 0.5.1 (abc 2024-11-01)", "0.5.1"},
		"bun":      {"1.1.30", "1.1.30"},
		"deno":     {"deno 1.46.3 (stable, release, x86_64-unknown-linux-gnu)\nv8 12.9", "1.46.3"},
		"rustup":   {"rustup 1.27.1 (54dd3d00f 2024-04-24)", "1.27.1"},
		"fnm":      {"fnm 1.37.1", "1.37.1"},
		"anaconda": {"conda 24.9.2", "24.9.2"},
		"gh":       {"gh version 2.60.1 (2024-10-18)\nhttps://github.com/cli/cli/releases/tag/v2.60.1", "2.60.1"},
	}
	for _, tl := range tools {
		c, ok := outputs[tl.ID]
		if !ok {
			t.Fatalf("no sample output for %q", tl.ID)
		}
		m := mustCompile(t, tl.VersionRegex).FindStringSubmatch(c.out)
		if m == nil || m[1] != c.want {
			t.Errorf("%s: regex %q on %q = %v, want %q", tl.ID, tl.VersionRegex, c.out, m, c.want)
		}
	}
}

func TestUpdateCommandFor(t *testing.T) {
	tl := Tool{UpdateCommands: map[string]string{
		"default": "cargo install fnm --force",
		"darwin":  "brew upgrade fnm",
	}}
	cases := []struct {
		goos, want string
		ok         bool
	}{
		{"darwin", "brew upgrade fnm", true},
		{"linux", "cargo install fnm --force", true},
		{"windows", "cargo install fnm --force", true},
	}
	for _, c := range cases {
		got, ok := tl.UpdateCommandFor(c.goos)
		if got != c.want || ok != c.ok {
			t.Errorf("UpdateCommandFor(%q) = %q, %v; want %q, %v", c.goos, got, ok, c.want, c.ok)
		}
	}

	gh := Tool{UpdateCommands: map[string]string{"darwin": "brew upgrade gh"}}
	if cmd, ok := gh.UpdateCommandFor("linux"); ok || cmd != "" {
		t.Errorf("linux should have no command, got %q, %v", cmd, ok)
	}
}

func TestFallbacksFor(t *testing.T) {
	tl := Tool{Fallbacks: map[string][]string{"windows": {`C:\a`}}}
	if got := tl.FallbacksFor("windows"); len(got) != 1 {
		t.Errorf("windows fallbacks = %v", got)
	}
	if got := tl.FallbacksFor("linux"); len(got) != 0 {
		t.Errorf("linux fallbacks = %v, want none", got)
	}
}

func TestParseCatalogErrors(t *testing.T) {
	const base = `{"id":"x","binary":"x","check_mode":"command","version_command":"x -v","version_regex":"(\\d+)"}`
	cases := map[string]struct{ json, wantErr string }{
		"bad json":       {`[`, "invalid tool catalog"},
		"missing id":     {`[{"binary":"x"}]`, "without id"},
		"missing binary": {`[{"id":"x"}]`, "binary is required"},
		"unknown mode":   {`[{"id":"x","binary":"x","check_mode":"web"}]`, "unknown check_mode"},
		"github no repo": {`[{"id":"x","binary":"x","check_mode":"github","version_command":"x","version_regex":"a"}]`, "github_repo is required"},
		"url not https":  {`[{"id":"x","binary":"x","check_mode":"url","remote_url":"http://x.test/v","remote_regex":"a","version_command":"x","version_regex":"a"}]`, "remote_url must be an https URL"},
		"url no regex":   {`[{"id":"x","binary":"x","check_mode":"url","remote_url":"https://x.test/v","version_command":"x","version_regex":"a"}]`, "remote_regex is required"},
		"no version cmd": {`[{"id":"x","binary":"x","check_mode":"command"}]`, "version_command and version_regex"},
		"bad regex":      {`[{"id":"x","binary":"x","check_mode":"command","version_command":"x","version_regex":"("}]`, "invalid version_regex"},
		"bad remote":     {`[{"id":"x","binary":"x","check_mode":"command","version_command":"x","version_regex":"a","remote_regex":"("}]`, "invalid remote_regex"},
		"duplicate":      {`[` + base + `,` + base + `]`, "duplicate tool id"},
	}
	for name, c := range cases {
		_, err := ParseCatalog([]byte(c.json))
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: err = %v, want containing %q", name, err, c.wantErr)
		}
	}
}
