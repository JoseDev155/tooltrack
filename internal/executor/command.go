// Package executor builds and runs the subprocesses used to read versions and
// update tools.
package executor

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// BuildCommand returns the command that runs resolvedPath with rawArgs. On
// Windows, .bat and .cmd scripts cannot be started directly by CreateProcess,
// so they are wrapped with cmd.exe /c.
func BuildCommand(resolvedPath string, rawArgs []string) *exec.Cmd {
	name, args := commandLine(runtime.GOOS, resolvedPath, rawArgs)
	return exec.Command(name, args...)
}

// BuildCommandContext is like BuildCommand but the process is killed when ctx
// is done.
func BuildCommandContext(ctx context.Context, resolvedPath string, rawArgs []string) *exec.Cmd {
	name, args := commandLine(runtime.GOOS, resolvedPath, rawArgs)
	return exec.CommandContext(ctx, name, args...)
}

// commandLine decides the program and arguments for goos. It is separate from
// BuildCommand so the Windows rule can be tested on any system.
func commandLine(goos, resolvedPath string, rawArgs []string) (string, []string) {
	if goos == "windows" {
		lower := strings.ToLower(resolvedPath)
		if strings.HasSuffix(lower, ".bat") || strings.HasSuffix(lower, ".cmd") {
			args := make([]string, 0, len(rawArgs)+2)
			args = append(args, "/c", resolvedPath)
			return "cmd.exe", append(args, rawArgs...)
		}
	}
	return resolvedPath, rawArgs
}

// SplitCommand splits a command line such as `conda update -n base "my env"`
// into words. Double and single quotes group words; there are no escapes, so
// Windows paths with backslashes are kept as written.
func SplitCommand(line string) ([]string, error) {
	var (
		words   []string
		cur     strings.Builder
		quote   rune
		inWord  bool
		flushed = func() {
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		}
	)
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
			inWord = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flushed()
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote in command %q", quote, line)
	}
	flushed()
	return words, nil
}
