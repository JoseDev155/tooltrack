package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/JoseDev155/tooltrack/assets"
)

// EnsureDefaults creates the data and logs directories under p and writes
// config.json and tools.json from the embedded defaults when they do not
// exist. Existing files are never overwritten.
func EnsureDefaults(p Paths) error {
	for _, dir := range []string{filepath.Dir(p.Config), filepath.Dir(p.Log)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: %w", err)
		}
	}
	if _, err := os.Stat(p.Config); errors.Is(err, fs.ErrNotExist) {
		if err := Save(p.Config, Default()); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if _, err := os.Stat(p.Tools); errors.Is(err, fs.ErrNotExist) {
		if err := os.WriteFile(p.Tools, assets.DefaultTools(), 0o644); err != nil {
			return fmt.Errorf("config: write %s: %w", p.Tools, err)
		}
	} else if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}
