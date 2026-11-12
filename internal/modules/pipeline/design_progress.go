package pipeline

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/agent"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

const (
	secDiscovery = "discovery"
	secTech      = "tech"
	secArch      = "arch"
	secRepo      = "repo"
)

const (
	reasonAfterDiscovery = "Relevansinya ditentukan setelah Penjelasan Sistem selesai"
	reasonSatisfied      = "Sudah ada dari proses sebelumnya dan masih valid"
	reasonStopped        = "Ditunda: proses ditutup sebelum bagian ini dikerjakan. Jalankan Susun rancangan untuk melanjutkan"
)

// tracksBlueprint: the job belongs to a blueprint run. An ended run only
// reopens for its own unfinished work, never for an unrelated revision.
func tracksBlueprint(p *core.Project, kind string) bool {
	switch kind {
	case work.KindDiscovery, work.KindArtifacts, work.KindDesign, work.KindPlan, work.KindTech, work.KindStructure, work.KindRepo:
	default:
		return false
	}
	if t := p.BlueprintProgress; t != nil {
		switch t.Status {
		case core.BPRunRunning:
			return true
		case core.BPRunFailed, core.BPRunIncomplete, core.BPRunStopped:
			if s := t.Section(sectionForJob(p, kind)); s != nil && s.Status != core.BPCompleted {
				return true
			}
		}
	}
	if p.BlueprintBatch > 0 {
		return true
	}
	return kind == work.KindDiscovery && p.BlueprintStarted != nil
}

func liveRun(p *core.Project) *core.BlueprintProgress {
	if t := p.BlueprintProgress; t != nil && t.Status == core.BPRunRunning {
		return t
	}
	return nil
}

func ensureBlueprintProgress(p *core.Project) {
	now := time.Now().UTC()
	t := p.BlueprintProgress
	if t == nil || t.Status == core.BPRunCompleted {
		startRun(p, now)
		return
	}
	t.ResumeRun(now)
	t.ReconcilePlan(planSections(p), now)
}

// startRun opens a new run. What already holds is not this run's work, so it
// is outside the denominator as already satisfied instead of a head start.
func startRun(p *core.Project, now time.Time) {
	started := now
	if p.BlueprintStarted != nil {
		started = *p.BlueprintStarted
	}
	var base int64
	if p.BlueprintProgress != nil {
		base = p.BlueprintProgress.Seq
	}
	items := planSections(p)
	for i := range items {
		if items[i].Status == core.BPCompleted {
			items[i].Status, items[i].SkipKind, items[i].Reason = core.BPSkipped, core.BPSkipAlreadySatisfied, reasonSatisfied
		}
	}
	p.BlueprintProgress = core.NewBlueprintProgress(fmt.Sprintf("run-%d-%d", now.Unix(), base+1), items, base, started)
}

// planSections is the full plan from project truth. Every section of the
// blueprint is always listed in execution order, with why it does not apply
// when it does not, so the list and the numbers can never disagree.
func planSections(p *core.Project) []core.BlueprintSection {
	decided := p.Discovery != nil
	coverage := core.Coverage(p, spec.Artifacts(p))
	out := []core.BlueprintSection{{ID: secDiscovery, Name: "Penjelasan Sistem", Kind: work.KindDiscovery, Status: statusOf(decided)}}

	var ui *core.BlueprintSection
	for _, a := range coverage {
		kind := work.KindArtifacts
		if a.ID == core.AreaUI {
			kind = work.KindDesign
		}
		s := core.BlueprintSection{ID: a.ID, Name: a.Title, Kind: kind, Status: core.BPPending}
		switch a.Status {
		case core.AreaNotApplicable:
			if decided || !decidedByDiscovery(a.ID) {
				s.Status, s.SkipKind, s.Reason = core.BPSkipped, core.BPSkipNotApplicable, capitalize(a.Reason)
			} else {
				s.Reason = reasonAfterDiscovery
			}
		case core.AreaSkipped:
			s.Status, s.SkipKind, s.Reason = core.BPSkipped, core.BPSkipNotRequired, a.Reason
		case core.AreaCovered:
			s.Status = core.BPCompleted
		}
		if a.ID == core.AreaUI {
			if s.Status == core.BPCompleted && !uiLanded(p) {
				s.Status = core.BPPending
			}
			ui = &s
			continue
		}
		out = append(out, s)
	}
	if ui != nil {
		out = append(out, *ui)
	}

	planPart := func(id, name, part string) core.BlueprintSection {
		s := core.BlueprintSection{ID: id, Name: name, Kind: work.KindPlan, Status: core.BPPending}
		existing := p.Structure != nil && p.Structure.Mode == core.StructureExisting
		switch {
		case !decided:
			s.Reason = reasonAfterDiscovery
		case !work.AppLike(p):
			s.Status, s.SkipKind, s.Reason = core.BPSkipped, core.BPSkipNotApplicable, "Produk ini tidak membutuhkan rancangan aplikasi"
		case existing && part != work.KindTech:
			s.Status, s.SkipKind, s.Reason = core.BPSkipped, core.BPSkipNotApplicable, "Project yang sudah ada memakai struktur repository-nya sendiri"
		case planPartLanded(p, part):
			s.Status = core.BPCompleted
		}
		return s
	}
	out = append(out, planPart(secTech, "Teknologi", work.KindTech),
		planPart(secArch, "Arsitektur", work.KindStructure),
		planPart(secRepo, "Struktur Repository", work.KindRepo))
	linkDependencies(out)
	return out
}

// linkDependencies mirrors what each job really reads: areas read discovery,
// the design reads the areas, the plan reads everything before it, and its
// rounds run in order. Sections left out are not waited for.
func linkDependencies(sections []core.BlueprintSection) {
	var areas, soFar []string
	need := func(ids []string) []string {
		var out []string
		for _, id := range ids {
			for i := range sections {
				if sections[i].ID == id && sections[i].Status != core.BPSkipped {
					out = append(out, id)
				}
			}
		}
		return out
	}
	for i := range sections {
		s := &sections[i]
		switch s.ID {
		case secDiscovery:
		case core.AreaUI:
			s.DependsOn = need(areas)
			if len(s.DependsOn) == 0 {
				s.DependsOn = need([]string{secDiscovery})
			}
		case secTech:
			s.DependsOn = need(soFar)
		case secArch:
			s.DependsOn = need([]string{secTech})
		case secRepo:
			s.DependsOn = need([]string{secArch})
		default:
			s.DependsOn = need([]string{secDiscovery})
			areas = append(areas, s.ID)
		}
		soFar = append(soFar, s.ID)
	}
}

func statusOf(landed bool) string {
	if landed {
		return core.BPCompleted
	}
	return core.BPPending
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Before discovery these areas cannot be judged: the data they look at does
// not exist yet, so "not applicable" would be a guess.
func decidedByDiscovery(id string) bool {
	switch id {
	case core.AreaUI, core.AreaUseCases, core.AreaData, core.AreaIntegration, core.AreaAcceptance:
		return true
	}
	return false
}

func uiLanded(p *core.Project) bool {
	return p.UI != nil && p.UI.Written() && p.UI.Request == nil && !work.DesignNeedsDetails(p)
}

// planPartLanded: no work is open or due on this part. While plan rounds are
// open a part outside them counts as landed: its stale mark clears when the
// last round syncs the sources.
func planPartLanded(p *core.Project, part string) bool {
	switch part {
	case work.KindTech:
		if p.Stack != nil && p.Stack.PlanRequest != nil {
			return false
		}
	case work.KindStructure:
		if p.Structure != nil && p.Structure.PlanRequest != nil {
			return false
		}
	case work.KindRepo:
		if p.Structure != nil && p.Structure.RepoRequest != nil {
			return false
		}
	}
	return planRequestsOpen(p) || !slices.Contains(unifiedParts(p), part)
}

func sectionForJob(p *core.Project, kind string) string {
	switch kind {
	case work.KindDiscovery:
		return secDiscovery
	case work.KindDesign:
		return core.AreaUI
	case work.KindArtifacts:
		if p.ArtifactRequest != nil {
			return p.ArtifactRequest.Area
		}
	case work.KindTech:
		return secTech
	case work.KindStructure:
		return secArch
	case work.KindRepo:
		return secRepo
	case work.KindPlan:
		switch work.CurrentPlanPart(p) {
		case work.KindTech:
			return secTech
		case work.KindStructure:
			return secArch
		case work.KindRepo:
			return secRepo
		}
	}
	return ""
}

func generatingActivity(id string) string {
	switch id {
	case secDiscovery:
		return "Membaca kebutuhan sistem dan mengidentifikasi aktor"
	case core.AreaUseCases:
		return "Mengidentifikasi aktor, alur, dan hak akses"
	case core.AreaData:
		return "Menganalisis kepemilikan data dan sumber kebenaran"
	case core.AreaSecurity:
		return "Menganalisis ancaman dan model keamanan"
	case core.AreaIntegration:
		return "Menganalisis integrasi dan dependensi eksternal"
	case core.AreaOperations:
		return "Menganalisis kebutuhan observability dan reliability"
	case core.AreaMigration:
		return "Menganalisis migrasi dan kompatibilitas"
	case core.AreaAcceptance:
		return "Menyusun acceptance criteria dan definition of done"
	case core.AreaUI:
		return "Menyusun aturan desain dan spesifikasi layout"
	case secTech:
		return "Merancang teknologi"
	case secArch:
		return "Menganalisis arsitektur aplikasi"
	case secRepo:
		return "Merancang struktur repository"
	}
	if a := core.AreaByID(id); a != nil {
		return "Menyusun " + a.Title
	}
	return "Menghasilkan output blueprint"
}

// rejectionOf is why the section's last answer was refused, "" when it was
// not, from the open request or from the run when the request was closed.
func rejectionOf(p *core.Project, id string) string {
	switch id {
	case core.AreaUI:
		if p.UI != nil && p.UI.Request != nil && p.UI.Request.Rejected != "" {
			return p.UI.Request.Rejected
		}
	case secTech:
		if p.Stack != nil && p.Stack.PlanRequest != nil && p.Stack.PlanRequest.Rejected != "" {
			return p.Stack.PlanRequest.Rejected
		}
	case secArch:
		if p.Structure != nil && p.Structure.PlanRequest != nil && p.Structure.PlanRequest.Rejected != "" {
			return p.Structure.PlanRequest.Rejected
		}
	case secRepo:
		if p.Structure != nil && p.Structure.RepoRequest != nil && p.Structure.RepoRequest.Rejected != "" {
			return p.Structure.RepoRequest.Rejected
		}
	}
	if r := p.ArtifactRequest; r != nil && r.Area == id && r.Rejected != "" {
		return r.Rejected
	}
	// A closed area request lost its refusal; the next request carries it
	// again (prepareBlueprintStep, areaWave). Other jobs have nowhere to put it.
	if s := p.BlueprintProgress.Section(id); s != nil && s.Kind == work.KindArtifacts && s.Validation == core.BPValFailed {
		return s.Error
	}
	return ""
}

// beginExecution opens the attempt the pipeline is about to make. The trigger
// states why: first try, the refusal going back to the AI, or a plain re-run.
func beginExecution(p *core.Project, id string, isMCP bool, now time.Time) {
	t := p.BlueprintProgress
	s := t.Section(id)
	if s == nil {
		return
	}
	if s.Status == core.BPCompleted || s.Status == core.BPSkipped {
		t.ReworkSection(id, "Dikerjakan ulang pada proses ini", now)
	}
	trigger, cause, activity := core.BPTriggerInitial, "", generatingActivity(id)
	switch rejected := rejectionOf(p, id); {
	case rejected != "":
		trigger, cause = core.BPTriggerRepair, rejected
		activity = fmt.Sprintf("Memperbaiki output %s (percobaan %d): %s", s.Name, s.Attempt+1, core.ShortCause(rejected))
	case s.Attempt > 0 && s.Status == core.BPInProgress:
		trigger, activity = core.BPTriggerContinuation, "Melengkapi "+s.Name
	case s.Attempt > 0:
		trigger, cause = core.BPTriggerRetry, s.Error
		activity = fmt.Sprintf("Mengulang %s (percobaan %d)", s.Name, s.Attempt+1)
	}
	if id == secTech && p.Stack != nil && p.Stack.PlanRequest != nil && p.Stack.PlanRequest.Model != "" {
		activity += fmt.Sprintf(" · model %s (eskalasi dari %s)", p.Stack.PlanRequest.Model, p.Stack.PlanRequest.EscalatedFrom)
	}
	if isMCP {
		activity += " — menunggu AI Host (MCP)"
	}
	t.BeginExecution(id, trigger, activity, cause, now)
}

func trackJobStart(p *core.Project, kind string, isMCP bool) {
	if !tracksBlueprint(p, kind) {
		return
	}
	ensureBlueprintProgress(p)
	if id := sectionForJob(p, kind); id != "" {
		beginExecution(p, id, isMCP, time.Now().UTC())
	}
}

// syncCompleted folds a landed answer into the run. own is the section whose
// attempt just passed validation: if its work needs another round it stays
// in progress instead of counting as done.
func syncCompleted(p *core.Project, own string) {
	t := liveRun(p)
	if t == nil {
		return
	}
	now := time.Now().UTC()
	t.ReconcilePlan(planSections(p), now)
	if s := t.Section(own); s != nil && s.Status == core.BPValidating {
		t.AcceptPartial(own, s.Name+" valid, masih ada bagian yang perlu dilengkapi", now)
	}
}

// finishRunIfDone closes the run once nothing can still move it.
func finishRunIfDone(p *core.Project) {
	t := liveRun(p)
	if t == nil || p.BlueprintBatch > 0 || t.Running() {
		return
	}
	if p.Job != nil && (p.Job.Status == core.JobRunning || p.Job.Status == core.JobWaiting || p.Job.Status == core.JobFailed) {
		return
	}
	now := time.Now().UTC()
	t.ReconcilePlan(planSections(p), now)
	t.CompleteRun(now)
}

func trackFail(p *core.Project, kind string, err error) {
	trackFailMsg(p, kind, err, safeProgressError(err))
}

// trackFailMsg is trackFail with the report to show.
func trackFailMsg(p *core.Project, kind string, err error, msg string) {
	t := liveRun(p)
	if t == nil {
		return
	}
	now := time.Now().UTC()
	t.FailSection(sectionForJob(p, kind), failureKind(err), msg, now)
	t.FailRun(msg, now)
}

// trackRejection records a refused answer. The next attempt, if any, opens
// its own execution.
func trackRejection(p *core.Project, kind string, err error) {
	trackRejectionMsg(p, kind, err, safeProgressError(err))
}

// trackRejectionMsg is trackRejection with the report to show: its first
// line labels the refusal, the others are its details.
func trackRejectionMsg(p *core.Project, kind string, err error, msg string) {
	if t := liveRun(p); t != nil {
		t.RejectOutput(sectionForJob(p, kind), failureKind(err), msg, time.Now().UTC())
	}
}

// failureKind names what went wrong, for the user and for the retry history.
func failureKind(err error) string {
	switch {
	case errors.Is(err, agent.ErrOverloaded):
		return core.BPFailOverloaded
	case errors.Is(err, agent.ErrLimit):
		return core.BPFailQuota
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return core.BPFailInterrupted
	case errors.Is(err, agent.ErrAgentFailed):
		return core.BPFailAgent
	case err != nil && strings.HasPrefix(err.Error(), "output AI"):
		return core.BPFailOutput
	}
	return core.BPFailValidation
}

// trackHostRepair: the MCP host received the refusal and answers again on the
// same waiting job, so the next attempt opens here; no job start happens.
func trackHostRepair(p *core.Project, kind string) {
	t := liveRun(p)
	if t == nil || p.Job == nil || p.Job.Kind != kind || p.Job.Status != core.JobWaiting {
		return
	}
	id := sectionForJob(p, kind)
	if s := t.Section(id); s != nil && s.Status == core.BPRepairing {
		activity := fmt.Sprintf("Memperbaiki output %s (percobaan %d): %s — menunggu AI Host (MCP)", s.Name, s.Attempt+1, core.ShortCause(s.Error))
		t.BeginExecution(id, core.BPTriggerRepair, activity, s.Error, time.Now().UTC())
	}
}

func (s *Service) markValidating(kind, section string) {
	p, err := s.Store.Load()
	if err != nil || liveRun(p) == nil {
		return
	}
	s.Store.Update(func(p *core.Project) error {
		t := liveRun(p)
		if t == nil {
			return nil
		}
		id := section
		if id == "" {
			id = sectionForJob(p, kind)
		}
		if sec := t.Section(id); sec != nil {
			word := "Memvalidasi"
			if sec.Attempt > 1 {
				word = "Memvalidasi ulang"
			}
			t.BeginValidation(id, fmt.Sprintf("%s output %s", word, sec.Name), time.Now().UTC())
		}
		return nil
	})
}

// markBusyWait closes the attempt the server refused; the wait is not work.
func (s *Service) markBusyWait(kind, section string, wait time.Duration, n, total int) {
	s.Store.Update(func(p *core.Project) error {
		if t := liveRun(p); t != nil {
			id := section
			if id == "" {
				id = sectionForJob(p, kind)
			}
			t.WaitRetry(id, fmt.Sprintf("Server AI sibuk, mencoba lagi dalam %s (%d/%d)", wait, n, total), time.Now().UTC())
		}
		return nil
	})
}

// markBusyRerun opens a new execution for the next attempt on the server.
func (s *Service) markBusyRerun(kind, section string) {
	s.Store.Update(func(p *core.Project) error {
		t := liveRun(p)
		if t == nil {
			return nil
		}
		id := section
		if id == "" {
			id = sectionForJob(p, kind)
		}
		if sec := t.Section(id); sec != nil && sec.Status == core.BPRetrying {
			t.BeginExecution(id, core.BPTriggerBusyRetry, fmt.Sprintf("Mengirim ulang %s ke AI (percobaan %d)", sec.Name, sec.Attempt+1), sec.Error, time.Now().UTC())
		}
		return nil
	})
}

// stopRun closes the run when the user dismisses its failed job.
func stopRun(p *core.Project) {
	if t := p.BlueprintProgress; t != nil {
		t.StopRun(reasonStopped, time.Now().UTC())
	}
}

// recoverRun settles a run whose process died. A run left running with no
// job behind it would otherwise spin forever.
func recoverRun(p *core.Project) {
	t := liveRun(p)
	if t == nil || (p.Job != nil && (p.Job.Status == core.JobRunning || p.Job.Status == core.JobWaiting)) {
		return
	}
	if p.Job != nil && p.Job.Status == core.JobFailed {
		t.FailRun(p.Job.Error, time.Now().UTC())
		return
	}
	if p.BlueprintBatch > 0 && len(p.BlueprintQueue) > 0 {
		const msg = "Proses terhenti sebelum selesai. Jalankan ulang."
		p.Job = &core.Job{Kind: p.BlueprintQueue[0], Status: core.JobFailed, Message: "Merancang Blueprint", Error: msg, Started: time.Now().UTC()}
		t.FailRun(msg, time.Now().UTC())
		return
	}
	finishRunIfDone(p)
}

// withETA readies a run for a reader: the estimate is derived, not stored.
func withETA(p *core.Project) {
	if t := p.BlueprintProgress; t != nil {
		t.ETA = t.Estimate(p.JobHistory, p.Job != nil && p.Job.Status == core.JobWaiting, time.Now().UTC())
	}
}

// BlueprintProgress returns the run without its timeline, and the timeline
// after the cursor.
func (s *Service) BlueprintProgress(after int64) (*core.BlueprintProgress, []core.BlueprintEvent, error) {
	p, err := s.Store.Load()
	if err != nil {
		return nil, nil, err
	}
	if p.BlueprintProgress == nil {
		return nil, nil, nil
	}
	withETA(p)
	events := p.BlueprintProgress.After(after)
	p.BlueprintProgress.Events = nil
	return p.BlueprintProgress, events, nil
}

func safeProgressError(err error) string {
	if err == nil {
		return "Proses gagal"
	}
	r := []rune(err.Error())
	if len(r) > 2000 {
		return string(r[:2000]) + "…"
	}
	return string(r)
}
