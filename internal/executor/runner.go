package executor

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// Output runs path with args and returns stdout and stderr together. The
// process is killed when timeout expires (a timeout of zero means no limit).
func Output(ctx context.Context, path string, args []string, timeout time.Duration) ([]byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	out, err := BuildCommandContext(ctx, path, args).CombinedOutput()
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("timed out after %s", timeout)
	}
	return out, err
}

// Job is one update to run.
type Job struct {
	ID   string
	Name string
	Path string
	Args []string
}

// Result is the outcome of a Job.
type Result struct {
	Job      Job
	Err      error
	Duration time.Duration
}

// Log receives the output of the jobs. *logger.Logger satisfies it.
type Log interface {
	io.Writer
	Infof(format string, args ...any)
	Errorf(format string, args ...any)
}

// Options controls Run.
type Options struct {
	// Workers is the number of jobs run at once. Values below 1 mean 1:
	// package managers often hold global locks, so jobs run one by one.
	Workers int
	// OnStart and OnDone, when set, are called as each job begins and ends.
	// They may be called from several goroutines.
	OnStart func(Job)
	OnDone  func(Result)
}

// Run executes the jobs and returns one Result per job, in the same order.
// stdout and stderr of every job go to log, never to the terminal. stdin is
// closed so a tool that asks a question fails instead of hanging.
func Run(ctx context.Context, jobs []Job, log Log, opts Options) []Result {
	workers := opts.Workers
	if workers < 1 {
		workers = 1
	}
	results := make([]Result, len(jobs))
	queue := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				results[i] = runOne(ctx, jobs[i], log, opts)
			}
		}()
	}
	for i := range jobs {
		queue <- i
	}
	close(queue)
	wg.Wait()
	return results
}

func runOne(ctx context.Context, job Job, log Log, opts Options) Result {
	if opts.OnStart != nil {
		opts.OnStart(job)
	}
	start := time.Now()
	var err error
	if ctx.Err() != nil {
		err = ctx.Err()
	} else {
		log.Infof("updating %s: %s %v", job.ID, job.Path, job.Args)
		cmd := BuildCommandContext(ctx, job.Path, job.Args)
		cmd.Stdout = log
		cmd.Stderr = log
		err = cmd.Run()
		if err != nil {
			log.Errorf("update of %s failed: %v", job.ID, err)
		} else {
			log.Infof("update of %s finished", job.ID)
		}
	}
	res := Result{Job: job, Err: err, Duration: time.Since(start)}
	if opts.OnDone != nil {
		opts.OnDone(res)
	}
	return res
}
