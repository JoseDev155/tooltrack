package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func ids(tools []Tool) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.ID
	}
	return out
}

func TestMerge(t *testing.T) {
	user := []Tool{
		{ID: "gh", Name: "my gh", Enabled: false},
		{ID: "custom", Name: "Custom"},
	}
	defaults := []Tool{
		{ID: "uv", Name: "Astral uv"},
		{ID: "gh", Name: "GitHub CLI", Enabled: true},
	}
	got := Merge(user, defaults)
	want := []string{"gh", "custom", "uv"}
	if g := ids(got); len(g) != 3 || g[0] != want[0] || g[1] != want[1] || g[2] != want[2] {
		t.Fatalf("ids = %v, want %v", g, want)
	}
	if got[0].Name != "my gh" || got[0].Enabled {
		t.Errorf("the user's entry must win: %+v", got[0])
	}
}

func TestLoadMergedWithoutFileReturnsDefaults(t *testing.T) {
	got, err := LoadMerged(filepath.Join(t.TempDir(), "tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	def, _ := Defaults()
	if len(got) != len(def) {
		t.Errorf("got %d tools, want %d", len(got), len(def))
	}
}

func TestLoadMergedAddsNewPresets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tools.json")
	data := `[{"id":"uv","name":"Mine","binary":"uv","check_mode":"command","version_command":"uv -V","version_regex":"(\\d+)","enabled":false}]`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadMerged(path)
	if err != nil {
		t.Fatal(err)
	}
	def, _ := Defaults()
	if len(got) != len(def) {
		t.Fatalf("got %d tools, want %d (user's uv + the other presets)", len(got), len(def))
	}
	if got[0].ID != "uv" || got[0].Name != "Mine" || got[0].Enabled {
		t.Errorf("the user's uv must be kept untouched: %+v", got[0])
	}
}

func TestLoadMergedInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tools.json")
	os.WriteFile(path, []byte(`[{"id":"x"}]`), 0o644)
	if _, err := LoadMerged(path); err == nil {
		t.Error("expected an error for an invalid catalog")
	}
}

func TestSaveLoadRoundTripOmitsRuntimeFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "tools.json")
	tools, err := Defaults()
	if err != nil {
		t.Fatal(err)
	}
	tools[0].ResolvedPath = "/secret/path"
	tools[0].InstalledVersion = "9.9.9"
	tools[0].HasUpdate = true
	if err := Save(path, tools); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	for _, leaked := range []string{"/secret/path", "9.9.9", "has_update"} {
		if contains(string(raw), leaked) {
			t.Errorf("runtime data %q was written to disk", leaked)
		}
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(tools) || got[0].ID != tools[0].ID || got[0].HasUpdate {
		t.Errorf("round trip mismatch: %+v", got[0])
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temporary file left behind: %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("expected an error")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestMergeMigratesOldRustupEntry(t *testing.T) {
	defaults, err := Defaults()
	if err != nil {
		t.Fatal(err)
	}
	old := Tool{ID: "rustup", Name: "Rustup", Binary: "rustup", CheckMode: CheckModeGitHub, GitHubRepo: "rust-lang/rustup"}
	got := Merge([]Tool{old}, defaults)[0]
	if got.CheckMode != CheckModeURL || got.RemoteURL == "" || got.RemoteRegex == "" || got.GitHubRepo != "" {
		t.Errorf("old rustup entry not migrated: %+v", got)
	}

	custom := old
	custom.GitHubRepo = "someone/rustup-fork"
	if got := Merge([]Tool{custom}, defaults)[0]; got.CheckMode != CheckModeGitHub || got.GitHubRepo != "someone/rustup-fork" {
		t.Errorf("customised entry must be left alone: %+v", got)
	}
}
