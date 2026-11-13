package stack

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The technology decision is reviewed, not just shown: a change becomes a
// proposal with its validation and impact, the user applies or cancels it,
// every applied change is a new version, and approval needs no errors.

// Change is one requested change: an empty Value returns the area to the
// recommendation.
type Change struct {
	Area   string `json:"area"`
	Value  string `json:"value"`
	Reason string `json:"reason,omitempty"`
}

var (
	ErrNoProposal = errors.New("tidak ada perubahan teknologi yang menunggu")
	ErrNoChange   = errors.New("tidak ada perubahan: pilihan sama dengan yang sekarang")
	ErrPending    = errors.New("terapkan atau batalkan perubahan teknologi yang menunggu dulu")
	// ErrNotDesigned: a new project has no technology until the AI plan (or
	// the user) decides it; Foundry has none of its own to approve.
	ErrNotDesigned = errors.New("teknologi belum dirancang: minta AI merancang teknologi dulu")
)

const maxHistory = 50

// Propose validates changes and records them as a proposal with the
// issues and impact the stack would have. Foundry derives no other area
// from a change: dependent areas are the AI's to revise. It merges with a
// waiting proposal; nothing in the decisions changes until Apply.
func Propose(s *core.TechStack, changes []Change, source, request, note string) error {
	if s == nil {
		return errors.New("teknologi belum direkomendasikan")
	}
	if len(changes) == 0 {
		return ErrNoChange
	}
	want := map[string]Change{}
	var order []string
	if s.Proposal != nil {
		for _, c := range s.Proposal.Changes {
			if !c.Follow {
				want[c.Area] = Change{Area: c.Area, Value: c.To, Reason: c.Reason}
				order = append(order, c.Area)
			}
		}
	}
	for _, c := range changes {
		c.Area = strings.TrimSpace(c.Area)
		if _, ok := want[c.Area]; !ok {
			order = append(order, c.Area)
		}
		want[c.Area] = c
	}
	next := &core.TechStack{Mode: s.Mode, Needs: s.Needs, Platform: s.Platform, Decisions: core.CloneDecisions(s.Decisions), Plan: s.Plan}
	changed := map[string]bool{}
	for _, a := range order {
		c := want[a]
		value := c.Value
		if d := next.Decision(a); d != nil && value == d.Recommended {
			value = ""
		}
		if err := Override(next, c.Area, value, c.Reason); err != nil {
			return err
		}
		changed[a] = true
	}
	for a := range changed {
		if d := s.Decision(a); d != nil && d.Choice() == next.Choice(a) {
			delete(changed, a)
		}
	}
	diff := Diff(s.Decisions, next.Decisions)
	if len(diff) == 0 {
		if s.Proposal != nil {
			Discard(s)
		}
		return ErrNoChange
	}
	for i := range diff {
		if _, own := want[diff[i].Area]; !own {
			diff[i].Follow = true
			if d := next.Decision(diff[i].Area); d != nil {
				diff[i].Reason = d.Reason
			}
		}
	}
	var review []core.TechIssue
	for _, d := range next.Decisions {
		if old := s.Decision(d.Area); d.Review != "" && (old == nil || old.Review != d.Review) {
			review = append(review, core.TechIssue{Level: "warning", Area: d.Area, Message: d.Review})
		}
	}
	prev := s.State()
	if s.Proposal != nil {
		prev = s.Proposal.Previous
	}
	s.Proposal = &core.TechProposal{Source: source, Request: request, Note: note, Changes: diff, Review: review,
		Issues: Validate(next), Impact: Impact(next, diff), Decisions: next.Decisions, Previous: prev, At: time.Now().UTC()}
	s.Status, s.Approved = core.TechReview, false
	return nil
}

// Apply makes the waiting proposal the decision and saves it as a new
// version. Errors do not stop it (the user decides); they block approval.
func Apply(s *core.TechStack) (*core.TechVersion, error) {
	if s == nil || s.Proposal == nil {
		return nil, ErrNoProposal
	}
	pr := s.Proposal
	before := core.CloneDecisions(s.Decisions)
	if pr.Decisions != nil {
		s.Decisions = core.CloneDecisions(pr.Decisions)
	} else {
		// proposal saved before it carried the decisions
		for _, c := range pr.Changes {
			value := c.To
			if d := s.Decision(c.Area); d != nil && value == d.Recommended {
				value = ""
			}
			if err := Override(s, c.Area, value, c.Reason); err != nil {
				return nil, err
			}
		}
	}
	s.Proposal = nil
	if pr.Plan != nil {
		s.Plan = pr.Plan
	}
	st := core.TechImpactChecked
	if pr.Previous == core.TechApproved || pr.Previous == core.TechChanged {
		st = core.TechChanged
	}
	s.SetStatus(st)
	Refresh(s)
	v := Record(s, before, pr.Source, pr.Request)
	for i, c := range v.Changes {
		for _, x := range pr.Changes {
			if x.Area == c.Area && x.Follow {
				v.Changes[i].Follow, v.Changes[i].Reason = true, x.Reason
			}
		}
	}
	return v, nil
}

// ProposeDecisions records a proposal whose decisions are given whole (the
// AI's technology plan decides every area itself) instead of being built
// from single changes. Nothing changes until Apply; the plan becomes the
// stack's plan then.
func ProposeDecisions(s *core.TechStack, next []core.TechDecision, plan *core.TechPlan, source, request, note string) error {
	if s == nil {
		return errors.New("teknologi belum direkomendasikan")
	}
	if s.Proposal != nil {
		return ErrPending
	}
	cand := &core.TechStack{Mode: s.Mode, Needs: s.Needs, Platform: s.Platform, Decisions: core.CloneDecisions(next), Plan: plan}
	diff := Diff(s.Decisions, cand.Decisions)
	if len(diff) == 0 {
		return ErrNoChange
	}
	var review []core.TechIssue
	for _, d := range cand.Decisions {
		if old := s.Decision(d.Area); d.Review != "" && (old == nil || old.Review != d.Review) {
			review = append(review, core.TechIssue{Level: "warning", Area: d.Area, Message: d.Review})
		}
	}
	s.Proposal = &core.TechProposal{Source: source, Request: request, Note: note, Changes: diff, Review: review,
		Issues: Validate(cand), Impact: Impact(cand, diff), Decisions: cand.Decisions, Previous: s.State(), At: time.Now().UTC(), Plan: plan}
	s.Status, s.Approved = core.TechReview, false
	return nil
}

// ApplyDecisions makes next the decision at once and saves a new version,
// for a plan that replaces a recommendation nobody chose or approved. A
// stack the user already approved or changed goes through ProposeDecisions.
func ApplyDecisions(s *core.TechStack, next []core.TechDecision, plan *core.TechPlan, source, request string) *core.TechVersion {
	before := core.CloneDecisions(s.Decisions)
	s.Decisions = core.CloneDecisions(next)
	s.Plan = plan
	Refresh(s)
	if s.State() == "" {
		s.SetStatus(core.TechRecommended)
	}
	return Record(s, before, source, request)
}

// ConfirmReview keeps area's choice after a decision it depends on
// changed: the review mark is cleared, nothing else changes.
func ConfirmReview(s *core.TechStack, area string) error {
	d := s.Decision(area)
	if d == nil || d.Review == "" {
		return fmt.Errorf("%s tidak menunggu tinjauan", AreaName(area))
	}
	d.Review = ""
	Refresh(s)
	return nil
}

// NeedsApproval reports whether the user must approve the stack: always,
// except an existing project whose stack is only what its code already
// uses.
func NeedsApproval(s *core.TechStack) bool {
	if s == nil || s.Mode != core.StructureExisting || s.Proposal != nil || len(s.History) > 0 {
		return true
	}
	for _, d := range s.Decisions {
		if d.Override != nil || d.Review != "" || !fromExisting(s, d) {
			return true
		}
	}
	return false
}

// Discard drops the waiting proposal; the decisions never changed.
func Discard(s *core.TechStack) error {
	if s == nil || s.Proposal == nil {
		return ErrNoProposal
	}
	s.SetStatus(s.Proposal.Previous)
	s.Proposal = nil
	return nil
}

// Approve accepts the current decision. A waiting proposal or an error
// finding blocks it.
func Approve(s *core.TechStack) error {
	if s == nil {
		return errors.New("teknologi belum direkomendasikan")
	}
	if s.Proposal != nil {
		return ErrPending
	}
	if s.Mode != core.StructureExisting && len(s.Decisions) == 0 {
		return ErrNotDesigned
	}
	Refresh(s)
	if r := s.Reviews(); len(r) > 0 {
		return fmt.Errorf("%d keputusan perlu ditinjau ulang dulu: %s", len(r), r[0].Review)
	}
	if errs := s.Errors(); len(errs) > 0 {
		return fmt.Errorf("%d masalah teknologi harus diselesaikan dulu: %s", len(errs), errs[0].Message)
	}
	if len(s.History) == 0 {
		Record(s, nil, "recommend", "")
	}
	s.SetStatus(core.TechApproved)
	return nil
}

// Record saves the current decisions as a new version. before is the state
// they changed from (nil for the first version); a stack saved before
// versions existed gets its old state as version 1 first.
func Record(s *core.TechStack, before []core.TechDecision, source, request string) *core.TechVersion {
	now := time.Now().UTC()
	if len(s.History) == 0 && before != nil {
		s.History = append(s.History, core.TechVersion{Version: 1, At: now, Source: "recommend",
			Status: core.TechRecommended, Decisions: before})
	}
	v := core.TechVersion{Version: s.Version() + 1, At: now, Source: source, Request: request,
		Status: s.State(), Decisions: core.CloneDecisions(s.Decisions)}
	if before != nil {
		v.Changes = Diff(before, s.Decisions)
		v.Impact = Impact(s, v.Changes)
		v.Design = slices.ContainsFunc(v.Impact, func(i core.TechImpact) bool { return i.Design })
	}
	s.History = append(s.History, v)
	if n := len(s.History); n > maxHistory {
		s.History = slices.Delete(s.History, 1, n-maxHistory+1) // keep v1 as the baseline
	}
	return &s.History[len(s.History)-1]
}

// Diff lists the areas whose chosen technology differs from before to
// after; an area that appears or disappears counts as a change.
func Diff(before, after []core.TechDecision) []core.TechChange {
	old := map[string]core.TechDecision{}
	for _, d := range before {
		old[d.Area] = d
	}
	var out []core.TechChange
	seen := map[string]bool{}
	for _, d := range after {
		seen[d.Area] = true
		o, ok := old[d.Area]
		if ok && o.Choice() == d.Choice() {
			continue
		}
		c := core.TechChange{Area: d.Area, To: d.Choice(), Language: d.Lang()}
		if ok {
			c.From = o.Choice()
		}
		if d.Override != nil {
			c.Reason = d.Override.Reason
		}
		out = append(out, c)
	}
	for _, d := range before {
		if !seen[d.Area] {
			out = append(out, core.TechChange{Area: d.Area, From: d.Choice()})
		}
	}
	return out
}

// affects is what each area's technology reaches into.
var affects = map[string][]string{
	core.TechFrontend:      {"kode frontend", "komponen UI dan handoff rancangan", "testing frontend", "deployment frontend"},
	core.TechUI:            {"komponen UI", "handoff rancangan", "token desain"},
	core.TechCLI:           {"kode CLI", "distribusi binary/paket", "testing CLI"},
	core.TechAPI:           {"kontrak API", "client di frontend/mobile", "dokumentasi API", "testing API"},
	core.TechTesting:       {"test suite", "CI"},
	core.TechQuality:       {"aturan lint dan format", "CI"},
	core.TechMobile:        {"aplikasi mobile", "handoff rancangan mobile", "testing mobile", "rilis ke store"},
	core.TechDesktop:       {"aplikasi desktop", "handoff rancangan desktop", "testing desktop", "distribusi installer"},
	core.TechBackend:       {"implementasi API", "struktur backend", "authentication dan authorization", "implementasi worker", "testing backend", "deployment", "observability"},
	core.TechDatabase:      {"model data dan migrasi", "query dan repository layer", "transaksi", "backup dan deployment", "testing data"},
	core.TechCache:         {"performance", "session dan rate limit", "state realtime", "deployment"},
	core.TechMessaging:     {"komunikasi antar proses", "worker", "realtime", "deployment", "observability"},
	core.TechWorkers:       {"job latar belakang", "antrean", "deployment", "observability"},
	core.TechStorage:       {"upload dan download file", "security file (signed URL)", "deployment", "backup"},
	core.TechSearch:        {"index pencarian", "sinkronisasi data", "API pencarian", "deployment"},
	core.TechArchitecture:  {"service boundary", "struktur repository", "deployment strategy", "komunikasi antar modul", "testing integrasi"},
	core.TechRepository:    {"struktur repository", "paket bersama", "CI/CD", "deployment"},
	core.TechDeployment:    {"infrastruktur", "CI/CD", "scalability", "security jaringan", "biaya operasional"},
	core.TechObservability: {"logging", "metrics dan tracing", "alerting", "deployment"},
}

// Impact is what each change affects in s (the stack after the change).
func Impact(s *core.TechStack, changes []core.TechChange) []core.TechImpact {
	var out []core.TechImpact
	for _, c := range changes {
		a := append([]string(nil), affects[c.Area]...)
		if len(a) == 0 {
			a = []string{"bagian sistem yang memakai " + AreaName(c.Area)}
		}
		if c.Area == core.TechArchitecture || c.Area == core.TechRepository {
			a = append(a, "struktur project: tidak diubah otomatis; minta AI menganalisis ulang struktur bila tidak lagi sesuai")
		}
		out = append(out, core.TechImpact{Area: c.Area, Affects: a, Design: core.AffectsDesign(c.Area)})
	}
	return out
}

// Refresh re-validates the current decisions.
func Refresh(s *core.TechStack) {
	if s != nil {
		s.Issues = Validate(s)
	}
}

func none(v string) bool { return v == "" || v == core.TechNone }

// Validate checks what Foundry can check without knowing technologies:
// a decision without a choice, and a replaced technology of the code
// (a migration). Compatibility between technologies is the AI plan's
// review (techplan), never a list kept in Foundry.
func Validate(s *core.TechStack) []core.TechIssue {
	if s == nil {
		return nil
	}
	var out []core.TechIssue
	add := func(level, area, msg string, args ...any) {
		out = append(out, core.TechIssue{Level: level, Area: area, Message: fmt.Sprintf(msg, args...)})
	}
	for _, d := range s.Decisions {
		if strings.TrimSpace(d.Choice()) == "" {
			add("error", d.Area, "%s belum punya pilihan teknologi.", AreaName(d.Area))
		}
		if d.Override != nil && fromExisting(s, d) {
			add("warning", d.Area, "%s sudah dipakai di kode; menggantinya dengan %s adalah migrasi kode dan data yang harus direncanakan.", d.Recommended, d.Choice())
		}
	}
	return out
}
