package tool

import (
	"regexp"
	"testing"
)

func mustCompile(t *testing.T, expr string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(expr)
	if err != nil {
		t.Fatalf("regexp %q: %v", expr, err)
	}
	return re
}
