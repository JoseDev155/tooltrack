// Package config loads and saves the user preferences (config.json) and
// decides where tooltrack keeps its files.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Allowed values for Config.Language and Config.DefaultAction.
const (
	LangES = "es"
	LangEN = "en"

	ActionAsk             = "ask"
	ActionUpdateAll       = "update_all"
	ActionUpdateSelected  = "update_selected"
	ActionExcludeSelected = "exclude_selected"
)

// EnvHome overrides the directory where data/ and logs/ live (portable mode).
const EnvHome = "TOOLTRACK_HOME"

// EnvGitHubToken is read when config.json has no github_token.
const EnvGitHubToken = "GITHUB_TOKEN"

// Config mirrors data/config.json.
type Config struct {
	Language            string `json:"language"`
	DefaultAction       string `json:"default_action"`
	GitHubToken         string `json:"github_token"`
	MaxConcurrentChecks int    `json:"max_concurrent_checks"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
}

// Default returns the configuration used when no file exists.
func Default() Config {
	return Config{
		Language:            LangES,
		DefaultAction:       ActionAsk,
		MaxConcurrentChecks: 5,
		TimeoutSeconds:      30,
	}
}

// Validate reports the first invalid field.
func (c Config) Validate() error {
	switch c.Language {
	case LangES, LangEN:
	default:
		return fmt.Errorf("config: invalid language %q (want %q or %q)", c.Language, LangES, LangEN)
	}
	switch c.DefaultAction {
	case ActionAsk, ActionUpdateAll, ActionUpdateSelected, ActionExcludeSelected:
	default:
		return fmt.Errorf("config: invalid default_action %q", c.DefaultAction)
	}
	if c.MaxConcurrentChecks < 1 {
		return fmt.Errorf("config: max_concurrent_checks must be at least 1, got %d", c.MaxConcurrentChecks)
	}
	if c.TimeoutSeconds < 1 {
		return fmt.Errorf("config: timeout_seconds must be at least 1, got %d", c.TimeoutSeconds)
	}
	return nil
}

// Token returns the GitHub token from the file, or from GITHUB_TOKEN when the
// file leaves it empty.
func (c Config) Token() string {
	if c.GitHubToken != "" {
		return c.GitHubToken
	}
	return os.Getenv(EnvGitHubToken)
}

// Load reads path. A missing file yields Default(); fields missing from an
// existing file keep their default value. Invalid values are an error.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Save writes the configuration to path, creating parent directories. The
// file is private to the user because it may hold a GitHub token.
func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("config: replace %s: %w", path, err)
	}
	return nil
}

// Dir returns the base directory for tooltrack files: $TOOLTRACK_HOME when
// set, otherwise "tooltrack" inside the user config directory.
func Dir() (string, error) {
	if home := os.Getenv(EnvHome); home != "" {
		return home, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: locate user config dir: %w", err)
	}
	return filepath.Join(base, "tooltrack"), nil
}

// Paths groups the files tooltrack reads and writes under a base directory.
type Paths struct {
	Dir    string
	Config string
	Tools  string
	Log    string
}

// PathsFor builds the standard layout under dir.
func PathsFor(dir string) Paths {
	return Paths{
		Dir:    dir,
		Config: filepath.Join(dir, "data", "config.json"),
		Tools:  filepath.Join(dir, "data", "tools.json"),
		Log:    filepath.Join(dir, "logs", "updater.log"),
	}
}
