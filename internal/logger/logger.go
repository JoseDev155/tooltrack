// Package logger writes timestamped lines to logs/updater.log and rotates the
// file by size.
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Defaults used by Open.
const (
	DefaultMaxBytes = 5 << 20
	DefaultBackups  = 3
)

// Logger is safe for concurrent use. It also implements io.Writer so a
// subprocess can send its stdout and stderr straight to the log.
type Logger struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	backups  int
	file     *os.File
	size     int64
	now      func() time.Time
}

// Open opens (creating it if needed) the log at path with the default
// rotation settings.
func Open(path string) (*Logger, error) {
	return OpenWithRotation(path, DefaultMaxBytes, DefaultBackups)
}

// OpenWithRotation rotates the file when it would grow past maxBytes, keeping
// the newest `backups` old files as path.1 (newest) ... path.N.
func OpenWithRotation(path string, maxBytes int64, backups int) (*Logger, error) {
	if maxBytes < 1 {
		return nil, fmt.Errorf("logger: maxBytes must be positive")
	}
	if backups < 0 {
		return nil, fmt.Errorf("logger: backups must not be negative")
	}
	l := &Logger{path: path, maxBytes: maxBytes, backups: backups, now: time.Now}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}
	if err := l.openFile(); err != nil {
		return nil, err
	}
	return l, nil
}

// Path returns the log file location.
func (l *Logger) Path() string { return l.path }

func (l *Logger) openFile() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("logger: %w", err)
	}
	l.file, l.size = f, info.Size()
	return nil
}

// Write appends p to the log as is, rotating first if needed.
func (l *Logger) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return 0, os.ErrClosed
	}
	if l.size > 0 && l.size+int64(len(p)) > l.maxBytes {
		if err := l.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := l.file.Write(p)
	l.size += int64(n)
	return n, err
}

func (l *Logger) rotate() error {
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	l.file = nil
	if l.backups == 0 {
		if err := os.Remove(l.path); err != nil {
			return fmt.Errorf("logger: %w", err)
		}
	} else {
		os.Remove(l.backup(l.backups))
		for i := l.backups - 1; i >= 1; i-- {
			if err := os.Rename(l.backup(i), l.backup(i+1)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("logger: %w", err)
			}
		}
		if err := os.Rename(l.path, l.backup(1)); err != nil {
			return fmt.Errorf("logger: %w", err)
		}
	}
	return l.openFile()
}

func (l *Logger) backup(i int) string { return fmt.Sprintf("%s.%d", l.path, i) }

func (l *Logger) line(level, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l, "%s [%s] %s\n", l.now().Format(time.RFC3339), level, msg)
}

// Infof writes an INFO line with a timestamp.
func (l *Logger) Infof(format string, args ...any) { l.line("INFO", format, args...) }

// Errorf writes an ERROR line with a timestamp.
func (l *Logger) Errorf(format string, args ...any) { l.line("ERROR", format, args...) }

// Close flushes and closes the file. Later writes return os.ErrClosed.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}
