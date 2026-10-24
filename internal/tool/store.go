package tool

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/JoseDev155/tooltrack/assets"
)

// Load reads and validates the catalog at path.
func Load(path string) ([]Tool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tool catalog: %w", err)
	}
	return ParseCatalog(data)
}

// Defaults returns the catalog embedded in the binary.
func Defaults() ([]Tool, error) {
	return ParseCatalog(assets.DefaultTools())
}

// Merge combines the user catalog with the embedded one. The user's entries
// keep their order and win over the defaults with the same id; defaults the
// user does not have yet (for example presets added in a newer release) are
// appended.
func Merge(user, defaults []Tool) []Tool {
	merged := make([]Tool, 0, len(user)+len(defaults))
	seen := make(map[string]bool, len(user))
	for _, t := range user {
		merged = append(merged, migrate(t, defaults))
		seen[t.ID] = true
	}
	for _, t := range defaults {
		if !seen[t.ID] {
			merged = append(merged, t)
		}
	}
	return merged
}

// migrate upgrades catalogs written by earlier releases. Rustup used to be
// checked through GitHub releases, which it does not publish, so an untouched
// entry of that kind takes the version source of the current preset. Entries
// the user changed are left alone.
func migrate(t Tool, defaults []Tool) Tool {
	if t.ID != "rustup" || t.CheckMode != CheckModeGitHub || t.GitHubRepo != "rust-lang/rustup" {
		return t
	}
	for _, d := range defaults {
		if d.ID == t.ID && d.CheckMode == CheckModeURL {
			t.CheckMode, t.GitHubRepo = d.CheckMode, ""
			t.RemoteURL, t.RemoteRegex = d.RemoteURL, d.RemoteRegex
		}
	}
	return t
}

// LoadMerged returns the user's catalog at path merged with the defaults. A
// missing file yields just the defaults. Nothing is written to disk; call
// Save to persist the result.
func LoadMerged(path string) ([]Tool, error) {
	defaults, err := Defaults()
	if err != nil {
		return nil, err
	}
	user, err := Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return nil, err
	}
	return Merge(user, defaults), nil
}

// Save writes the catalog to path atomically. Runtime fields are not saved.
func Save(path string, tools []Tool) error {
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("tool catalog: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("tool catalog: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("tool catalog: %w", err)
	}
	return nil
}
