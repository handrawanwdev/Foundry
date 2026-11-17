package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// errStopped refuses a job start while StopJob is stopping the running ones,
// so a job chained by an answer that just landed does not start.
var errStopped = fmt.Errorf("%w: proses sedang dihentikan", context.Canceled)

// stopMessage is the error a stopped job shows.
const stopMessage = "Dihentikan pengguna. Proses AI sudah dimatikan; hasil yang sudah tersimpan tetap ada. Coba lagi untuk melanjutkan."

// stopWait bounds how long StopJob waits for the stopped jobs to return.
// Their agent processes are killed through the cancelled context, so they
// return in moments.
const stopWait = 20 * time.Second

// jobControl is the cancellable context of the running jobs. Every job
// started until a stop shares it; StopJob cancels it and the next job gets a
// new one.
type jobControl struct {
	stop    sync.Mutex // serialises StopJob calls
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	halting bool   // StopJob is stopping the jobs
	next    string // the kind a chain wanted to start meanwhile
}

func (c *jobControl) current(parent context.Context) context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx == nil {
		c.ctx, c.cancel = context.WithCancel(parent)
	}
	return c.ctx
}

// halt cancels the running jobs and refuses new ones until resume.
func (c *jobControl) halt() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.halting, c.next = true, ""
	if c.cancel != nil {
		c.cancel()
	}
}

// resume ends the stop: the next job starts on a fresh context. It returns
// the kind a chain wanted to start during the stop.
func (c *jobControl) resume() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	next := c.next
	c.ctx, c.cancel, c.halting, c.next = nil, nil, false, ""
	return next
}

func (c *jobControl) stopping() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.halting
}

// held reports whether a start of kind is refused because of a stop, and
// remembers the first such kind: it is where a retry continues.
func (c *jobControl) held(kind string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.halting && c.next == "" {
		c.next = kind
	}
	return c.halting
}

// StopJob stops the running AI job: its agent processes are killed, nothing
// it chains starts, and the job ends as a stopped failure. Answers already
// landed stay; a retry continues from there, a dismiss closes it. A job
// waiting for an MCP host is stopped the same way, and a late answer from
// the host is refused.
func (s *Service) StopJob() error {
	s.jobs.stop.Lock()
	defer s.jobs.stop.Unlock()
	p, err := s.Store.Load()
	if err != nil {
		return err
	}
	if p.Job == nil || (p.Job.Status != core.JobRunning && p.Job.Status != core.JobWaiting) {
		if liveRun(p) != nil {
			return s.closeIdleRun()
		}
		return ErrNothingToDo
	}
	s.context() // a job started before any context existed still has one to cancel
	s.jobs.halt()
	waitErr := s.waitJobs(stopWait)
	next := s.jobs.resume()
	_, err = s.Store.Update(func(p *core.Project) error {
		kind, msg := next, ""
		if p.Job != nil {
			if p.Job.Status == core.JobDone && next == "" {
				return nil // it ended on its own before the stop reached it
			}
			msg = p.Job.Message
			if kind == "" {
				kind = p.Job.Kind
			}
		}
		if kind == "" {
			return nil
		}
		if msg == "" || next != "" {
			msg = jobMessage(kind)
		}
		p.Job = &core.Job{Kind: kind, Status: core.JobFailed, Message: msg, Error: stopMessage, Started: time.Now().UTC(), Stopped: true}
		p.Log("ui", "Proses dihentikan: "+msg)
		trackFail(p, kind, fmt.Errorf("%w: %s", context.Canceled, stopMessage))
		return nil
	})
	if err != nil {
		return err
	}
	return waitErr
}

// waitJobs waits up to timeout for the job goroutines to return.
func (s *Service) waitJobs(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return errors.New("proses AI belum berhenti setelah " + timeout.String())
	}
}

// isBlueprintStep reports whether kind is a step of the blueprint queue.
func isBlueprintStep(kind string) bool {
	switch kind {
	case work.KindArtifacts, work.KindDesign, work.KindPlan:
		return true
	}
	return false
}

// closeIdleRun ends a blueprint run that still shows work while no job runs:
// nothing is left to kill, so the run is closed the way it would have been.
func (s *Service) closeIdleRun() error {
	_, err := s.Store.Update(func(p *core.Project) error {
		t := liveRun(p)
		if t == nil || p.Job != nil && (p.Job.Status == core.JobRunning || p.Job.Status == core.JobWaiting) {
			return ErrNothingToDo
		}
		if p.Job != nil && p.Job.Status == core.JobFailed {
			t.FailRun(p.Job.Error, time.Now().UTC())
			return nil
		}
		p.BlueprintQueue, p.BlueprintBatch = nil, 0
		finishRunIfDone(p)
		p.Log("ui", "Perancangan Blueprint ditutup: tidak ada proses AI yang berjalan")
		return nil
	})
	return err
}
