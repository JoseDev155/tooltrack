package tool

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/JoseDev155/tooltrack/internal/executor"
)

// DefaultVersionTimeout limits each version command.
const DefaultVersionTimeout = 5 * time.Second

// ErrNoUpdateCommand means the tool has no update command for this system.
var ErrNoUpdateCommand = errors.New("no update command for this operating system")

// RemoteVersioner returns the latest release tag of a GitHub repository.
// *GitHubClient implements it.
type RemoteVersioner interface {
	LatestRelease(ctx context.Context, repo string) (string, error)
}

// Checker finds installed tools and compares their versions. The function
// fields exist so tests can replace the operating system; NewChecker fills
// them with the real ones.
type Checker struct {
	GOOS           string
	LookPath       func(file string) (string, error)
	Stat           func(name string) (fs.FileInfo, error)
	Getenv         func(key string) string
	HomeDir        func() (string, error)
	Output         func(ctx context.Context, path string, args []string, timeout time.Duration) ([]byte, error)
	GitHub         RemoteVersioner
	Fetch          Fetcher
	VersionTimeout time.Duration
}

// NewChecker returns a Checker for the running system. gh may be nil when
// only command-mode tools are checked.
func NewChecker(gh RemoteVersioner) *Checker {
	return &Checker{
		GOOS:           runtime.GOOS,
		LookPath:       exec.LookPath,
		Stat:           os.Stat,
		Getenv:         os.Getenv,
		HomeDir:        os.UserHomeDir,
		Output:         executor.Output,
		GitHub:         gh,
		Fetch:          NewFetcher(30 * time.Second),
		VersionTimeout: DefaultVersionTimeout,
	}
}

var envPattern = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)

// ExpandPath expands a leading ~ and %NAME% variables. %USERPROFILE% is the
// home directory; the others come from the environment, and unknown ones are
// left as written.
func (c *Checker) ExpandPath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := c.HomeDir(); err == nil {
			return filepath.Join(home, filepath.FromSlash(strings.ReplaceAll(strings.TrimLeft(p[1:], `/\`), `\`, "/")))
		}
		return p
	}
	return envPattern.ReplaceAllStringFunc(p, func(m string) string {
		name := m[1 : len(m)-1]
		if strings.EqualFold(name, "USERPROFILE") {
			if home, err := c.HomeDir(); err == nil {
				return home
			}
			return m
		}
		if v := c.Getenv(name); v != "" {
			return v
		}
		return m
	})
}

// Resolve looks for the binary on PATH and then in the fallback locations of
// the current system. On success it sets ResolvedPath and IsInstalled.
func (c *Checker) Resolve(t *Tool) bool {
	t.ResolvedPath, t.IsInstalled = "", false
	if path, err := c.LookPath(t.Binary); err == nil {
		t.ResolvedPath, t.IsInstalled = path, true
		return true
	}
	for _, fb := range t.FallbacksFor(c.GOOS) {
		expanded := c.ExpandPath(fb)
		if info, err := c.Stat(expanded); err == nil && !info.IsDir() {
			t.ResolvedPath, t.IsInstalled = expanded, true
			return true
		}
	}
	return false
}

// Check fills the runtime fields of t: whether it is installed, its local
// and latest versions, and whether an update is available. Failures are
// recorded in LastError instead of being returned, so one broken tool never
// stops the others.
func (c *Checker) Check(ctx context.Context, t *Tool) {
	t.InstalledVersion, t.RemoteVersion, t.HasUpdate, t.LastError = "", "", false, ""
	if !c.Resolve(t) {
		return
	}

	local, err := c.localVersion(ctx, t)
	if err != nil {
		t.LastError = err.Error()
		return
	}
	t.InstalledVersion = local

	remote, err := c.remoteVersion(ctx, t)
	if err != nil {
		t.LastError = err.Error()
		return
	}
	t.RemoteVersion = remote
	if remote == "" {
		return
	}
	has, err := HasUpdate(local, remote)
	if err != nil {
		t.LastError = err.Error()
		return
	}
	t.HasUpdate = has
}

// CheckAll checks every enabled tool, at most concurrency at a time.
func (c *Checker) CheckAll(ctx context.Context, tools []Tool, concurrency int) {
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i := range tools {
		if !tools[i].Enabled {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(t *Tool) {
			defer wg.Done()
			defer func() { <-sem }()
			c.Check(ctx, t)
		}(&tools[i])
	}
	wg.Wait()
}

func (c *Checker) localVersion(ctx context.Context, t *Tool) (string, error) {
	out, err := c.runCommand(ctx, t, t.VersionCommand)
	if err != nil {
		return "", fmt.Errorf("version command failed: %w", err)
	}
	return ExtractVersion(string(out), t.VersionRegex)
}

func (c *Checker) remoteVersion(ctx context.Context, t *Tool) (string, error) {
	switch t.CheckMode {
	case CheckModeGitHub:
		if c.GitHub == nil {
			return "", errors.New("GitHub client not configured")
		}
		tag, err := c.GitHub.LatestRelease(ctx, t.GitHubRepo)
		if err != nil {
			return "", err
		}
		if v, err := ParseVersion(tag); err == nil {
			return v.String(), nil
		}
		return "", fmt.Errorf("latest release tag %q has no version number", tag)
	case CheckModeURL:
		if c.Fetch == nil {
			return "", errors.New("URL fetcher not configured")
		}
		body, err := c.Fetch(ctx, t.RemoteURL)
		if err != nil {
			return "", err
		}
		return ExtractVersion(string(body), t.RemoteRegex)
	case CheckModeCommand:
		if t.RemoteCommand == "" {
			return "", nil
		}
		out, err := c.runCommand(ctx, t, t.RemoteCommand)
		if err != nil {
			return "", fmt.Errorf("remote command failed: %w", err)
		}
		expr := t.RemoteRegex
		if expr == "" {
			expr = t.VersionRegex
		}
		return ExtractVersion(string(out), expr)
	}
	return "", fmt.Errorf("unknown check_mode %q", t.CheckMode)
}

// runCommand runs a command line whose first word is normally the tool's own
// binary, which is replaced by the resolved path.
func (c *Checker) runCommand(ctx context.Context, t *Tool, line string) ([]byte, error) {
	path, args, err := c.program(t, line)
	if err != nil {
		return nil, err
	}
	return c.Output(ctx, path, args, c.VersionTimeout)
}

func (c *Checker) program(t *Tool, line string) (string, []string, error) {
	words, err := executor.SplitCommand(line)
	if err != nil {
		return "", nil, err
	}
	if len(words) == 0 {
		return "", nil, errors.New("empty command")
	}
	if words[0] == t.Binary && t.ResolvedPath != "" {
		return t.ResolvedPath, words[1:], nil
	}
	path, err := c.LookPath(words[0])
	if err != nil {
		return "", nil, fmt.Errorf("%q not found: %w", words[0], err)
	}
	return path, words[1:], nil
}

// UpdateJob builds the job that updates t on the current system. Call Check
// or Resolve first so ResolvedPath is set. It returns ErrNoUpdateCommand when
// the tool has no command for this system.
func (c *Checker) UpdateJob(t *Tool) (executor.Job, error) {
	line, ok := t.UpdateCommandFor(c.GOOS)
	if !ok {
		return executor.Job{}, fmt.Errorf("%s: %w", t.ID, ErrNoUpdateCommand)
	}
	path, args, err := c.program(t, line)
	if err != nil {
		return executor.Job{}, fmt.Errorf("%s: %w", t.ID, err)
	}
	return executor.Job{ID: t.ID, Name: t.Name, Path: path, Args: args}, nil
}
