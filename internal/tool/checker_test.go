package tool

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeInfo struct{ dir bool }

func (fakeInfo) Name() string       { return "f" }
func (fakeInfo) Size() int64        { return 0 }
func (fakeInfo) Mode() fs.FileMode  { return 0 }
func (fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool      { return i.dir }
func (fakeInfo) Sys() any           { return nil }

type fakeGitHub struct {
	tag   string
	err   error
	calls atomic.Int32
}

func (f *fakeGitHub) LatestRelease(ctx context.Context, repo string) (string, error) {
	f.calls.Add(1)
	return f.tag, f.err
}

// newChecker returns a Checker on a fake Linux system with the given tools on
// PATH, where running a path answers with outputs[path].
func newChecker(onPath map[string]string, outputs map[string]string, gh RemoteVersioner) *Checker {
	return &Checker{
		GOOS: "linux",
		LookPath: func(file string) (string, error) {
			if p, ok := onPath[file]; ok {
				return p, nil
			}
			return "", errors.New("not found")
		},
		Stat:    func(string) (fs.FileInfo, error) { return nil, os.ErrNotExist },
		Getenv:  func(k string) string { return map[string]string{"LOCALAPPDATA": `C:\Users\u\AppData\Local`}[k] },
		HomeDir: func() (string, error) { return "/home/u", nil },
		Output: func(ctx context.Context, path string, args []string, timeout time.Duration) ([]byte, error) {
			out, ok := outputs[path+" "+strings.Join(args, " ")]
			if !ok {
				return nil, errors.New("exit status 1")
			}
			return []byte(out), nil
		},
		GitHub:         gh,
		VersionTimeout: time.Second,
	}
}

func uvTool() Tool {
	return Tool{
		ID: "uv", Name: "Astral uv", Binary: "uv", CheckMode: CheckModeGitHub,
		GitHubRepo: "astral-sh/uv", VersionCommand: "uv --version",
		VersionRegex: `uv\s+(\d+\.\d+\.\d+)`, Enabled: true,
	}
}

func TestCheckUpdateAvailable(t *testing.T) {
	gh := &fakeGitHub{tag: "0.5.2"}
	c := newChecker(map[string]string{"uv": "/bin/uv"}, map[string]string{"/bin/uv --version": "uv 0.5.1 (abc)"}, gh)
	tl := uvTool()
	c.Check(context.Background(), &tl)

	if !tl.IsInstalled || tl.ResolvedPath != "/bin/uv" || tl.InstalledVersion != "0.5.1" ||
		tl.RemoteVersion != "0.5.2" || !tl.HasUpdate || tl.LastError != "" {
		t.Errorf("unexpected state: %+v", tl)
	}
}

func TestCheckUpToDate(t *testing.T) {
	gh := &fakeGitHub{tag: "v0.5.1"}
	c := newChecker(map[string]string{"uv": "/bin/uv"}, map[string]string{"/bin/uv --version": "uv 0.5.1"}, gh)
	tl := uvTool()
	c.Check(context.Background(), &tl)
	if tl.HasUpdate || tl.RemoteVersion != "0.5.1" || tl.LastError != "" {
		t.Errorf("unexpected state: %+v", tl)
	}
}

func TestCheckLocalNewerThanRemoteIsNotAnUpdate(t *testing.T) {
	gh := &fakeGitHub{tag: "1.5.0"}
	c := newChecker(map[string]string{"uv": "/bin/uv"}, map[string]string{"/bin/uv --version": "uv 2.0.0"}, gh)
	tl := uvTool()
	c.Check(context.Background(), &tl)
	if tl.HasUpdate {
		t.Errorf("2.0.0 installed must not update to 1.5.0: %+v", tl)
	}
}

func TestCheckPrefixedTag(t *testing.T) {
	gh := &fakeGitHub{tag: "bun-v1.1.31"}
	c := newChecker(map[string]string{"bun": "/bin/bun"}, map[string]string{"/bin/bun --version": "1.1.30"}, gh)
	tl := Tool{ID: "bun", Binary: "bun", CheckMode: CheckModeGitHub, GitHubRepo: "oven-sh/bun",
		VersionCommand: "bun --version", VersionRegex: `(\d+\.\d+\.\d+)`}
	c.Check(context.Background(), &tl)
	if !tl.HasUpdate || tl.RemoteVersion != "1.1.31" {
		t.Errorf("unexpected state: %+v", tl)
	}
}

func TestCheckNotInstalled(t *testing.T) {
	gh := &fakeGitHub{tag: "1.0.0"}
	c := newChecker(nil, nil, gh)
	tl := uvTool()
	tl.InstalledVersion, tl.HasUpdate = "stale", true // leftovers of a previous run
	c.Check(context.Background(), &tl)

	if tl.IsInstalled || tl.HasUpdate || tl.InstalledVersion != "" || tl.LastError != "" {
		t.Errorf("unexpected state: %+v", tl)
	}
	if gh.calls.Load() != 0 {
		t.Error("GitHub must not be queried for a tool that is not installed")
	}
}

func TestCheckVersionCommandFails(t *testing.T) {
	c := newChecker(map[string]string{"uv": "/bin/uv"}, nil, &fakeGitHub{tag: "1.0.0"})
	tl := uvTool()
	c.Check(context.Background(), &tl)
	if !tl.IsInstalled || tl.LastError == "" || !strings.Contains(tl.LastError, "version command failed") {
		t.Errorf("unexpected state: %+v", tl)
	}
}

func TestCheckUnparseableOutput(t *testing.T) {
	c := newChecker(map[string]string{"uv": "/bin/uv"}, map[string]string{"/bin/uv --version": "garbage"}, &fakeGitHub{tag: "1.0.0"})
	tl := uvTool()
	c.Check(context.Background(), &tl)
	if tl.LastError == "" || tl.InstalledVersion != "" {
		t.Errorf("unexpected state: %+v", tl)
	}
}

func TestCheckGitHubFailureKeepsLocalVersion(t *testing.T) {
	gh := &fakeGitHub{err: errors.New("rate limit exceeded")}
	c := newChecker(map[string]string{"uv": "/bin/uv"}, map[string]string{"/bin/uv --version": "uv 0.5.1"}, gh)
	tl := uvTool()
	c.Check(context.Background(), &tl)
	if tl.InstalledVersion != "0.5.1" || tl.LastError != "rate limit exceeded" || tl.HasUpdate {
		t.Errorf("unexpected state: %+v", tl)
	}
}

func TestCheckWithoutGitHubClient(t *testing.T) {
	c := newChecker(map[string]string{"uv": "/bin/uv"}, map[string]string{"/bin/uv --version": "uv 0.5.1"}, nil)
	tl := uvTool()
	c.Check(context.Background(), &tl)
	if !strings.Contains(tl.LastError, "GitHub client not configured") {
		t.Errorf("LastError = %q", tl.LastError)
	}
}

func TestCheckBadTag(t *testing.T) {
	c := newChecker(map[string]string{"uv": "/bin/uv"}, map[string]string{"/bin/uv --version": "uv 0.5.1"}, &fakeGitHub{tag: "nightly"})
	tl := uvTool()
	c.Check(context.Background(), &tl)
	if !strings.Contains(tl.LastError, "no version number") {
		t.Errorf("LastError = %q", tl.LastError)
	}
}

func TestCheckCommandModeWithoutRemote(t *testing.T) {
	c := newChecker(map[string]string{"conda": "/opt/conda"}, map[string]string{"/opt/conda --version": "conda 24.9.2"}, nil)
	tl := Tool{ID: "anaconda", Binary: "conda", CheckMode: CheckModeCommand,
		VersionCommand: "conda --version", VersionRegex: `conda\s+(\d+\.\d+\.\d+)`}
	c.Check(context.Background(), &tl)
	if tl.InstalledVersion != "24.9.2" || tl.RemoteVersion != "" || tl.HasUpdate || tl.LastError != "" {
		t.Errorf("unexpected state: %+v", tl)
	}
}

func TestCheckCommandModeWithRemoteCommand(t *testing.T) {
	c := newChecker(map[string]string{"conda": "/opt/conda"}, map[string]string{
		"/opt/conda --version":    "conda 24.9.2",
		"/opt/conda search conda": "conda  24.11.0  py312_0",
	}, nil)
	tl := Tool{ID: "anaconda", Binary: "conda", CheckMode: CheckModeCommand,
		VersionCommand: "conda --version", VersionRegex: `conda\s+(\d+\.\d+\.\d+)`,
		RemoteCommand: "conda search conda", RemoteRegex: `conda\s+(\d+\.\d+\.\d+)`}
	c.Check(context.Background(), &tl)
	if tl.RemoteVersion != "24.11.0" || !tl.HasUpdate || tl.LastError != "" {
		t.Errorf("unexpected state: %+v", tl)
	}
}

func TestResolvePrefersPath(t *testing.T) {
	c := newChecker(map[string]string{"conda": "/usr/bin/conda"}, nil, nil)
	c.Stat = func(string) (fs.FileInfo, error) { t.Error("fallbacks must not be tried"); return nil, os.ErrNotExist }
	tl := Tool{Binary: "conda", Fallbacks: map[string][]string{"linux": {"~/x"}}}
	if !c.Resolve(&tl) || tl.ResolvedPath != "/usr/bin/conda" {
		t.Errorf("unexpected state: %+v", tl)
	}
}

func TestResolveWindowsFallbacks(t *testing.T) {
	c := newChecker(nil, nil, nil)
	c.GOOS = "windows"
	var tried []string
	c.Stat = func(name string) (fs.FileInfo, error) {
		tried = append(tried, name)
		switch name {
		case `/home/u\miniconda3\condabin\conda.bat`:
			return fakeInfo{}, nil
		case `/home/u\anaconda3\condabin\conda.bat`:
			return fakeInfo{dir: true}, nil // a directory must be skipped
		}
		return nil, os.ErrNotExist
	}
	tl := Tool{Binary: "conda", Fallbacks: map[string][]string{"windows": {
		`%USERPROFILE%\anaconda3\condabin\conda.bat`,
		`%USERPROFILE%\miniconda3\condabin\conda.bat`,
		`C:\ProgramData\anaconda3\condabin\conda.bat`,
	}}}
	if !c.Resolve(&tl) {
		t.Fatalf("not resolved; tried %q", tried)
	}
	if tl.ResolvedPath != `/home/u\miniconda3\condabin\conda.bat` || !tl.IsInstalled {
		t.Errorf("unexpected state: %+v", tl)
	}
	if len(tried) != 2 {
		t.Errorf("tried %q, want to stop at the first match", tried)
	}
}

func TestResolveFallbacksOnlyForCurrentSystem(t *testing.T) {
	c := newChecker(nil, nil, nil)
	c.Stat = func(string) (fs.FileInfo, error) { return fakeInfo{}, nil }
	tl := Tool{Binary: "conda", Fallbacks: map[string][]string{"windows": {`C:\conda.bat`}}}
	if c.Resolve(&tl) {
		t.Error("windows fallbacks must not apply on linux")
	}
}

func TestResolveLinuxHomeFallback(t *testing.T) {
	c := newChecker(nil, nil, nil)
	want := filepath.Join("/home/u", "miniconda3", "bin", "conda") // ExpandPath joins with the host separator
	c.Stat = func(name string) (fs.FileInfo, error) {
		if name == want {
			return fakeInfo{}, nil
		}
		return nil, os.ErrNotExist
	}
	tl := Tool{Binary: "conda", Fallbacks: map[string][]string{"linux": {"~/anaconda3/bin/conda", "~/miniconda3/bin/conda"}}}
	if !c.Resolve(&tl) || tl.ResolvedPath != want {
		t.Errorf("unexpected state: %+v", tl)
	}
}

func TestExpandPath(t *testing.T) {
	c := newChecker(nil, nil, nil)
	cases := map[string]string{
		`%USERPROFILE%\a.bat`:       `/home/u\a.bat`,
		`%userprofile%\a.bat`:       `/home/u\a.bat`,
		`%LOCALAPPDATA%\Programs\x`: `C:\Users\u\AppData\Local\Programs\x`,
		`%UNKNOWN_VAR%\x`:           `%UNKNOWN_VAR%\x`,
		`C:\ProgramData\x`:          `C:\ProgramData\x`,
		"~/bin/tool":                filepath.Join("/home/u", "bin", "tool"),
		"~":                         filepath.Join("/home/u"),
		"/abs/path":                 "/abs/path",
	}
	for in, want := range cases {
		if got := c.ExpandPath(in); got != want {
			t.Errorf("ExpandPath(%q) = %q, want %q", in, got, want)
		}
	}

	c.HomeDir = func() (string, error) { return "", errors.New("no home") }
	if got := c.ExpandPath("~/x"); got != "~/x" {
		t.Errorf("without a home directory the path is kept, got %q", got)
	}
}

func TestCheckAllSkipsDisabledAndLimitsConcurrency(t *testing.T) {
	var running, peak atomic.Int32
	var mu sync.Mutex
	c := newChecker(map[string]string{"t": "/bin/t"}, nil, nil)
	c.Output = func(ctx context.Context, path string, args []string, timeout time.Duration) ([]byte, error) {
		n := running.Add(1)
		mu.Lock()
		if n > peak.Load() {
			peak.Store(n)
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		running.Add(-1)
		return []byte("t 1.0.0"), nil
	}
	mk := func(enabled bool) Tool {
		return Tool{ID: "t", Binary: "t", CheckMode: CheckModeCommand,
			VersionCommand: "t --version", VersionRegex: `t\s+(\d+\.\d+\.\d+)`, Enabled: enabled}
	}
	tools := []Tool{mk(true), mk(true), mk(true), mk(true), mk(true), mk(false)}
	c.CheckAll(context.Background(), tools, 2)

	for i := 0; i < 5; i++ {
		if tools[i].InstalledVersion != "1.0.0" {
			t.Errorf("tool %d not checked: %+v", i, tools[i])
		}
	}
	if tools[5].IsInstalled || tools[5].InstalledVersion != "" {
		t.Errorf("disabled tool was checked: %+v", tools[5])
	}
	if peak.Load() > 2 {
		t.Errorf("peak concurrency %d, limit 2", peak.Load())
	}
}

func TestUpdateJob(t *testing.T) {
	c := newChecker(map[string]string{"cargo": "/bin/cargo"}, nil, nil)
	tl := Tool{ID: "fnm", Name: "Fast Node Manager", Binary: "fnm", ResolvedPath: "/bin/fnm",
		UpdateCommands: map[string]string{"default": "cargo install fnm --force", "darwin": "brew upgrade fnm"}}
	job, err := c.UpdateJob(&tl)
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != "fnm" || job.Path != "/bin/cargo" || strings.Join(job.Args, " ") != "install fnm --force" {
		t.Errorf("unexpected job: %+v", job)
	}
}

func TestUpdateJobUsesResolvedPathOfTheTool(t *testing.T) {
	c := newChecker(nil, nil, nil) // the binary is NOT on PATH
	tl := Tool{ID: "anaconda", Binary: "conda", ResolvedPath: "/home/u/miniconda3/bin/conda",
		UpdateCommands: map[string]string{"default": "conda update -n base -c defaults conda -y"}}
	job, err := c.UpdateJob(&tl)
	if err != nil {
		t.Fatal(err)
	}
	if job.Path != "/home/u/miniconda3/bin/conda" || len(job.Args) != 7 {
		t.Errorf("unexpected job: %+v", job)
	}
}

func TestUpdateJobErrors(t *testing.T) {
	c := newChecker(nil, nil, nil)
	gh := Tool{ID: "gh", Binary: "gh", UpdateCommands: map[string]string{"darwin": "brew upgrade gh"}}
	if _, err := c.UpdateJob(&gh); !errors.Is(err, ErrNoUpdateCommand) {
		t.Errorf("err = %v, want ErrNoUpdateCommand", err)
	}

	missing := Tool{ID: "fnm", Binary: "fnm", UpdateCommands: map[string]string{"default": "cargo install fnm"}}
	if _, err := c.UpdateJob(&missing); err == nil || !strings.Contains(err.Error(), `"cargo" not found`) {
		t.Errorf("err = %v", err)
	}

	bad := Tool{ID: "x", Binary: "x", UpdateCommands: map[string]string{"default": `x "unterminated`}}
	if _, err := c.UpdateJob(&bad); err == nil {
		t.Error("expected an error for an unterminated quote")
	}
}

func rustupTool() Tool {
	return Tool{
		ID: "rustup", Name: "Rustup", Binary: "rustup", CheckMode: CheckModeURL,
		RemoteURL:      "https://static.example.test/rustup/release-stable.toml",
		RemoteRegex:    `version\s*=\s*'(\d+\.\d+\.\d+)'`,
		VersionCommand: "rustup --version", VersionRegex: `rustup\s+(\d+\.\d+\.\d+)`, Enabled: true,
	}
}

func TestCheckURLMode(t *testing.T) {
	c := newChecker(map[string]string{"rustup": "/usr/bin/rustup"}, map[string]string{"/usr/bin/rustup --version": "rustup 1.28.0 (abc 2025-01-01)"}, nil)
	var asked string
	c.Fetch = func(ctx context.Context, url string) ([]byte, error) {
		asked = url
		return []byte("schema-version = '1'\nversion = '1.29.2'\n"), nil
	}
	tl := rustupTool()
	c.Check(context.Background(), &tl)
	if asked != tl.RemoteURL {
		t.Errorf("fetched %q, want %q", asked, tl.RemoteURL)
	}
	if tl.RemoteVersion != "1.29.2" || !tl.HasUpdate || tl.LastError != "" {
		t.Errorf("unexpected state: %+v", tl)
	}
}

func TestCheckURLModeErrors(t *testing.T) {
	c := newChecker(map[string]string{"rustup": "/usr/bin/rustup"}, map[string]string{"/usr/bin/rustup --version": "rustup 1.29.2"}, nil)
	tl := rustupTool()
	c.Check(context.Background(), &tl)
	if tl.LastError == "" || tl.InstalledVersion != "1.29.2" {
		t.Errorf("without a fetcher: %+v", tl)
	}
	c.Fetch = func(context.Context, string) ([]byte, error) { return nil, errors.New("boom") }
	tl = rustupTool()
	c.Check(context.Background(), &tl)
	if tl.LastError == "" || tl.RemoteVersion != "" || tl.InstalledVersion != "1.29.2" {
		t.Errorf("fetch failure: %+v", tl)
	}
	c.Fetch = func(context.Context, string) ([]byte, error) { return []byte("<html>no version</html>"), nil }
	tl = rustupTool()
	c.Check(context.Background(), &tl)
	if tl.LastError == "" || tl.HasUpdate {
		t.Errorf("unparseable body: %+v", tl)
	}
}
