package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JoseDev155/tooltrack/internal/tool"
)

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("Default().Validate() = %v", err)
	}
}

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got != Default() {
		t.Errorf("got %+v, want defaults", got)
	}
}

func TestLoadPartialFileKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"language":"en"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Default()
	want.Language = LangEN
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadRejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"language":    `{"language":"fr"}`,
		"action":      `{"default_action":"nuke"}`,
		"concurrency": `{"max_concurrent_checks":0}`,
		"timeout":     `{"timeout_seconds":-1}`,
		"syntax":      `{`,
	}
	for name, body := range cases {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	want := Config{
		Language:            LangEN,
		DefaultAction:       ActionUpdateAll,
		GitHubToken:         "ghp_example",
		MaxConcurrentChecks: 2,
		TimeoutSeconds:      10,
	}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temporary file left behind: %v", err)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	cfg := Default()
	cfg.Language = "xx"
	path := filepath.Join(t.TempDir(), "config.json")
	if err := Save(path, cfg); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("invalid config must not be written: %v", err)
	}
}

func TestToken(t *testing.T) {
	t.Setenv(EnvGitHubToken, "from-env")
	if got := (Config{}).Token(); got != "from-env" {
		t.Errorf("empty file token: got %q, want env value", got)
	}
	if got := (Config{GitHubToken: "from-file"}).Token(); got != "from-file" {
		t.Errorf("file token: got %q", got)
	}
}

func TestDirHonoursEnv(t *testing.T) {
	t.Setenv(EnvHome, "/custom/home")
	got, err := Dir()
	if err != nil || got != "/custom/home" {
		t.Errorf("Dir() = %q, %v", got, err)
	}

	t.Setenv(EnvHome, "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	t.Setenv("HOME", "/home/u")
	got, err = Dir()
	if err != nil {
		t.Skipf("no user config dir on this system: %v", err)
	}
	if !strings.HasSuffix(got, "tooltrack") {
		t.Errorf("Dir() = %q, want path ending in tooltrack", got)
	}
}

func TestPathsFor(t *testing.T) {
	p := PathsFor("base")
	if p.Config != filepath.Join("base", "data", "config.json") ||
		p.Tools != filepath.Join("base", "data", "tools.json") ||
		p.Log != filepath.Join("base", "logs", "updater.log") {
		t.Errorf("unexpected layout: %+v", p)
	}
}

func TestEnsureDefaults(t *testing.T) {
	p := PathsFor(t.TempDir())
	if err := EnsureDefaults(p); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(p.Config)
	if err != nil || cfg != Default() {
		t.Errorf("config = %+v, %v", cfg, err)
	}
	data, err := os.ReadFile(p.Tools)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.ParseCatalog(data); err != nil {
		t.Errorf("tools.json is not a valid catalog: %v", err)
	}
	if info, err := os.Stat(filepath.Dir(p.Log)); err != nil || !info.IsDir() {
		t.Errorf("logs directory missing: %v", err)
	}
}

func TestEnsureDefaultsKeepsExistingFiles(t *testing.T) {
	p := PathsFor(t.TempDir())
	if err := EnsureDefaults(p); err != nil {
		t.Fatal(err)
	}
	custom := Default()
	custom.Language = LangEN
	if err := Save(p.Config, custom); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Tools, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureDefaults(p); err != nil {
		t.Fatal(err)
	}
	if got, _ := Load(p.Config); got != custom {
		t.Errorf("config overwritten: %+v", got)
	}
	if data, _ := os.ReadFile(p.Tools); string(data) != "[]" {
		t.Errorf("tools.json overwritten: %s", data)
	}
}

func TestDefaultLanguageIsEnglish(t *testing.T) {
	if got := Default().Language; got != LangEN {
		t.Errorf("default language = %q, want %q", got, LangEN)
	}
}
