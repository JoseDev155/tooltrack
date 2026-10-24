// Package ui is the Bubble Tea front end.
package ui

import (
	"context"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/JoseDev155/tooltrack/internal/app"
	"github.com/JoseDev155/tooltrack/internal/config"
	"github.com/JoseDev155/tooltrack/internal/ui/styles"
	"github.com/JoseDev155/tooltrack/internal/ui/views"
)

// OpenFile opens a file in the system's default viewer. Tests replace it.
var OpenFile = defaultOpenFile

func defaultOpenFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// Messages produced by background work.
type (
	checkedMsg  struct{}
	eventMsg    app.UpdateEvent
	finishedMsg struct{}
)

// Entries of StateMainMenu, in order.
const (
	mainStatus = iota
	mainSettings
	mainExit
	mainLen
)

// Entries of StateActionSelect, in order.
const (
	menuAll = iota
	menuSelect
	menuExclude
	menuExit
	menuLen
)

var (
	languages = []string{config.LangEN, config.LangES}
	actions   = []string{config.ActionAsk, config.ActionUpdateAll, config.ActionUpdateSelected, config.ActionExcludeSelected}
)

const settingsRows = 2

// Model is the Bubble Tea model. All screen logic lives here; rendering is
// delegated to the views package.
type Model struct {
	app    *app.App
	state  app.State
	cursor int
	picked map[string]bool
	note   string
	spin   spinner.Model

	autoApplied bool // default_action already applied this session

	// Check phase.
	checkCtx    context.Context
	cancelCheck context.CancelFunc

	// Update phase.
	rows      []views.ProgressRow
	rowIndex  map[string]int
	ids       []string
	byID      map[string]app.UpdateResult
	results   []app.UpdateResult
	events    chan app.UpdateEvent
	cancelRun context.CancelFunc
	cancelled bool
	quitting  bool
}

// New returns the model for a loaded application.
func New(a *app.App) *Model {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	s.Style = styles.Cursor
	return &Model{app: a, spin: s, state: app.StateMainMenu, picked: map[string]bool{}}
}

// State returns the current screen.
func (m *Model) State() app.State { return m.state }

// startCheck switches to the loading screen and looks up every version.
func (m *Model) startCheck() tea.Cmd {
	m.state = app.StateLoading
	m.cursor = 0
	m.note = ""
	m.checkCtx, m.cancelCheck = context.WithCancel(context.Background())
	return tea.Batch(m.spin.Tick, m.checkCmd())
}

// goTo shows another screen with the cursor on its first entry.
func (m *Model) goTo(s app.State) {
	m.state, m.cursor, m.note = s, 0, ""
}

func (m *Model) checkCmd() tea.Cmd {
	ctx, a := m.checkCtx, m.app
	return func() tea.Msg {
		a.Check(ctx)
		return checkedMsg{}
	}
}

// Init does nothing: versions are looked up only when the user asks for the
// status, not when the program starts.
func (m *Model) Init() tea.Cmd { return nil }

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case checkedMsg:
		if m.state != app.StateLoading {
			return m, nil
		}
		return m.afterCheck()

	case eventMsg:
		m.onEvent(app.UpdateEvent(msg))
		return m, m.waitEvent()

	case finishedMsg:
		return m.onFinished()

	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

// afterCheck shows the dashboard, or applies the configured default action
// the first time.
func (m *Model) afterCheck() (tea.Model, tea.Cmd) {
	m.state, m.cursor, m.note = app.StateDashboard, 0, ""
	if m.autoApplied {
		return m, nil
	}
	m.autoApplied = true
	switch m.app.Cfg.DefaultAction {
	case config.ActionUpdateAll:
		if ids := m.app.Plan(app.ActionAll, nil); len(ids) > 0 {
			return m, m.startUpdate(ids)
		}
	case config.ActionUpdateSelected:
		m.enterSelection(app.StateSelectUpdate)
	case config.ActionExcludeSelected:
		m.enterSelection(app.StateSelectExclude)
	}
	return m, nil
}

func (m *Model) enterSelection(s app.State) {
	m.state, m.cursor, m.note = s, 0, ""
	m.picked = map[string]bool{}
}

func (m *Model) quit() (tea.Model, tea.Cmd) {
	if m.cancelCheck != nil {
		m.cancelCheck()
	}
	return m, tea.Quit
}

func (m *Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" || key == "q" {
		if m.state == app.StateUpdating {
			// Stop the running tool, wait for the run to wind down, then quit.
			m.cancelled, m.quitting = true, true
			m.cancelRun()
			return m, nil
		}
		return m.quit()
	}
	if m.state == app.StateUpdating || m.state == app.StateLoading {
		return m, nil
	}

	switch key {
	case "up", "k":
		m.move(-1)
		return m, nil
	case "down", "j":
		m.move(1)
		return m, nil
	}

	switch m.state {
	case app.StateMainMenu:
		return m.onMainKey(key)
	case app.StateDashboard:
		switch key {
		case "enter":
			m.goTo(app.StateActionSelect)
		case "r":
			return m, m.startCheck()
		case "esc":
			m.goTo(app.StateMainMenu)
		}
	case app.StateActionSelect:
		return m.onMenuKey(key)
	case app.StateSelectUpdate, app.StateSelectExclude:
		return m.onSelectKey(key)
	case app.StateSettings:
		return m.onSettingsKey(key)
	case app.StateFinished:
		switch key {
		case "enter":
			m.goTo(app.StateMainMenu)
		case "l":
			if m.hasFailures() {
				if err := OpenFile(m.app.Paths.Log); err != nil {
					m.note = err.Error()
				}
			}
		}
	}
	return m, nil
}

func (m *Model) move(delta int) {
	n := 0
	switch m.state {
	case app.StateMainMenu:
		n = mainLen
	case app.StateActionSelect:
		n = menuLen
	case app.StateSelectUpdate, app.StateSelectExclude:
		n = len(m.app.Updatable())
	case app.StateSettings:
		n = settingsRows
	}
	if n == 0 {
		return
	}
	m.cursor = (m.cursor + delta + n) % n
}

func (m *Model) onMainKey(key string) (tea.Model, tea.Cmd) {
	if key != "enter" {
		return m, nil
	}
	switch m.cursor {
	case mainStatus:
		return m, m.startCheck()
	case mainSettings:
		m.goTo(app.StateSettings)
	case mainExit:
		return m.quit()
	}
	return m, nil
}

func (m *Model) onMenuKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state, m.cursor, m.note = app.StateDashboard, 0, ""
	case "enter":
		m.note = ""
		switch m.cursor {
		case menuAll:
			ids := m.app.Plan(app.ActionAll, nil)
			if len(ids) == 0 {
				m.note = m.app.T.T("app.nothing_to_update")
				return m, nil
			}
			return m, m.startUpdate(ids)
		case menuSelect:
			m.enterSelection(app.StateSelectUpdate)
		case menuExclude:
			m.enterSelection(app.StateSelectExclude)
		case menuExit:
			return m.quit()
		}
	}
	return m, nil
}

func (m *Model) onSelectKey(key string) (tea.Model, tea.Cmd) {
	updatable := m.app.Updatable()
	switch key {
	case "esc":
		m.state, m.cursor, m.note = app.StateActionSelect, 0, ""
	case " ", "space":
		if m.cursor < len(updatable) {
			id := updatable[m.cursor].ID
			m.picked[id] = !m.picked[id]
		}
	case "enter":
		if len(updatable) == 0 {
			m.state, m.cursor = app.StateActionSelect, 0
			return m, nil
		}
		action := app.ActionSelected
		if m.state == app.StateSelectExclude {
			action = app.ActionExclude
		}
		ids := m.app.Plan(action, m.picked)
		if len(ids) == 0 {
			m.note = m.app.T.T("app.nothing_to_update")
			return m, nil
		}
		return m, m.startUpdate(ids)
	}
	return m, nil
}

func (m *Model) onSettingsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.goTo(app.StateMainMenu)
	case "enter", " ", "space":
		var err error
		switch m.cursor {
		case 0:
			err = m.app.SetLanguage(next(languages, m.app.Cfg.Language))
		case 1:
			err = m.app.SetDefaultAction(next(actions, m.app.Cfg.DefaultAction))
		}
		if err != nil {
			m.note = err.Error()
		} else {
			m.note = m.app.T.T("settings.saved")
		}
	}
	return m, nil
}

// next returns the value after current in values, wrapping around.
func next(values []string, current string) string {
	for i, v := range values {
		if v == current {
			return values[(i+1)%len(values)]
		}
	}
	return values[0]
}

// startUpdate begins updating ids in the background.
func (m *Model) startUpdate(ids []string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelRun = cancel
	m.state, m.cursor, m.note = app.StateUpdating, 0, ""
	m.cancelled, m.quitting = false, false
	m.ids = ids
	m.byID = map[string]app.UpdateResult{}
	m.results = nil
	m.rows = make([]views.ProgressRow, len(ids))
	m.rowIndex = make(map[string]int, len(ids))
	for i, id := range ids {
		name := id
		for _, t := range m.app.Tools {
			if t.ID == id {
				name = t.Name
			}
		}
		m.rows[i] = views.ProgressRow{Name: name}
		m.rowIndex[id] = i
	}

	events := make(chan app.UpdateEvent, 16)
	m.events = events
	a := m.app
	go func() {
		a.RunUpdates(ctx, ids, events)
		close(events)
	}()
	return tea.Batch(m.spin.Tick, m.waitEvent())
}

func (m *Model) waitEvent() tea.Cmd {
	events := m.events
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return finishedMsg{}
		}
		return eventMsg(ev)
	}
}

func (m *Model) onEvent(ev app.UpdateEvent) {
	i, ok := m.rowIndex[ev.ID]
	if !ok {
		return
	}
	if ev.Result == nil {
		m.rows[i].State = views.RowRunning
		return
	}
	m.byID[ev.ID] = *ev.Result
	if ev.Result.Err != nil {
		m.rows[i].State, m.rows[i].Err = views.RowFailed, ev.Result.Err.Error()
	} else {
		m.rows[i].State = views.RowDone
	}
}

func (m *Model) onFinished() (tea.Model, tea.Cmd) {
	m.cancelRun()
	m.results = m.results[:0]
	for _, id := range m.ids {
		if r, ok := m.byID[id]; ok {
			m.results = append(m.results, r)
		}
	}
	if m.quitting {
		return m.quit()
	}
	m.state, m.cursor, m.note = app.StateFinished, 0, ""
	return m, nil
}

func (m *Model) hasFailures() bool {
	for _, r := range m.results {
		if r.Err != nil {
			return true
		}
	}
	return false
}

// View renders the current screen.
func (m *Model) View() string {
	tr := m.app.T
	var body, help string
	switch m.state {
	case app.StateMainMenu:
		items := make([]string, mainLen)
		items[mainStatus] = tr.T("menu.status")
		items[mainSettings] = tr.T("menu.settings")
		items[mainExit] = tr.T("menu.exit")
		body = views.MainMenu(items, m.cursor)
		help = tr.T("help.menu")
	case app.StateLoading:
		body = views.Loading(m.spin.View(), tr.T("app.checking"))
	case app.StateDashboard:
		body = styles.Title.Render(tr.T("menu.status")) + "\n\n" + views.Dashboard(tr, m.app.Visible())
		help = tr.T("help.dashboard")
	case app.StateActionSelect:
		items := make([]string, menuLen)
		items[menuAll] = tr.T("action.all")
		items[menuSelect] = tr.T("action.select")
		items[menuExclude] = tr.T("action.exclude")
		items[menuExit] = tr.T("menu.exit")
		body = views.Menu(tr.T("menu.title"), items, m.cursor)
		help = tr.T("help.menu")
	case app.StateSelectUpdate, app.StateSelectExclude:
		title := tr.T("select.update_title")
		if m.state == app.StateSelectExclude {
			title = tr.T("select.exclude_title")
		}
		updatable := m.app.Updatable()
		if len(updatable) == 0 {
			body = styles.Title.Render(title) + "\n\n" + tr.T("app.nothing_to_update") + "\n"
		} else {
			items := make([]views.ChecklistItem, len(updatable))
			for i, t := range updatable {
				items[i] = views.ChecklistItem{
					Label:   t.Name + "  " + styles.Muted.Render(t.InstalledVersion+" → "+t.RemoteVersion),
					Checked: m.picked[t.ID],
				}
			}
			body = views.Checklist(title, items, m.cursor)
		}
		help = tr.T("help.select")
	case app.StateUpdating:
		body = views.Progress(tr.T("app.updating"), m.rows, m.spin.View())
		help = tr.T("help.updating")
		if m.quitting {
			body += "\n" + tr.T("app.quitting") + "\n"
			help = ""
		}
	case app.StateFinished:
		body = views.Summary(tr, m.results, m.cancelled, m.app.Paths.Log)
		help = tr.T("help.finished_ok")
		if m.hasFailures() {
			help = tr.T("help.finished")
		}
	case app.StateSettings:
		rows := []views.SettingRow{
			{Label: tr.T("settings.language"), Value: tr.T("lang." + m.app.Cfg.Language)},
			{Label: tr.T("settings.default_action"), Value: tr.T("default_action." + m.app.Cfg.DefaultAction)},
		}
		body = views.Settings(tr.T("settings.title"), rows, m.cursor, "", tr.T("settings.config_hint", m.app.Paths.Config))
		help = tr.T("help.settings")
	}

	var b strings.Builder
	b.WriteString(body)
	if m.note != "" && m.state != app.StateSettings {
		b.WriteString("\n" + styles.Warn.Render(m.note) + "\n")
	}
	if m.state == app.StateSettings && m.note != "" {
		b.WriteString("\n" + styles.OK.Render(m.note) + "\n")
	}
	if help != "" {
		b.WriteString("\n" + styles.Help.Render(help) + "\n")
	}
	return styles.Frame.Render(b.String())
}
