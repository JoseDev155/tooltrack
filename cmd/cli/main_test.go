package main

import "testing"

func TestRunVersion(t *testing.T) {
	if err := run([]string{"--version"}); err != nil {
		t.Errorf("run(--version) = %v", err)
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	if err := run([]string{"--no-such-flag"}); err == nil {
		t.Error("expected an error for an unknown flag")
	}
}
