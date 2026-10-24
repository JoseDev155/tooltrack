package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/JoseDev155/tooltrack/internal/config"
	"github.com/JoseDev155/tooltrack/internal/i18n"
	"github.com/JoseDev155/tooltrack/internal/logger"
	"github.com/JoseDev155/tooltrack/internal/tool"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
}

// newApp builds an App over a temporary directory with the given tools. The
// checker runs on a fake Linux system where only the names in onPath exist.
func newApp(t *testing.T, tools []tool.Tool) *App {
	t.Helper()
	paths := config.PathsFor(t.TempDir())
	if err := config.EnsureDefaults(paths); err != nil {
		t.Fatal(err)
	}
	log, err := logger.Open(paths.Log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	tr, err := i18n.New("en")
	if err != nil {
		t.Fatal(err)
	}
	return &App{
		Cfg: config.Default(), Paths: paths, Tools: tools, T: tr, Log: log,
		Checker: &tool.Checker{GOOS: "linux", LookPath: func(f string) (string, error) { return "", errors.New("nope") }},
	}
}

func installed(id, name string, update bool) tool.Tool {
	return tool.Tool{
		ID: id, Name: name, Binary: id, Enabled: true, IsInstalled: true,
		InstalledVersion: "1.0.0", RemoteVersion: "1.0.0", HasUpdate: update,
		UpdateCommands: map[string]string{"default": id + " update"},
	}
}

func TestStatusOf(t *testing.T) {
	cases := []struct {
		name string
		tool tool.Tool
		want Status
	}{
		{"not installed", tool.Tool{}, StatusNotInstalled},
		{"up to date", tool.Tool{IsInstalled: true, RemoteVersion: "1.0.0"}, StatusUpToDate},
		{"update", tool.Tool{IsInstalled: true, RemoteVersion: "1.1.0", HasUpdate: true}, StatusUpdateAvailable},
		{"lookup error", tool.Tool{IsInstalled: true, LastError: "rate limit"}, StatusUnknown},
		{"no remote version", tool.Tool{IsInstalled: true}, StatusUnknown},
	}
	for _, c := range cases {
		if got := StatusOf(&c.tool); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestStatusKeysExistInEveryLocale(t *testing.T) {
	for _, lang := range []string{"es", "en"} {
		tr, err := i18n.New(lang)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []Status{StatusNotInstalled, StatusUpToDate, StatusUpdateAvailable, StatusUnknown} {
			if got := tr.T(s.StatusKey()); got == s.StatusKey() {
				t.Errorf("%s: no translation for %q", lang, s.StatusKey())
			}
		}
	}
}

func TestStateString(t *testing.T) {
	seen := map[string]bool{}
	for s := StateLoading; s <= StateSettings; s++ {
		name := s.String()
		if name == "unknown" || seen[name] {
			t.Errorf("state %d has name %q", s, name)
		}
		seen[name] = true
	}
	if State(99).String() != "unknown" {
		t.Error("out-of-range state should be unknown")
	}
}

func TestUpdatableFiltersTools(t *testing.T) {
	disabled := installed("off", "Off", true)
	disabled.Enabled = false
	missing := installed("missing", "Missing", true)
	missing.IsInstalled = false
	current := installed("current", "Current", false)
	noCmd := installed("nocmd", "NoCmd", true)
	noCmd.UpdateCommands = map[string]string{"darwin": "brew upgrade x"}

	a := newApp(t, []tool.Tool{installed("a", "A", true), disabled, missing, current, noCmd})
	var ids []string
	for _, tl := range a.Updatable() {
		ids = append(ids, tl.ID)
	}
	if !reflect.DeepEqual(ids, []string{"a"}) {
		t.Errorf("Updatable = %v, want [a]", ids)
	}
}

func TestPlan(t *testing.T) {
	a := newApp(t, []tool.Tool{installed("a", "A", true), installed("b", "B", true), installed("c", "C", true), installed("d", "D", false)})
	picked := map[string]bool{"b": true, "d": true}
	cases := []struct {
		action Action
		want   []string
	}{
		{ActionAll, []string{"a", "b", "c"}},
		{ActionSelected, []string{"b"}}, // d is picked but has no update
		{ActionExclude, []string{"a", "c"}},
	}
	for _, c := range cases {
		if got := a.Plan(c.action, picked); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Plan(%v) = %v, want %v", c.action, got, c.want)
		}
	}
	if got := a.Plan(ActionSelected, nil); len(got) != 0 {
		t.Errorf("nothing picked should plan nothing, got %v", got)
	}
}

func TestVisibleSkipsDisabled(t *testing.T) {
	off := installed("off", "Off", false)
	off.Enabled = false
	a := newApp(t, []tool.Tool{installed("on", "On", false), off})
	if got := a.Visible(); len(got) != 1 || got[0].ID != "on" {
		t.Errorf("Visible = %v", got)
	}
}

func TestSortToolsAlphabetical(t *testing.T) {
	a := newApp(t, []tool.Tool{{Name: "zeta"}, {Name: "Alpha"}, {Name: "beta"}})
	a.sortTools()
	var names []string
	for _, tl := range a.Tools {
		names = append(names, tl.Name)
	}
	if !reflect.DeepEqual(names, []string{"Alpha", "beta", "zeta"}) {
		t.Errorf("names = %v", names)
	}
}

func shTool(id, script string) tool.Tool {
	return tool.Tool{
		ID: id, Name: id, Binary: id, Enabled: true, IsInstalled: true, HasUpdate: true,
		UpdateCommands: map[string]string{"default": `sh -c "` + script + `"`},
	}
}

func TestRunUpdates(t *testing.T) {
	skipOnWindows(t)
	a := newApp(t, []tool.Tool{shTool("ok", "echo updated-ok"), shTool("bad", "echo broke >&2; exit 2"), shTool("ok2", "true")})
	a.Checker.LookPath = func(f string) (string, error) { return f, nil } // "sh" is found by exec itself

	events := make(chan UpdateEvent, 16)
	results := a.RunUpdates(context.Background(), []string{"ok", "bad", "ok2"}, events)
	close(events)

	if len(results) != 3 || results[0].Err != nil || results[1].Err == nil || results[2].Err != nil {
		t.Fatalf("results = %+v", results)
	}
	for i, id := range []string{"ok", "bad", "ok2"} {
		if results[i].ID != id || results[i].Name != id {
			t.Errorf("result %d = %+v", i, results[i])
		}
	}

	var started, finished []string
	for ev := range events {
		if ev.Result == nil {
			started = append(started, ev.ID)
		} else {
			finished = append(finished, ev.ID)
		}
	}
	if !reflect.DeepEqual(started, []string{"ok", "bad", "ok2"}) || !reflect.DeepEqual(finished, []string{"ok", "bad", "ok2"}) {
		t.Errorf("started %v, finished %v", started, finished)
	}

	logData, err := os.ReadFile(a.Paths.Log)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"updated-ok", "broke", "update of bad failed"} {
		if !strings.Contains(string(logData), want) {
			t.Errorf("log is missing %q:\n%s", want, logData)
		}
	}
}

func TestRunUpdatesReportsUnbuildableJobs(t *testing.T) {
	a := newApp(t, []tool.Tool{
		{ID: "gh", Name: "GitHub CLI", Binary: "gh", UpdateCommands: map[string]string{"darwin": "brew upgrade gh"}},
	})
	events := make(chan UpdateEvent, 8)
	results := a.RunUpdates(context.Background(), []string{"gh", "ghost"}, events)
	close(events)

	if results[0].Err == nil || !strings.Contains(results[0].Err.Error(), "no update command") {
		t.Errorf("gh: %v", results[0].Err)
	}
	if results[1].Err == nil || !strings.Contains(results[1].Err.Error(), "unknown tool") {
		t.Errorf("ghost: %v", results[1].Err)
	}
	var n int
	for ev := range events {
		if ev.Result != nil {
			n++
		}
	}
	if n != 2 {
		t.Errorf("got %d finished events, want 2 (one per failed tool)", n)
	}
}

func TestRunUpdatesWithoutEventChannel(t *testing.T) {
	skipOnWindows(t)
	a := newApp(t, []tool.Tool{shTool("ok", "true")})
	a.Checker.LookPath = func(f string) (string, error) { return f, nil }
	if res := a.RunUpdates(context.Background(), []string{"ok"}, nil); res[0].Err != nil {
		t.Errorf("err = %v", res[0].Err)
	}
}

func TestSettersPersistAndSwitchLanguage(t *testing.T) {
	a := newApp(t, nil)
	if err := a.SetLanguage("es"); err != nil {
		t.Fatal(err)
	}
	if a.T.Lang() != "es" || a.T.T("menu.exit") != "Salir" {
		t.Errorf("translator not switched: %q", a.T.Lang())
	}
	if err := a.SetDefaultAction(config.ActionUpdateAll); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(a.Paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if got.Language != "es" || got.DefaultAction != config.ActionUpdateAll {
		t.Errorf("saved config = %+v", got)
	}
	if err := a.SetLanguage("xx"); err == nil {
		t.Error("an unknown language must be rejected")
	}
	if a.Cfg.Language != "es" {
		t.Error("a failed change must leave the config untouched")
	}
}

func TestSettersDoNotPersistEnvironmentToken(t *testing.T) {
	t.Setenv(config.EnvGitHubToken, "from-env-secret")
	a := newApp(t, nil)
	if err := a.SetLanguage("en"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(a.Paths.Config)
	if strings.Contains(string(data), "from-env-secret") {
		t.Errorf("the environment token was written to config.json:\n%s", data)
	}
}

func TestNewLoadsEverything(t *testing.T) {
	t.Setenv(config.EnvHome, "")
	paths := config.PathsFor(t.TempDir())
	a, err := New(paths)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if len(a.Tools) != 7 || a.T.Lang() != "es" || a.Checker == nil {
		t.Errorf("unexpected app: %d tools, lang %q", len(a.Tools), a.T.Lang())
	}
	if a.Tools[0].Name != "Astral uv" && strings.ToLower(a.Tools[0].Name) > strings.ToLower(a.Tools[1].Name) {
		t.Errorf("tools are not sorted: %q, %q", a.Tools[0].Name, a.Tools[1].Name)
	}
	if _, err := os.Stat(filepath.Join(paths.Dir, "logs", "updater.log")); err != nil {
		t.Errorf("log not created: %v", err)
	}
}

func TestNewFailsOnInvalidConfig(t *testing.T) {
	paths := config.PathsFor(t.TempDir())
	if err := config.EnsureDefaults(paths); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(paths.Config, []byte(`{"language":"fr"}`), 0o600)
	if _, err := New(paths); err == nil {
		t.Error("expected an error")
	}
}

func TestCheckUsesConfiguredConcurrency(t *testing.T) {
	a := newApp(t, []tool.Tool{{ID: "x", Name: "X", Binary: "x", Enabled: true}})
	a.Cfg.MaxConcurrentChecks = 1
	a.Check(context.Background()) // not installed: must complete without error or panic
	if a.Tools[0].IsInstalled {
		t.Error("tool should not be found")
	}
}
