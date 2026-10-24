// Package tool describes the supported development tools and how to detect
// and compare their versions.
package tool

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// CheckMode selects where the latest version of a tool comes from.
type CheckMode string

const (
	// CheckModeGitHub reads the latest release tag from the GitHub API.
	CheckModeGitHub CheckMode = "github"
	// CheckModeCommand asks the tool itself (RemoteCommand/RemoteRegex).
	// Without a RemoteCommand the remote version stays unknown.
	CheckModeCommand CheckMode = "command"
)

// Tool is one entry of the catalog (assets/defaults.json and data/tools.json).
type Tool struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Binary         string    `json:"binary"`
	CheckMode      CheckMode `json:"check_mode"`
	GitHubRepo     string    `json:"github_repo,omitempty"`
	VersionCommand string    `json:"version_command"`
	VersionRegex   string    `json:"version_regex"`
	RemoteCommand  string    `json:"remote_command,omitempty"`
	RemoteRegex    string    `json:"remote_regex,omitempty"`

	// UpdateCommands maps a GOOS value ("windows", "darwin", "linux") to the
	// command line that updates the tool. The "default" key applies to any
	// system without its own entry.
	UpdateCommands map[string]string `json:"update_commands,omitempty"`

	// Fallbacks maps a GOOS value to extra install locations to try when the
	// binary is not on PATH. Entries may use ~, %USERPROFILE% and %LOCALAPPDATA%.
	Fallbacks map[string][]string `json:"fallbacks,omitempty"`

	Enabled bool `json:"enabled"`

	// Runtime fields, never written to disk.
	ResolvedPath     string `json:"-"`
	InstalledVersion string `json:"-"`
	RemoteVersion    string `json:"-"`
	HasUpdate        bool   `json:"-"`
	IsInstalled      bool   `json:"-"`
	LastError        string `json:"-"`
}

// UpdateCommandFor returns the update command line for the given GOOS value.
// ok is false when the tool has no command for that system.
func (t *Tool) UpdateCommandFor(goos string) (cmd string, ok bool) {
	if c := t.UpdateCommands[goos]; c != "" {
		return c, true
	}
	if c := t.UpdateCommands["default"]; c != "" {
		return c, true
	}
	return "", false
}

// FallbacksFor returns the extra install locations for the given GOOS value.
func (t *Tool) FallbacksFor(goos string) []string {
	return t.Fallbacks[goos]
}

// Validate reports problems that would make the tool unusable.
func (t *Tool) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("tool without id")
	}
	if t.Binary == "" {
		return fmt.Errorf("tool %q: binary is required", t.ID)
	}
	switch t.CheckMode {
	case CheckModeGitHub:
		if t.GitHubRepo == "" {
			return fmt.Errorf("tool %q: github_repo is required in github mode", t.ID)
		}
	case CheckModeCommand:
	default:
		return fmt.Errorf("tool %q: unknown check_mode %q", t.ID, t.CheckMode)
	}
	if t.VersionCommand == "" || t.VersionRegex == "" {
		return fmt.Errorf("tool %q: version_command and version_regex are required", t.ID)
	}
	if _, err := regexp.Compile(t.VersionRegex); err != nil {
		return fmt.Errorf("tool %q: invalid version_regex: %w", t.ID, err)
	}
	if t.RemoteRegex != "" {
		if _, err := regexp.Compile(t.RemoteRegex); err != nil {
			return fmt.Errorf("tool %q: invalid remote_regex: %w", t.ID, err)
		}
	}
	return nil
}

// ParseCatalog decodes and validates a tools.json document. It rejects
// duplicate ids.
func ParseCatalog(data []byte) ([]Tool, error) {
	var tools []Tool
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, fmt.Errorf("invalid tool catalog: %w", err)
	}
	seen := make(map[string]bool, len(tools))
	for i := range tools {
		if err := tools[i].Validate(); err != nil {
			return nil, err
		}
		if seen[tools[i].ID] {
			return nil, fmt.Errorf("duplicate tool id %q", tools[i].ID)
		}
		seen[tools[i].ID] = true
	}
	return tools, nil
}
