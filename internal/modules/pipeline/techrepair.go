package pipeline

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/agent"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

// Repair of a refused technology plan. Every refused answer is an attempt in
// the request's history with its error count; the next attempt runs only
// while the attempts make progress. An attempt that fixes nothing moves the
// work to a stronger model once, when one is configured; after that, or at
// the attempt limit, the work stops with a diagnosis and keeps its draft so
// a retry continues from it.

// DefaultTechAttempts bounds the answers of one automatic chain, the first
// included.
const DefaultTechAttempts = 4

// RepairPolicy configures the automatic repair. Zero values use the
// defaults.
type RepairPolicy struct {
	// MaxAttempts bounds the answers per chain; a user retry starts a new
	// chain.
	MaxAttempts int
	// Escalation names the stronger model for a runner, "" for none.
	// Default: agent.EscalationModel (Haiku to Sonnet on Claude Code).
	Escalation func(*agent.Runner) string
	// WithModel builds the runner of the escalated model. Default:
	// Runner.WithModel.
	WithModel func(*agent.Runner, string) (*agent.Runner, error)
}

func (s *Service) maxTechAttempts() int {
	if s.Repair.MaxAttempts > 0 {
		return s.Repair.MaxAttempts
	}
	return DefaultTechAttempts
}

// escalation is the model a stuck chain may move to; "" in MCP mode, where
// the host picks its own model, and when none is known.
func (s *Service) escalation() (from, to string) {
	runner, isMCP, err := s.AI.Runner()
	if err != nil || isMCP || runner == nil {
		return "", ""
	}
	from = runner.Model()
	if from == "" {
		from = "default"
	}
	if s.Repair.Escalation != nil {
		return from, s.Repair.Escalation(runner)
	}
	return from, agent.EscalationModel(runner)
}

// techRunner is runner, or the escalated runner when the technology request
// the job answers moved to another model.
func (s *Service) techRunner(p *core.Project, kind string, runner *agent.Runner) *agent.Runner {
	r := techRequestOf(p, kind)
	if r == nil || r.Model == "" || runner == nil {
		return runner
	}
	build := s.Repair.WithModel
	if build == nil {
		build = (*agent.Runner).WithModel
	}
	esc, err := build(runner, r.Model)
	if err != nil {
		s.Store.Update(func(p *core.Project) error {
			p.Log("system", fmt.Sprintf("Eskalasi ke model %s gagal (%v); memakai model yang dikonfigurasi", r.Model, err))
			return nil
		})
		return runner
	}
	return esc
}

// techRequestOf is the technology request a job of kind answers now.
func techRequestOf(p *core.Project, kind string) *core.TechPlanRequest {
	if p.Stack == nil || p.Stack.PlanRequest == nil {
		return nil
	}
	switch {
	case kind == work.KindTech, kind == work.KindPlan && work.CurrentPlanPart(p) == work.KindTech:
		return p.Stack.PlanRequest
	}
	return nil
}

// techStrategy is how the attempt that just ran (or runs next) was asked.
func techStrategy(p *core.Project, kind string) string {
	if kind == work.KindPlan {
		return work.PlanStrategy(p)
	}
	return work.TechStrategy(p)
}

// recordTechAttempt adds a refused answer to the request's history, keeps
// the best draft, and decides whether the chain goes on. It runs before the
// request changes, so the strategy is the one the attempt used.
func recordTechAttempt(p *core.Project, kind string, err error, escFrom, escTo string, max int) bool {
	r := techRequestOf(p, kind)
	if r == nil {
		return false
	}
	att := core.TechAttempt{N: len(r.Attempts) + 1, At: time.Now().UTC(), Strategy: techStrategy(p, kind), Model: r.Model, Errors: -1}
	var rej *techplan.RejectError
	if errors.As(err, &rej) {
		att.Errors, att.Codes = len(rej.Issues), techplan.CodeCounts(rej.Issues)
		if m := rej.Merge; m != nil {
			att.Repaired, att.Ignored = m.Repaired, m.Ignored
		}
		// Never trade a draft for a worse one: what an earlier attempt got
		// right stays the base of the next repair.
		better := r.Draft == nil || len(rej.Issues) <= len(r.Issues)
		if better {
			r.Issues = rej.Issues
			if r.Action == core.TechPlanGenerate && rej.Plan != nil {
				r.Draft = rej.Plan
			}
		}
	}
	if r.ChainStart > len(r.Attempts) {
		r.ChainStart = len(r.Attempts)
	}
	chain := r.Attempts[r.ChainStart:]
	att.Progress = core.TechProgressFirst
	if n := len(chain); n > 0 {
		att.Progress = progressOf(chain[n-1].Errors, att.Errors)
	}
	r.Attempts = append(r.Attempts, att)
	r.Rejected = err.Error()
	if n := len(chain) + 1; n >= max {
		return false
	}
	switch att.Progress {
	case core.TechProgressFirst, core.TechProgressImproved:
		return true
	}
	if r.Model == "" && escTo != "" {
		prev := chain[len(chain)-1]
		r.Model, r.EscalatedFrom = escTo, escFrom
		r.Escalation = fmt.Sprintf("percobaan %d tidak membuat progres (%s → %s error)", att.N, errCount(prev.Errors), errCount(att.Errors))
		p.Log("system", fmt.Sprintf("Eskalasi model %s → %s untuk rencana teknologi: %s", escFrom, escTo, r.Escalation))
		return true
	}
	return false
}

func progressOf(before, after int) string {
	switch {
	case after < 0 && before < 0:
		return core.TechProgressStalled
	case after < 0:
		return core.TechProgressRegressed
	case before < 0, after < before:
		return core.TechProgressImproved
	case after == before:
		return core.TechProgressStalled
	}
	return core.TechProgressRegressed
}

func errCount(n int) string {
	if n < 0 {
		return "tidak terbaca"
	}
	return fmt.Sprint(n)
}

var strategyLabel = map[string]string{
	core.TechAttemptGenerate: "rancangan penuh",
	core.TechAttemptRepair:   "perbaikan terarah",
	core.TechAttemptSyntax:   "perbaikan sintaks JSON",
}

var progressLabel = map[string]string{
	core.TechProgressFirst:     "percobaan pertama",
	core.TechProgressImproved:  "membaik",
	core.TechProgressStalled:   "tidak ada progres",
	core.TechProgressRegressed: "memburuk",
}

// techDiagnosis is the report of a refused or failed technology plan: the
// first line for one-line labels, the rest for the details. final is set
// when the chain stopped.
func techDiagnosis(p *core.Project, kind string, err error, final bool) string {
	r := techRequestOf(p, kind)
	var rej *techplan.RejectError
	if r == nil || !errors.As(err, &rej) && len(r.Attempts) == 0 {
		return safeProgressError(err)
	}
	var lines []string
	issues := r.Issues
	if rej != nil {
		issues = rej.Issues
	}
	if rej != nil {
		lines = append(lines, fmt.Sprintf("Rencana teknologi ditolak validator: %d masalah (%s)", len(rej.Issues), techplan.GroupSummary(rej.Issues)))
	} else {
		lines = append(lines, "Rencana teknologi ditolak: "+core.ShortCause(err.Error()))
	}
	if n := len(r.Attempts); n > 0 {
		last := r.Attempts[n-1]
		var trail []string
		for _, a := range r.Attempts[min(r.ChainStart, n):] {
			trail = append(trail, errCount(a.Errors))
		}
		model := last.Model
		if model == "" {
			model = "model terkonfigurasi"
		}
		lines = append(lines, fmt.Sprintf("Percobaan %d · %s · %s · %s · error per percobaan: %s",
			last.N, model, strategyLabel[last.Strategy], progressLabel[last.Progress], strings.Join(trail, " → ")))
		if len(last.Repaired) > 0 {
			lines = append(lines, fmt.Sprintf("Diperbaiki otomatis: %d area (%s)", len(last.Repaired), strings.Join(cutList(last.Repaired, 8), ", ")))
		}
		for _, ig := range cutList(last.Ignored, 3) {
			lines = append(lines, "Diabaikan: "+ig)
		}
	}
	if r.Escalation != "" {
		lines = append(lines, fmt.Sprintf("Eskalasi %s → %s: %s", r.EscalatedFrom, r.Model, r.Escalation))
	}
	if r.Draft != nil {
		lines = append(lines, fmt.Sprintf("Disimpan: %d keputusan tervalidasi dari draf terbaik (%d masalah tersisa)", validItems(r.Draft, r.Issues), len(r.Issues)))
	}
	status := "Status: menunggu perbaikan otomatis berikutnya"
	if final {
		status = "Status: gagal; bagian valid disimpan, Coba lagi melanjutkan dari draf"
	}
	lines = append(lines, status)
	for _, is := range issues {
		lines = append(lines, fmt.Sprintf("%s: %s", orDash(is.Path), is.Message))
	}
	return strings.Join(lines, "\n")
}

// validItems counts the draft's decisions no issue names.
func validItems(plan *core.TechPlan, issues []core.TechIssue) int {
	bad := map[string]bool{}
	for _, is := range issues {
		bad[is.Area] = true
	}
	n := 0
	for _, it := range plan.Items {
		if !bad[it.Area] {
			n++
		}
	}
	return n
}

func cutList(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return append(in[:n:n], fmt.Sprintf("+%d lagi", len(in)-n))
}

func orDash(s string) string {
	if s == "" {
		return "plan"
	}
	return s
}

// newTechChain starts a fresh automatic chain on the open technology
// request, for a retry the user asked for. Its draft and history stay.
func newTechChain(p *core.Project) {
	if p.Stack != nil && p.Stack.PlanRequest != nil {
		r := p.Stack.PlanRequest
		r.ChainStart = len(r.Attempts)
	}
}

// keepTechDraft carries the draft of a refused design into a new design
// request, while the draft's sources still hold: a new run continues the
// repair instead of throwing the valid decisions away.
func keepTechDraft(p *core.Project, next *core.TechPlanRequest) *core.TechPlanRequest {
	old := p.Stack.PlanRequest
	if old == nil || old.Draft == nil || next.Action != core.TechPlanGenerate || old.Action != core.TechPlanGenerate {
		return next
	}
	if len(techplan.StaleAreas(p, old.Draft)) > 0 {
		return next
	}
	next.Draft, next.Issues, next.Rejected = old.Draft, old.Issues, old.Rejected
	next.Attempts, next.ChainStart = old.Attempts, len(old.Attempts)
	next.Model, next.EscalatedFrom, next.Escalation = old.Model, old.EscalatedFrom, old.Escalation
	return next
}
