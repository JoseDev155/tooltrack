package executor

import (
	"context"
	"reflect"
	"runtime"
	"testing"
)

func TestCommandLine(t *testing.T) {
	args := []string{"update", "-n", "base"}
	cases := []struct {
		name, goos, path string
		wantName         string
		wantArgs         []string
	}{
		{"windows bat is wrapped", "windows", `C:\conda\condabin\conda.bat`, "cmd.exe", []string{"/c", `C:\conda\condabin\conda.bat`, "update", "-n", "base"}},
		{"windows cmd is wrapped", "windows", `C:\x\tool.cmd`, "cmd.exe", []string{"/c", `C:\x\tool.cmd`, "update", "-n", "base"}},
		{"extension check ignores case", "windows", `C:\x\TOOL.BAT`, "cmd.exe", []string{"/c", `C:\x\TOOL.BAT`, "update", "-n", "base"}},
		{"windows exe is direct", "windows", `C:\x\uv.exe`, `C:\x\uv.exe`, args},
		{"linux script is direct", "linux", "/opt/tool.bat", "/opt/tool.bat", args},
		{"darwin binary is direct", "darwin", "/usr/local/bin/bun", "/usr/local/bin/bun", args},
	}
	for _, c := range cases {
		gotName, gotArgs := commandLine(c.goos, c.path, args)
		if gotName != c.wantName || !reflect.DeepEqual(gotArgs, c.wantArgs) {
			t.Errorf("%s: got %q %q, want %q %q", c.name, gotName, gotArgs, c.wantName, c.wantArgs)
		}
	}
}

func TestCommandLineDoesNotMutateInput(t *testing.T) {
	args := make([]string, 1, 8)
	args[0] = "update"
	commandLine("windows", `C:\x\tool.bat`, args)
	if got := args[:2]; got[1] != "" {
		t.Errorf("input slice was modified: %q", got)
	}
}

func TestBuildCommandUsesHostRules(t *testing.T) {
	cmd := BuildCommand("some-tool", []string{"--version"})
	want := []string{"some-tool", "--version"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("Args = %q, want %q", cmd.Args, want)
	}
	if runtime.GOOS == "windows" {
		cmd = BuildCommand(`C:\x\a.bat`, []string{"v"})
		if cmd.Args[0] != "cmd.exe" || cmd.Args[1] != "/c" {
			t.Errorf("bat not wrapped on windows: %q", cmd.Args)
		}
	}
}

func TestBuildCommandContextRuns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	out, err := BuildCommandContext(context.Background(), "sh", []string{"-c", "echo ok"}).Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "ok\n" {
		t.Errorf("output = %q", out)
	}
}

func TestBuildCommandContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := BuildCommandContext(ctx, "sh", []string{"-c", "echo ok"})
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	if err := cmd.Run(); err == nil {
		t.Error("a cancelled context should stop the command")
	}
}

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"uv self update", []string{"uv", "self", "update"}},
		{"  conda   update  -n base  ", []string{"conda", "update", "-n", "base"}},
		{`winget upgrade "My Tool" --silent`, []string{"winget", "upgrade", "My Tool", "--silent"}},
		{`run 'a b' c`, []string{"run", "a b", "c"}},
		{`C:\bin\tool.exe --flag`, []string{`C:\bin\tool.exe`, "--flag"}},
		{`x "" y`, []string{"x", "", "y"}},
		{`a"b c"d`, []string{"ab cd"}},
		{"", nil},
		{"   ", nil},
	}
	for _, c := range cases {
		got, err := SplitCommand(c.in)
		if err != nil {
			t.Errorf("SplitCommand(%q) error: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitCommand(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSplitCommandUnterminatedQuote(t *testing.T) {
	if _, err := SplitCommand(`tool "oops`); err == nil {
		t.Error("expected an error for an unterminated quote")
	}
}
