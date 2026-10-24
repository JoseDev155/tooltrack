package executor

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type bufLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *bufLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *bufLog) Infof(f string, a ...any)  { fmt.Fprintf(b, "INFO "+f+"\n", a...) }
func (b *bufLog) Errorf(f string, a ...any) { fmt.Fprintf(b, "ERROR "+f+"\n", a...) }
func (b *bufLog) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
}

func sh(id, script string) Job {
	return Job{ID: id, Name: id, Path: "sh", Args: []string{"-c", script}}
}

func TestOutputCapturesStdoutAndStderr(t *testing.T) {
	skipOnWindows(t)
	out, err := Output(context.Background(), "sh", []string{"-c", "echo out; echo err >&2"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "out") || !strings.Contains(string(out), "err") {
		t.Errorf("output = %q", out)
	}
}

func TestOutputTimeout(t *testing.T) {
	skipOnWindows(t)
	start := time.Now()
	_, err := Output(context.Background(), "sh", []string{"-c", "sleep 5"}, 100*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Error("the process was not killed on timeout")
	}
}

func TestOutputMissingBinary(t *testing.T) {
	if _, err := Output(context.Background(), "definitely-not-a-real-binary-xyz", nil, time.Second); err == nil {
		t.Error("expected an error")
	}
}

func TestRunKeepsOrderAndLogsOutput(t *testing.T) {
	skipOnWindows(t)
	log := &bufLog{}
	jobs := []Job{sh("a", "echo from-a"), sh("b", "echo from-b >&2; exit 3"), sh("c", "echo from-c")}
	results := Run(context.Background(), jobs, log, Options{})

	if len(results) != 3 {
		t.Fatalf("got %d results", len(results))
	}
	for i, id := range []string{"a", "b", "c"} {
		if results[i].Job.ID != id {
			t.Errorf("result %d is for %q, want %q", i, results[i].Job.ID, id)
		}
	}
	if results[0].Err != nil || results[1].Err == nil || results[2].Err != nil {
		t.Errorf("errors = %v, %v, %v", results[0].Err, results[1].Err, results[2].Err)
	}
	out := log.String()
	for _, want := range []string{"from-a", "from-b", "from-c", "ERROR update of b failed", "INFO update of a finished"} {
		if !strings.Contains(out, want) {
			t.Errorf("log is missing %q:\n%s", want, out)
		}
	}
}

func TestRunMissingBinaryIsAnError(t *testing.T) {
	log := &bufLog{}
	res := Run(context.Background(), []Job{{ID: "x", Path: "definitely-not-a-real-binary-xyz"}}, log, Options{})
	if res[0].Err == nil {
		t.Error("expected an error")
	}
}

func TestRunIsSequentialByDefault(t *testing.T) {
	skipOnWindows(t)
	var mu sync.Mutex
	var order []string
	opts := Options{
		OnStart: func(j Job) { mu.Lock(); order = append(order, "start "+j.ID); mu.Unlock() },
		OnDone:  func(r Result) { mu.Lock(); order = append(order, "done "+r.Job.ID); mu.Unlock() },
	}
	Run(context.Background(), []Job{sh("a", "true"), sh("b", "true")}, &bufLog{}, opts)
	want := "start a,done a,start b,done b"
	if got := strings.Join(order, ","); got != want {
		t.Errorf("order = %s, want %s", got, want)
	}
}

func TestRunWithWorkersRunsAllJobs(t *testing.T) {
	skipOnWindows(t)
	var jobs []Job
	for i := 0; i < 6; i++ {
		jobs = append(jobs, sh(fmt.Sprint("j", i), "true"))
	}
	var done int
	var mu sync.Mutex
	res := Run(context.Background(), jobs, &bufLog{}, Options{Workers: 3, OnDone: func(Result) { mu.Lock(); done++; mu.Unlock() }})
	if done != 6 {
		t.Errorf("OnDone called %d times", done)
	}
	for i, r := range res {
		if r.Err != nil || r.Job.ID != jobs[i].ID {
			t.Errorf("result %d = %+v", i, r)
		}
	}
}

func TestRunStdinIsClosed(t *testing.T) {
	skipOnWindows(t)
	done := make(chan []Result, 1)
	go func() { done <- Run(context.Background(), []Job{sh("ask", "read answer")}, &bufLog{}, Options{}) }()
	select {
	case res := <-done:
		if res[0].Err == nil {
			t.Error("a tool that waits for input should fail, not succeed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("job hung waiting for input")
	}
}

func TestRunCancelledContextSkipsJobs(t *testing.T) {
	skipOnWindows(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	log := &bufLog{}
	res := Run(ctx, []Job{sh("a", "echo should-not-run")}, log, Options{})
	if res[0].Err == nil {
		t.Error("expected the context error")
	}
	if strings.Contains(log.String(), "should-not-run") {
		t.Error("the job ran despite the cancelled context")
	}
}
