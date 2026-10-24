package ui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/JoseDev155/tooltrack/internal/app"
	"github.com/JoseDev155/tooltrack/internal/config"
	"github.com/JoseDev155/tooltrack/internal/i18n"
	"github.com/JoseDev155/tooltrack/internal/logger"
	"github.com/JoseDev155/tooltrack/internal/tool"
)

type fakeRemote map[string]string

func (f fakeRemote) LatestRelease(_ context.Context, repo string) (string, error) {
	if v, ok := f[repo]; ok {
		return v, nil
	}
	return "", errors.New("rate limit exceeded")
}

func mkTool(id, name, repo, update string) tool.Tool {
	return tool.Tool{
		ID: id, Name: name, Binary: id, CheckMode: tool.CheckModeGitHub, GitHubRepo: repo,
		VersionCommand: id + " --version", VersionRegex: `v\s+(\d+\.\d+\.\d+)`,
		UpdateCommands: map[string]string{"default": update}, Enabled: true,
	}
}

// newModel builds a Model over a fake system where every tool reports
// version 1.0.0 and the latest releases are given by remote.
func newModel(t *testing.T, tools []tool.Tool, remote fakeRemote, mutate func(*app.App)) *Model {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	paths := config.PathsFor(t.TempDir())
	if err := config.EnsureDefaults(paths); err != nil {
		t.Fatal(err)
	}
	log, err := logger.Open(paths.Log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	tr, _ := i18n.New("en")
	a := &app.App{
		Cfg: config.Default(), Paths: paths, Tools: tools, T: tr, Log: log,
		Checker: &tool.Checker{
			GOOS: "linux",
			LookPath: func(f string) (string, error) {
				if f == "sh" {
					return exec.LookPath("sh")
				}
				return "/fake/" + f, nil
			},
			Output:         func(context.Context, string, []string, time.Duration) ([]byte, error) { return []byte("v 1.0.0"), nil },
			GitHub:         remote,
			VersionTimeout: time.Second,
		},
	}
	if mutate != nil {
		mutate(a)
	}
	return New(a)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// pump runs cmd and every command it leads to, feeding the resulting
// messages back into the model, until nothing is left to do. It reports
// whether the program asked to quit. Spinner ticks are skipped.
func pump(t *testing.T, m *Model, cmd tea.Cmd) (quit bool) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	deadline := time.After(10 * time.Second)
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		done := make(chan tea.Msg, 1)
		go func() { done <- c() }()
		var msg tea.Msg
		select {
		case msg = <-done:
		case <-deadline:
			t.Fatal("timed out waiting for a command")
		}
		switch msg := msg.(type) {
		case spinner.TickMsg:
		case tea.QuitMsg:
			quit = true
		case tea.BatchMsg:
			queue = append(queue, msg...)
		default:
			_, next := m.Update(msg)
			queue = append(queue, next)
		}
	}
	return quit
}

// press sends a key and runs the resulting commands.
func press(t *testing.T, m *Model, k string) bool {
	t.Helper()
	_, cmd := m.Update(key(k))
	return pump(t, m, cmd)
}

func start(t *testing.T, m *Model) {
	t.Helper()
	pump(t, m, m.Init())
}

func twoTools(updateA, updateB string) ([]tool.Tool, fakeRemote) {
	return []tool.Tool{
			mkTool("alpha", "Alpha", "o/alpha", updateA),
			mkTool("beta", "Beta", "o/beta", updateB),
		},
		fakeRemote{"o/alpha": "v2.0.0", "o/beta": "v2.0.0"}
}

func TestKeySpaceString(t *testing.T) {
	if s := key("space").String(); s != " " && s != "space" {
		t.Fatalf("test helper produces %q for space", s)
	}
}

func TestDashboardAfterCheck(t *testing.T) {
	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	remote["o/beta"] = "v1.0.0" // beta is up to date
	m := newModel(t, tools, remote, nil)
	if m.State() != app.StateLoading || !strings.Contains(m.View(), "Looking up versions") {
		t.Fatalf("initial state %v:\n%s", m.State(), m.View())
	}
	start(t, m)

	if m.State() != app.StateDashboard {
		t.Fatalf("state = %v", m.State())
	}
	view := m.View()
	for _, want := range []string{"Alpha", "Beta", "1.0.0", "2.0.0", "New version available", "Up to date"} {
		if !strings.Contains(view, want) {
			t.Errorf("dashboard is missing %q:\n%s", want, view)
		}
	}
}

func TestDashboardShowsLookupErrors(t *testing.T) {
	tools, _ := twoTools(`sh -c "true"`, `sh -c "true"`)
	m := newModel(t, tools, fakeRemote{}, nil) // every lookup fails
	start(t, m)
	view := m.View()
	if !strings.Contains(view, "Not verified") || !strings.Contains(view, "rate limit exceeded") {
		t.Errorf("view:\n%s", view)
	}
}

func TestMenuNavigationWraps(t *testing.T) {
	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	m := newModel(t, tools, remote, nil)
	start(t, m)

	press(t, m, "enter")
	if m.State() != app.StateActionSelect || m.cursor != 0 {
		t.Fatalf("state %v cursor %d", m.State(), m.cursor)
	}
	press(t, m, "up")
	if m.cursor != menuLen-1 {
		t.Errorf("up from the top should wrap to the end, cursor = %d", m.cursor)
	}
	press(t, m, "down")
	if m.cursor != 0 {
		t.Errorf("down from the end should wrap to the top, cursor = %d", m.cursor)
	}
	press(t, m, "j")
	press(t, m, "k")
	if m.cursor != 0 {
		t.Errorf("j then k should return to 0, cursor = %d", m.cursor)
	}
	press(t, m, "esc")
	if m.State() != app.StateDashboard {
		t.Errorf("esc should go back, state = %v", m.State())
	}
	for _, want := range []string{"Update all", "Select tools to update", "Select tools to exclude", "Tool preferences", "Exit"} {
		press(t, m, "enter")
		if !strings.Contains(m.View(), want) {
			t.Errorf("menu is missing %q:\n%s", want, m.View())
		}
		press(t, m, "esc")
	}
}

func TestUpdateAll(t *testing.T) {
	tools, remote := twoTools(`sh -c "echo updated-alpha"`, `sh -c "echo updated-beta"`)
	m := newModel(t, tools, remote, nil)
	start(t, m)
	press(t, m, "enter") // dashboard -> menu
	press(t, m, "enter") // "Update all"

	if m.State() != app.StateFinished {
		t.Fatalf("state = %v", m.State())
	}
	if len(m.results) != 2 || m.results[0].Err != nil || m.results[1].Err != nil {
		t.Fatalf("results = %+v", m.results)
	}
	view := m.View()
	if !strings.Contains(view, "Update summary") || !strings.Contains(view, "Alpha") || strings.Contains(view, "An error occurred") {
		t.Errorf("summary:\n%s", view)
	}
	data, _ := os.ReadFile(m.app.Paths.Log)
	if !strings.Contains(string(data), "updated-alpha") || !strings.Contains(string(data), "updated-beta") {
		t.Errorf("tool output is not in the log:\n%s", data)
	}

	press(t, m, "enter") // back to start: checks again
	if m.State() != app.StateDashboard {
		t.Errorf("state = %v, want dashboard", m.State())
	}
}

func TestUpdateSelected(t *testing.T) {
	tools, remote := twoTools(`sh -c "echo only-alpha"`, `sh -c "echo only-beta"`)
	m := newModel(t, tools, remote, nil)
	start(t, m)
	press(t, m, "enter")
	press(t, m, "down")
	press(t, m, "enter") // "Select tools to update"
	if m.State() != app.StateSelectUpdate || !strings.Contains(m.View(), "[ ] Alpha") {
		t.Fatalf("state %v:\n%s", m.State(), m.View())
	}
	press(t, m, "space") // tick Alpha
	if !strings.Contains(m.View(), "[x] Alpha") {
		t.Errorf("Alpha should be ticked:\n%s", m.View())
	}
	press(t, m, "enter")

	if m.State() != app.StateFinished || len(m.results) != 1 || m.results[0].ID != "alpha" {
		t.Fatalf("state %v results %+v", m.State(), m.results)
	}
	data, _ := os.ReadFile(m.app.Paths.Log)
	if !strings.Contains(string(data), "only-alpha") || strings.Contains(string(data), "only-beta") {
		t.Errorf("log:\n%s", data)
	}
}

func TestSelectionToggleOff(t *testing.T) {
	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	m := newModel(t, tools, remote, nil)
	start(t, m)
	press(t, m, "enter")
	press(t, m, "down")
	press(t, m, "enter")
	press(t, m, "space")
	press(t, m, "space")
	if strings.Contains(m.View(), "[x]") {
		t.Errorf("a second space should untick:\n%s", m.View())
	}
	press(t, m, "enter") // nothing ticked
	if m.State() != app.StateSelectUpdate || !strings.Contains(m.View(), "No tools have updates available") {
		t.Errorf("state %v:\n%s", m.State(), m.View())
	}
}

func TestUpdateExcluding(t *testing.T) {
	tools, remote := twoTools(`sh -c "echo ran-alpha"`, `sh -c "echo ran-beta"`)
	m := newModel(t, tools, remote, nil)
	start(t, m)
	press(t, m, "enter")
	press(t, m, "down")
	press(t, m, "down")
	press(t, m, "enter") // "Select tools to exclude"
	if m.State() != app.StateSelectExclude {
		t.Fatalf("state = %v", m.State())
	}
	press(t, m, "space") // exclude Alpha
	press(t, m, "enter")

	if len(m.results) != 1 || m.results[0].ID != "beta" {
		t.Fatalf("results = %+v", m.results)
	}
}

func TestNothingToUpdate(t *testing.T) {
	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	remote["o/alpha"], remote["o/beta"] = "v1.0.0", "v1.0.0"
	m := newModel(t, tools, remote, nil)
	start(t, m)
	press(t, m, "enter")
	press(t, m, "enter") // "Update all"
	if m.State() != app.StateActionSelect || !strings.Contains(m.View(), "No tools have updates available") {
		t.Errorf("state %v:\n%s", m.State(), m.View())
	}

	press(t, m, "down")
	press(t, m, "enter") // selection screen with nothing to select
	if m.State() != app.StateSelectUpdate || !strings.Contains(m.View(), "No tools have updates available") {
		t.Errorf("state %v:\n%s", m.State(), m.View())
	}
	press(t, m, "enter")
	if m.State() != app.StateActionSelect {
		t.Errorf("enter on an empty selection should go back, state = %v", m.State())
	}
}

func TestFailureShowsLogAndOpensIt(t *testing.T) {
	tools, remote := twoTools(`sh -c "echo broke >&2; exit 3"`, `sh -c "true"`)
	m := newModel(t, tools, remote, nil)
	var opened string
	old := OpenFile
	OpenFile = func(p string) error { opened = p; return nil }
	defer func() { OpenFile = old }()

	start(t, m)
	press(t, m, "enter")
	press(t, m, "enter")

	view := m.View()
	for _, want := range []string{"An error occurred during the update", "See the message in the log file: " + m.app.Paths.Log, "open the log"} {
		if !strings.Contains(view, want) {
			t.Errorf("summary is missing %q:\n%s", want, view)
		}
	}
	press(t, m, "l")
	if opened != m.app.Paths.Log {
		t.Errorf("opened %q, want the log path", opened)
	}
}

func TestOpenLogOnlyWhenSomethingFailed(t *testing.T) {
	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	m := newModel(t, tools, remote, nil)
	old := OpenFile
	OpenFile = func(p string) error { t.Error("log opened without failures"); return nil }
	defer func() { OpenFile = old }()
	start(t, m)
	press(t, m, "enter")
	press(t, m, "enter")
	press(t, m, "l")
	if strings.Contains(m.View(), "open the log") {
		t.Error("the open-log hint must only appear after a failure")
	}
}

func TestOpenLogErrorIsShown(t *testing.T) {
	tools, remote := twoTools(`sh -c "exit 1"`, `sh -c "true"`)
	m := newModel(t, tools, remote, nil)
	old := OpenFile
	OpenFile = func(string) error { return errors.New("no viewer installed") }
	defer func() { OpenFile = old }()
	start(t, m)
	press(t, m, "enter")
	press(t, m, "enter")
	press(t, m, "l")
	if !strings.Contains(m.View(), "no viewer installed") {
		t.Errorf("view:\n%s", m.View())
	}
}

func TestToolWithoutUpdateCommandIsNotOffered(t *testing.T) {
	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	tools[1].UpdateCommands = map[string]string{"darwin": "brew upgrade beta"}
	m := newModel(t, tools, remote, nil)
	start(t, m)
	press(t, m, "enter")
	press(t, m, "enter")
	if len(m.results) != 1 || m.results[0].ID != "alpha" {
		t.Errorf("results = %+v", m.results)
	}
}

func TestSettingsChangeLanguageAndDefaultAction(t *testing.T) {
	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	m := newModel(t, tools, remote, func(a *app.App) { a.Cfg.Language = "en" })
	start(t, m)
	press(t, m, "enter")
	for i := 0; i < 3; i++ {
		press(t, m, "down")
	}
	press(t, m, "enter") // Tool preferences
	if m.State() != app.StateSettings {
		t.Fatalf("state = %v", m.State())
	}
	if !strings.Contains(m.View(), "Language") || !strings.Contains(m.View(), "English") {
		t.Errorf("view:\n%s", m.View())
	}

	press(t, m, "enter") // language en -> es
	if m.app.Cfg.Language != "es" || !strings.Contains(m.View(), "Idioma") || !strings.Contains(m.View(), "Guardado") {
		t.Errorf("language not switched:\n%s", m.View())
	}
	press(t, m, "down")
	press(t, m, "space") // default action ask -> update_all
	if m.app.Cfg.DefaultAction != config.ActionUpdateAll {
		t.Errorf("default action = %q", m.app.Cfg.DefaultAction)
	}

	saved, err := config.Load(m.app.Paths.Config)
	if err != nil || saved.Language != "es" || saved.DefaultAction != config.ActionUpdateAll {
		t.Errorf("saved config = %+v, %v", saved, err)
	}
	press(t, m, "esc")
	if m.State() != app.StateActionSelect {
		t.Errorf("state = %v", m.State())
	}
}

func TestDefaultActionUpdateAllRunsAtStartup(t *testing.T) {
	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	m := newModel(t, tools, remote, func(a *app.App) { a.Cfg.DefaultAction = config.ActionUpdateAll })
	start(t, m)
	if m.State() != app.StateFinished || len(m.results) != 2 {
		t.Fatalf("state %v results %+v", m.State(), m.results)
	}
	press(t, m, "enter") // back to the start: must NOT update again
	if m.State() != app.StateDashboard {
		t.Errorf("state = %v, want dashboard", m.State())
	}
}

func TestDefaultActionSelectionScreens(t *testing.T) {
	for action, want := range map[string]app.State{
		config.ActionUpdateSelected:  app.StateSelectUpdate,
		config.ActionExcludeSelected: app.StateSelectExclude,
		config.ActionAsk:             app.StateDashboard,
	} {
		tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
		m := newModel(t, tools, remote, func(a *app.App) { a.Cfg.DefaultAction = action })
		start(t, m)
		if m.State() != want {
			t.Errorf("default_action %q: state = %v, want %v", action, m.State(), want)
		}
	}
}

func TestDefaultActionUpdateAllWithNothingToUpdate(t *testing.T) {
	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	remote["o/alpha"], remote["o/beta"] = "v1.0.0", "v1.0.0"
	m := newModel(t, tools, remote, func(a *app.App) { a.Cfg.DefaultAction = config.ActionUpdateAll })
	start(t, m)
	if m.State() != app.StateDashboard {
		t.Errorf("state = %v, want dashboard", m.State())
	}
}

func TestRefreshKey(t *testing.T) {
	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	m := newModel(t, tools, remote, nil)
	start(t, m)
	remote["o/alpha"] = "v1.0.0"
	press(t, m, "r")
	if m.State() != app.StateDashboard || m.app.Tools[0].HasUpdate {
		t.Errorf("state %v, alpha.HasUpdate %v", m.State(), m.app.Tools[0].HasUpdate)
	}
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
		m := newModel(t, tools, remote, nil)
		start(t, m)
		if !press(t, m, k) {
			t.Errorf("%s should quit from the dashboard", k)
		}
	}

	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	m := newModel(t, tools, remote, nil)
	start(t, m)
	press(t, m, "enter")
	for i := 0; i < menuExit; i++ {
		press(t, m, "down")
	}
	if !press(t, m, "enter") {
		t.Error(`the "Exit" entry should quit`)
	}
}

func TestQuitWhileLoading(t *testing.T) {
	tools, remote := twoTools(`sh -c "true"`, `sh -c "true"`)
	m := newModel(t, tools, remote, nil)
	_, cmd := m.Update(key("q"))
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Errorf("got %T, want QuitMsg", msg)
	}
	if m.checkCtx.Err() == nil {
		t.Error("quitting must cancel the version lookup")
	}
}

func TestQuitDuringUpdateCancelsAndWaits(t *testing.T) {
	tools, remote := twoTools(`sh -c "sleep 30"`, `sh -c "true"`)
	m := newModel(t, tools, remote, nil)
	start(t, m)
	press(t, m, "enter")

	// Start "Update all" without waiting for it to finish.
	_, cmd := m.Update(key("enter"))
	if m.State() != app.StateUpdating {
		t.Fatalf("state = %v", m.State())
	}
	if _, quitCmd := m.Update(key("q")); quitCmd != nil {
		t.Error("quit during an update must wait for the run to wind down")
	}
	if !m.quitting || !strings.Contains(m.View(), "Closing") {
		t.Errorf("view:\n%s", m.View())
	}

	began := time.Now()
	if !pump(t, m, cmd) {
		t.Fatal("the program should quit once the cancelled run ends")
	}
	if time.Since(began) > 8*time.Second {
		t.Error("cancelling did not stop the running tool")
	}
}

func TestKeysIgnoredWhileUpdating(t *testing.T) {
	tools, remote := twoTools(`sh -c "sleep 30"`, `sh -c "true"`)
	m := newModel(t, tools, remote, nil)
	start(t, m)
	press(t, m, "enter")
	_, cmd := m.Update(key("enter"))
	defer func() { m.cancelRun(); pump(t, m, cmd) }()

	for _, k := range []string{"enter", "down", "esc", "space"} {
		m.Update(key(k))
	}
	if m.State() != app.StateUpdating {
		t.Errorf("state = %v", m.State())
	}
}

func TestViewShowsProgressWhileUpdating(t *testing.T) {
	tools, remote := twoTools(`sh -c "sleep 30"`, `sh -c "true"`)
	m := newModel(t, tools, remote, nil)
	start(t, m)
	press(t, m, "enter")
	_, cmd := m.Update(key("enter"))
	defer func() { m.cancelRun(); pump(t, m, cmd) }()

	view := m.View()
	if !strings.Contains(view, "Updating tools") || !strings.Contains(view, "Alpha") || !strings.Contains(view, "Beta") {
		t.Errorf("view:\n%s", view)
	}
}

func TestNextWraps(t *testing.T) {
	if got := next([]string{"a", "b"}, "b"); got != "a" {
		t.Errorf("got %q", got)
	}
	if got := next([]string{"a", "b"}, "zzz"); got != "a" {
		t.Errorf("unknown current value should start over, got %q", got)
	}
}
