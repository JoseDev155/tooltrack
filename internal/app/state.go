// Package app holds the application logic shared by every front end (TUI
// today, GUI later): loading, checking, choosing and running updates.
package app

import "github.com/JoseDev155/tooltrack/internal/tool"

// State is a screen of the interactive UI.
type State int

const (
	StateLoading State = iota
	StateDashboard
	StateActionSelect
	StateSelectUpdate
	StateSelectExclude
	StateUpdating
	StateFinished
	StateSettings
	// StateMainMenu is the first screen: status, preferences, exit.
	StateMainMenu
)

func (s State) String() string {
	switch s {
	case StateLoading:
		return "loading"
	case StateDashboard:
		return "dashboard"
	case StateActionSelect:
		return "action-select"
	case StateSelectUpdate:
		return "select-update"
	case StateSelectExclude:
		return "select-exclude"
	case StateUpdating:
		return "updating"
	case StateFinished:
		return "finished"
	case StateSettings:
		return "settings"
	case StateMainMenu:
		return "main-menu"
	}
	return "unknown"
}

// Status is how a tool is shown in the dashboard.
type Status int

const (
	StatusNotInstalled Status = iota
	StatusUpToDate
	StatusUpdateAvailable
	// StatusUnknown is an installed tool whose latest version could not be
	// determined (lookup error, or no way to ask for it).
	StatusUnknown
)

// StatusOf classifies a checked tool.
func StatusOf(t *tool.Tool) Status {
	switch {
	case !t.IsInstalled:
		return StatusNotInstalled
	case t.LastError != "" || t.RemoteVersion == "":
		return StatusUnknown
	case t.HasUpdate:
		return StatusUpdateAvailable
	}
	return StatusUpToDate
}

// StatusKey returns the i18n key for the status label.
func (s Status) StatusKey() string {
	switch s {
	case StatusNotInstalled:
		return "status.not_found"
	case StatusUpToDate:
		return "status.updated"
	case StatusUpdateAvailable:
		return "status.available"
	}
	return "status.unknown"
}
