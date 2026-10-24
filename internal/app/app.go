package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/JoseDev155/tooltrack/internal/config"
	"github.com/JoseDev155/tooltrack/internal/executor"
	"github.com/JoseDev155/tooltrack/internal/i18n"
	"github.com/JoseDev155/tooltrack/internal/logger"
	"github.com/JoseDev155/tooltrack/internal/tool"
)

// Action says which tools an update run covers.
type Action int

const (
	// ActionAll updates every tool that has an update.
	ActionAll Action = iota
	// ActionSelected updates only the picked tools.
	ActionSelected
	// ActionExclude updates every tool except the picked ones.
	ActionExclude
)

// App is the loaded application. The fields are exported so front ends and
// tests can inspect and replace them.
type App struct {
	Cfg     config.Config
	Paths   config.Paths
	Tools   []tool.Tool
	T       *i18n.Translator
	Log     *logger.Logger
	Checker *tool.Checker
}

// New creates the data directories and default files on first run, then loads
// the configuration, the tool catalog, the translations and the log.
func New(paths config.Paths) (*App, error) {
	if err := config.EnsureDefaults(paths); err != nil {
		return nil, err
	}
	cfg, err := config.Load(paths.Config)
	if err != nil {
		return nil, err
	}
	tools, err := tool.LoadMerged(paths.Tools)
	if err != nil {
		return nil, err
	}
	tr, err := i18n.New(cfg.Language)
	if err != nil {
		return nil, err
	}
	log, err := logger.Open(paths.Log)
	if err != nil {
		return nil, err
	}
	gh := tool.NewGitHubClient(cfg.Token(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	a := &App{Cfg: cfg, Paths: paths, Tools: tools, T: tr, Log: log, Checker: tool.NewChecker(gh)}
	a.sortTools()
	return a, nil
}

// Close releases the log file.
func (a *App) Close() error { return a.Log.Close() }

// sortTools orders the catalog alphabetically by name, as the dashboard shows it.
func (a *App) sortTools() {
	sort.SliceStable(a.Tools, func(i, j int) bool {
		return strings.ToLower(a.Tools[i].Name) < strings.ToLower(a.Tools[j].Name)
	})
}

// Check looks up the installed and latest version of every enabled tool.
func (a *App) Check(ctx context.Context) {
	a.Checker.CheckAll(ctx, a.Tools, a.Cfg.MaxConcurrentChecks)
}

// Visible returns the enabled tools in dashboard order.
func (a *App) Visible() []*tool.Tool {
	var out []*tool.Tool
	for i := range a.Tools {
		if a.Tools[i].Enabled {
			out = append(out, &a.Tools[i])
		}
	}
	return out
}

// Updatable returns the tools that can be updated now: enabled, installed,
// with a newer version available and an update command for this system.
func (a *App) Updatable() []*tool.Tool {
	var out []*tool.Tool
	for _, t := range a.Visible() {
		if !t.IsInstalled || !t.HasUpdate {
			continue
		}
		if _, ok := t.UpdateCommandFor(a.Checker.GOOS); !ok {
			continue
		}
		out = append(out, t)
	}
	return out
}

// Plan returns the ids to update for an action. picked holds the ids the
// user ticked; only updatable tools are ever returned.
func (a *App) Plan(action Action, picked map[string]bool) []string {
	var ids []string
	for _, t := range a.Updatable() {
		switch action {
		case ActionAll:
		case ActionSelected:
			if !picked[t.ID] {
				continue
			}
		case ActionExclude:
			if picked[t.ID] {
				continue
			}
		}
		ids = append(ids, t.ID)
	}
	return ids
}

// UpdateResult is the outcome of updating one tool.
type UpdateResult struct {
	ID       string
	Name     string
	Err      error
	Duration time.Duration
}

// UpdateEvent reports progress: Result is nil when the update of ID starts.
type UpdateEvent struct {
	ID     string
	Result *UpdateResult
}

// RunUpdates updates the tools with the given ids one after another and
// returns one result per id, in order. Progress is sent to events (which may
// be nil) and the output of the tools goes to the log. A tool that cannot be
// updated on this system, or whose command cannot be built, fails without
// stopping the rest.
func (a *App) RunUpdates(ctx context.Context, ids []string, events chan<- UpdateEvent) []UpdateResult {
	send := func(ev UpdateEvent) {
		if events != nil {
			events <- ev
		}
	}
	results := make([]UpdateResult, len(ids))
	var jobs []executor.Job
	var jobIndex []int // position in ids of each job

	for i, id := range ids {
		t := a.find(id)
		if t == nil {
			results[i] = UpdateResult{ID: id, Name: id, Err: fmt.Errorf("unknown tool %q", id)}
			continue
		}
		results[i] = UpdateResult{ID: id, Name: t.Name}
		job, err := a.Checker.UpdateJob(t)
		if err != nil {
			if errors.Is(err, tool.ErrNoUpdateCommand) {
				err = errors.New(a.T.T("error.no_update_command"))
			}
			results[i].Err = err
			a.Log.Errorf("update of %s skipped: %v", id, err)
			continue
		}
		jobs = append(jobs, job)
		jobIndex = append(jobIndex, i)
	}

	// Jobs that failed to build are reported before the ones that run.
	for i, r := range results {
		if r.Err != nil {
			send(UpdateEvent{ID: ids[i]})
			r := results[i]
			send(UpdateEvent{ID: ids[i], Result: &r})
		}
	}

	done := executor.Run(ctx, jobs, a.Log, executor.Options{
		Workers: 1,
		OnStart: func(j executor.Job) { send(UpdateEvent{ID: j.ID}) },
		OnDone: func(r executor.Result) {
			ur := UpdateResult{ID: r.Job.ID, Name: r.Job.Name, Err: r.Err, Duration: r.Duration}
			send(UpdateEvent{ID: r.Job.ID, Result: &ur})
		},
	})
	for k, r := range done {
		i := jobIndex[k]
		results[i].Err = r.Err
		results[i].Duration = r.Duration
	}
	return results
}

func (a *App) find(id string) *tool.Tool {
	for i := range a.Tools {
		if a.Tools[i].ID == id {
			return &a.Tools[i]
		}
	}
	return nil
}

// SetLanguage switches the interface language and saves it.
func (a *App) SetLanguage(lang string) error {
	cfg := a.Cfg
	cfg.Language = lang
	return a.saveConfig(cfg)
}

// SetDefaultAction changes the startup action and saves it.
func (a *App) SetDefaultAction(action string) error {
	cfg := a.Cfg
	cfg.DefaultAction = action
	return a.saveConfig(cfg)
}

func (a *App) saveConfig(cfg config.Config) error {
	tr, err := i18n.New(cfg.Language)
	if err != nil {
		return err
	}
	if err := config.Save(a.Paths.Config, cfg); err != nil {
		return err
	}
	a.Cfg, a.T = cfg, tr
	return nil
}
