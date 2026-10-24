package logger

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func newLogger(t *testing.T, maxBytes int64, backups int) (*Logger, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logs", "updater.log")
	l, err := OpenWithRotation(path, maxBytes, backups)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l, path
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestOpenCreatesDirectoryAndFile(t *testing.T) {
	l, path := newLogger(t, 1024, 1)
	if l.Path() != path {
		t.Errorf("Path() = %q", l.Path())
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file not created: %v", err)
	}
}

func TestLinesCarryTimestampAndLevel(t *testing.T) {
	l, path := newLogger(t, 1<<20, 1)
	l.now = func() time.Time { return time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC) }
	l.Infof("updating %s", "uv")
	l.Errorf("failed: %v", errors.New("boom"))

	want := "2026-10-24T09:00:00Z [INFO] updating uv\n" +
		"2026-10-24T09:00:00Z [ERROR] failed: boom\n"
	if got := read(t, path); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDefaultTimestampIsRFC3339(t *testing.T) {
	l, path := newLogger(t, 1<<20, 1)
	l.Infof("hello")
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2}) \[INFO\] hello\n$`)
	if got := read(t, path); !re.MatchString(got) {
		t.Errorf("unexpected line %q", got)
	}
}

func TestWriteActsAsRawWriter(t *testing.T) {
	l, path := newLogger(t, 1<<20, 1)
	if _, err := l.Write([]byte("raw output\n")); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "raw output\n" {
		t.Errorf("got %q", got)
	}
}

func TestAppendsToExistingFile(t *testing.T) {
	l, path := newLogger(t, 1<<20, 1)
	l.Write([]byte("first\n"))
	l.Close()

	l2, err := OpenWithRotation(path, 1<<20, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer l2.Close()
	l2.Write([]byte("second\n"))
	if got := read(t, path); got != "first\nsecond\n" {
		t.Errorf("got %q", got)
	}
}

func TestRotationKeepsNewestBackups(t *testing.T) {
	l, path := newLogger(t, 10, 2)
	for _, s := range []string{"aaaaaaaa\n", "bbbbbbbb\n", "cccccccc\n", "dddddddd\n"} {
		if _, err := l.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if got := read(t, path); got != "dddddddd\n" {
		t.Errorf("current = %q", got)
	}
	if got := read(t, path+".1"); got != "cccccccc\n" {
		t.Errorf(".1 = %q", got)
	}
	if got := read(t, path+".2"); got != "bbbbbbbb\n" {
		t.Errorf(".2 = %q", got)
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Errorf("a third backup must not exist: %v", err)
	}
}

func TestRotationWithoutBackupsTruncates(t *testing.T) {
	l, path := newLogger(t, 10, 0)
	l.Write([]byte("aaaaaaaa\n"))
	l.Write([]byte("bbbbbbbb\n"))
	if got := read(t, path); got != "bbbbbbbb\n" {
		t.Errorf("current = %q", got)
	}
	if _, err := os.Stat(path + ".1"); !os.IsNotExist(err) {
		t.Errorf("no backup expected: %v", err)
	}
}

func TestLargeSingleWriteDoesNotLoop(t *testing.T) {
	l, path := newLogger(t, 5, 1)
	big := strings.Repeat("x", 50)
	if n, err := l.Write([]byte(big)); err != nil || n != 50 {
		t.Fatalf("Write = %d, %v", n, err)
	}
	if got := read(t, path); got != big {
		t.Errorf("content lost: %q", got)
	}
}

func TestWriteAfterCloseFails(t *testing.T) {
	l, _ := newLogger(t, 100, 1)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Write([]byte("x")); !errors.Is(err, os.ErrClosed) {
		t.Errorf("err = %v, want os.ErrClosed", err)
	}
	if err := l.Close(); err != nil {
		t.Errorf("second Close = %v", err)
	}
}

func TestOpenRejectsBadSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	if _, err := OpenWithRotation(path, 0, 1); err == nil {
		t.Error("maxBytes 0 should fail")
	}
	if _, err := OpenWithRotation(path, 10, -1); err == nil {
		t.Error("negative backups should fail")
	}
}

func TestConcurrentWritesAreWholeLines(t *testing.T) {
	l, path := newLogger(t, 1<<20, 1)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				l.Infof("message %d", j)
			}
		}()
	}
	wg.Wait()
	lines := strings.Split(strings.TrimSuffix(read(t, path), "\n"), "\n")
	if len(lines) != 500 {
		t.Fatalf("got %d lines, want 500", len(lines))
	}
	for _, line := range lines {
		if !strings.Contains(line, "[INFO] message ") {
			t.Fatalf("corrupted line %q", line)
		}
	}
}
