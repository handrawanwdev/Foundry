package pipeline

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/agent"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// A wave runs independent blueprint areas side by side. Written areas depend
// only on discovery, so their AI calls can overlap; each answer lands alone
// under the store lock, so a failure keeps every sibling's valid output.
// Built-in areas (use cases, acceptance) share data and stay sequential, and
// an MCP host pulls one job at a time, so its areas stay sequential too.

const defaultAreaParallel = 3

var errNoWave = errors.New("tidak ada area paralel")

type waveArea struct {
	id, text string
	rejected string
}

// areaWave is the leading run of independent areas of the queue.
func areaWave(p *core.Project, steps []string) []waveArea {
	slots := 0
	for slots < len(steps) && steps[slots] == work.KindArtifacts {
		slots++
	}
	areas, texts := work.ScopeAreas(p, spec.Artifacts(p))
	var out []waveArea
	for i, id := range areas {
		a := core.AreaByID(id)
		if a == nil || a.Builtin || len(out) == slots {
			break
		}
		out = append(out, waveArea{id: id, text: texts[i], rejected: rejectionOf(p, id)})
	}
	return out
}

func (s *Service) areaLimit() int {
	if s.AreaParallel > 0 {
		return s.AreaParallel
	}
	return defaultAreaParallel
}

// startAreaWave starts the wave at the head of the queue. false means the
// head is sequential work (or the AI is an MCP host): the caller goes on.
func (s *Service) startAreaWave(steps []string, total int) (bool, error) {
	limit := s.areaLimit()
	if limit < 2 {
		return false, nil
	}
	runner, isMCP, err := s.AI.Runner()
	if err != nil || isMCP {
		return false, nil
	}
	var wave []waveArea
	p, err := s.Store.Update(func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		wave = areaWave(p, steps)
		if len(wave) < 2 {
			return errNoWave
		}
		markBlueprintQueue(p, steps, total)
		now := time.Now().UTC()
		p.ArtifactRequest = nil
		p.Job = &core.Job{Kind: work.KindArtifacts, Status: core.JobRunning, Message: "Merancang Blueprint", Started: now}
		p.Log("ui", fmt.Sprintf("Susun rancangan blueprint: %d area paralel", len(wave)))
		trackJobStart(p, work.KindArtifacts, false)
		if t := p.BlueprintProgress; t != nil {
			t.Parallel = limit
		}
		return nil
	})
	if errors.Is(err, errNoWave) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		errs := make([]error, len(wave))
		sem := make(chan struct{}, limit)
		var wg sync.WaitGroup
		for i, a := range wave {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				errs[i] = s.runArea(runner, p.Dir, a)
			}()
		}
		wg.Wait()
		s.finishAreaWave(steps, len(wave), errors.Join(errs...))
	}()
	return true, nil
}

// runArea makes one area's attempts: the first, and one repair with the
// refusal in the prompt, the same allowance a sequential job gets.
func (s *Service) runArea(runner *agent.Runner, dir string, a waveArea) error {
	kind := work.KindArtifacts
	for repaired := false; ; repaired = true {
		prompt, err := s.startArea(a)
		if err != nil {
			return err
		}
		t0 := time.Now().UTC()
		out, err := s.runAgentFor(runner, kind, a.id, dir, prompt)
		if err != nil {
			s.failArea(a.id, err)
			return fmt.Errorf("%s: %w", a.id, err)
		}
		saveAIRaw(dir, kind+"-"+a.id, out)
		s.markValidating(kind, a.id)
		err = s.landArea(a, out, t0)
		if err == nil {
			clearAIRaw(dir, kind+"-"+a.id)
			return nil
		}
		s.rejectArea(a.id, err)
		if repaired {
			s.failArea(a.id, err)
			return fmt.Errorf("%s: %w", a.id, err)
		}
		a.rejected = err.Error()
	}
}

// startArea opens the attempt and builds its prompt from the current project,
// so a repair sees whatever its siblings landed meanwhile.
func (s *Service) startArea(a waveArea) (string, error) {
	var prompt string
	_, err := s.Store.Update(func(p *core.Project) error {
		req := &core.ChangeRequest{Text: a.text, Area: a.id, Rejected: a.rejected}
		p.ArtifactRequest = req
		if t := liveRun(p); t != nil {
			beginExecution(p, a.id, false, time.Now().UTC())
		}
		prompt = work.ArtifactsPrompt(p, spec.Artifacts(p))
		p.ArtifactRequest = nil
		return nil
	})
	return prompt, err
}

func (s *Service) landArea(a waveArea, raw string, started time.Time) error {
	_, err := s.Store.Update(func(p *core.Project) error {
		if p.Job == nil || p.Job.Kind != work.KindArtifacts || p.Job.Status != core.JobRunning {
			return fmt.Errorf("%w: tidak ada job %s yang aktif", ErrWrongStage, work.KindArtifacts)
		}
		p.ArtifactRequest = &core.ChangeRequest{Text: a.text, Area: a.id, Rejected: a.rejected}
		if err := work.ApplyArtifacts(p, raw); err != nil {
			return err
		}
		stampRefs(p)
		if err := checkAreaAnswered(p, a.id); err != nil {
			return err
		}
		core.RecordJobTiming(p, work.KindArtifacts, started, time.Now().UTC())
		p.Log("agent", "Area "+a.id+" selesai")
		syncCompleted(p, a.id)
		return nil
	})
	return err
}

// checkAreaAnswered: an answer that parsed but left its own area open did not
// do the job; left unchecked the queue would ask for the same area again and
// the last area would never run.
func checkAreaAnswered(p *core.Project, id string) error {
	if id == "" {
		return nil
	}
	for _, a := range core.Coverage(p, spec.Artifacts(p)) {
		if a.ID != id {
			continue
		}
		switch a.Status {
		case core.AreaMissing:
			return fmt.Errorf("jawaban tidak menulis area %s (%s): kembalikan artefak untuk area ini, atau lewati dengan alasan", a.ID, a.Title)
		case core.AreaInvalid:
			return fmt.Errorf("jawaban belum memperbaiki area %s (%s): masih ada %d hal wajib", a.ID, a.Title, a.Errors)
		}
	}
	return nil
}

func (s *Service) rejectArea(id string, cause error) {
	s.Store.Update(func(p *core.Project) error {
		p.Log("system", fmt.Sprintf("%s ditolak: %v", id, cause))
		if t := liveRun(p); t != nil {
			t.RejectOutput(id, failureKind(cause), safeProgressError(cause), time.Now().UTC())
		}
		return nil
	})
}

func (s *Service) failArea(id string, cause error) {
	s.Store.Update(func(p *core.Project) error {
		p.Log("system", fmt.Sprintf("%s gagal: %v", id, cause))
		if t := liveRun(p); t != nil {
			t.FailSection(id, failureKind(cause), safeProgressError(cause), time.Now().UTC())
		}
		return nil
	})
}

// finishAreaWave settles the job once every area ended. A failed wave keeps
// the queue, so a retry regenerates only the areas still open.
func (s *Service) finishAreaWave(steps []string, n int, failed error) {
	if failed != nil {
		s.Store.Update(func(p *core.Project) error {
			msg := fmt.Sprintf("Sebagian area gagal dan area lain tetap tersimpan: %v", failed)
			p.Job = &core.Job{Kind: work.KindArtifacts, Status: core.JobFailed, Message: "Merancang Blueprint", Error: msg, Started: time.Now().UTC()}
			p.Log("system", msg)
			trackFail(p, work.KindArtifacts, errors.New(msg))
			return nil
		})
		return
	}
	var err error
	_, err = s.Store.Update(func(p *core.Project) error {
		started := time.Now().UTC()
		if p.Job != nil {
			started = p.Job.Started
		}
		p.Job = &core.Job{Kind: work.KindArtifacts, Status: core.JobDone, Message: "Merancang Blueprint selesai", Started: started}
		p.BlueprintQueue = steps[n:]
		return nil
	})
	if err == nil {
		if rest := steps[n:]; len(rest) > 0 {
			err = s.startBlueprintQueue(rest, 0)
		} else {
			err = s.clearBlueprintBatch()
		}
	}
	if err != nil {
		s.failJob(work.KindArtifacts, err)
	}
}
