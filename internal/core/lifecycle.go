package core

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// A business document (an order, a request, an invoice) moves through
// statuses. Its lifecycle is written with the use cases from the
// requirements: the statuses it can be in, and each transition with the
// action, the actor and the use case step that performs it. Documents do
// not share one lifecycle; a document without status changes has none.
// The screens follow it: the actions a page offers are the transitions
// available in the document's status, and a status the lifecycle does not
// have is not shown.

// Lifecycle is the status model of one business document.
type Lifecycle struct {
	Entity      string           `json:"entity"`
	States      []LifeState      `json:"states"`
	Transitions []LifeTransition `json:"transitions"`
}

// LifeState is one status. Kind marks where the document starts and where
// it ends; Locked a status in which the document can no longer be edited
// (only the transitions out of it change it).
type LifeState struct {
	ID     string `json:"id"`
	Label  string `json:"label,omitempty"`
	Kind   string `json:"kind,omitempty"` // initial | final | "" (in between)
	Locked bool   `json:"locked,omitempty"`
}

// LifeTransition moves the document from one of From to To when Actor
// takes Action. Flow and Step trace it to the use case step that performs
// it; Guard is what must hold before it is allowed.
type LifeTransition struct {
	From   []string `json:"from"`
	To     string   `json:"to"`
	Action string   `json:"action"`
	Kind   string   `json:"kind,omitempty"`
	Actor  string   `json:"actor"`
	Flow   string   `json:"flow,omitempty"`
	Step   int      `json:"step,omitempty"`
	Guard  string   `json:"guard,omitempty"`
}

// State kinds.
const (
	StateInitial = "initial"
	StateFinal   = "final"
)

// StateKinds are the valid LifeState kinds besides "".
var StateKinds = []string{StateInitial, StateFinal}

// Transition kinds: what a transition means for the document.
const (
	TransSubmit   = "submit"
	TransApprove  = "approve"
	TransReject   = "reject"
	TransRevise   = "revise"
	TransCancel   = "cancel"
	TransComplete = "complete"
)

// TransitionKinds are the valid LifeTransition kinds besides "".
var TransitionKinds = []string{TransSubmit, TransApprove, TransReject, TransRevise, TransCancel, TransComplete}

// State is the status with the id, or nil.
func (lc *Lifecycle) State(id string) *LifeState {
	for i := range lc.States {
		if lc.States[i].ID == id {
			return &lc.States[i]
		}
	}
	return nil
}

// StateLabel is the status as the user reads it.
func (lc *Lifecycle) StateLabel(id string) string {
	if s := lc.State(id); s != nil && strings.TrimSpace(s.Label) != "" {
		return s.Label
	}
	return id
}

// Initial is the status a new document starts in ("" when none is marked).
func (lc *Lifecycle) Initial() string {
	for _, s := range lc.States {
		if s.Kind == StateInitial {
			return s.ID
		}
	}
	if len(lc.States) > 0 {
		return lc.States[0].ID
	}
	return ""
}

// Out lists the transitions available in a status.
func (lc *Lifecycle) Out(state string) []LifeTransition {
	var out []LifeTransition
	for _, t := range lc.Transitions {
		if slices.Contains(t.From, state) {
			out = append(out, t)
		}
	}
	return out
}

// PathTo is a shortest way from the initial status to state: the
// transitions taken, in order (nil when it cannot be reached).
func (lc *Lifecycle) PathTo(state string) []LifeTransition {
	start := lc.Initial()
	if start == "" || state == start {
		return nil
	}
	prev := map[string]int{start: -1}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for i, t := range lc.Transitions {
			if _, seen := prev[t.To]; seen || !slices.Contains(t.From, cur) {
				continue
			}
			prev[t.To] = i
			if t.To == state {
				var path []LifeTransition
				for s := state; prev[s] >= 0; {
					tr := lc.Transitions[prev[s]]
					path = append([]LifeTransition{tr}, path...)
					s = fromOf(lc, tr, prev)
				}
				return path
			}
			queue = append(queue, t.To)
		}
	}
	return nil
}

// fromOf is the status a transition on a found path was taken from.
func fromOf(lc *Lifecycle, t LifeTransition, prev map[string]int) string {
	for _, f := range t.From {
		if _, ok := prev[f]; ok {
			return f
		}
	}
	return lc.Initial()
}

// SystemActor reports whether an actor is the system, a device or a worker
// rather than a person.
func SystemActor(actor string) bool {
	f := strings.Fields(strings.ToLower(strings.TrimSpace(actor)))
	return len(f) > 0 && systemWords[strings.Trim(f[0], ",.:;")]
}

// LifecycleOf is the lifecycle of the document (any case), or nil.
func (p *Project) LifecycleOf(entity string) *Lifecycle {
	for i := range p.Lifecycles {
		if strings.EqualFold(p.Lifecycles[i].Entity, strings.TrimSpace(entity)) {
			return &p.Lifecycles[i]
		}
	}
	return nil
}

// StepTransition is a lifecycle transition a use case step performs.
type StepTransition struct {
	Entity string         `json:"entity"`
	T      LifeTransition `json:"transition"`
}

// StepTransitions lists the transitions each step of a use case performs,
// by step number.
func (p *Project) StepTransitions(flowID string) map[int][]StepTransition {
	out := map[int][]StepTransition{}
	for _, lc := range p.Lifecycles {
		for _, t := range lc.Transitions {
			if t.Step > 0 && strings.EqualFold(t.Flow, flowID) {
				out[t.Step] = append(out[t.Step], StepTransition{Entity: lc.Entity, T: t})
			}
		}
	}
	return out
}

// TransitionText is a transition as one line: "PurchaseOrder: Menunggu
// persetujuan → Disetujui".
func (lc *Lifecycle) TransitionText(t LifeTransition) string {
	from := make([]string, len(t.From))
	for i, f := range t.From {
		from[i] = lc.StateLabel(f)
	}
	return fmt.Sprintf("%s: %s → %s", lc.Entity, strings.Join(from, "/"), lc.StateLabel(t.To))
}

// LifecycleIssues validates the lifecycles against the use cases: statuses
// and transitions are consistent, every status can be reached and left
// (unless it ends the document), every approval can be refused, and each
// transition is traced to a use case step done by a known actor.
func (p *Project) LifecycleIssues(lcs []Lifecycle, flows []Flow) []Issue {
	var out []Issue
	add := func(level, where, format string, a ...any) {
		out = append(out, Issue{Level: level, Where: where, Message: fmt.Sprintf(format, a...)})
	}
	flow := func(id string) *Flow {
		for i := range flows {
			if strings.EqualFold(flows[i].ID, id) {
				return &flows[i]
			}
		}
		return nil
	}
	roles := p.knownRoles(flows)
	seen := map[string]bool{}
	for _, lc := range lcs {
		w := "lifecycle:" + lc.Entity
		key := strings.ToLower(strings.TrimSpace(lc.Entity))
		switch {
		case key == "":
			add(IssueError, "use-cases", "siklus status tanpa nama dokumen (entity)")
			continue
		case seen[key]:
			add(IssueError, w, "dokumen %s punya lebih dari satu siklus status", lc.Entity)
		}
		seen[key] = true
		if len(lc.States) == 0 {
			add(IssueError, w, "status belum diisi")
			continue
		}
		states := map[string]bool{}
		initial, final := 0, 0
		for _, s := range lc.States {
			switch {
			case strings.TrimSpace(s.ID) == "":
				add(IssueError, w, "status tanpa id")
			case states[s.ID]:
				add(IssueError, w, "status %q ditulis lebih dari sekali", s.ID)
			}
			states[s.ID] = true
			switch s.Kind {
			case StateInitial:
				initial++
			case StateFinal:
				final++
			case "":
			default:
				add(IssueError, w, "status %q: jenis %q tidak dikenal (initial, final, atau kosong)", s.ID, s.Kind)
			}
		}
		if initial == 0 {
			add(IssueWarning, w, "belum ada status awal (kind initial): status pertama %q dianggap awal", lc.States[0].ID)
		}
		if final == 0 {
			add(IssueWarning, w, "belum ada status akhir (kind final): dokumen tidak pernah selesai")
		}
		for i, t := range lc.Transitions {
			tw, n := w, i+1
			if strings.TrimSpace(t.Action) == "" {
				add(IssueError, tw, "transisi %d: aksi belum diisi", n)
			}
			if len(t.From) == 0 {
				add(IssueError, tw, "transisi %d: status asal (from) belum diisi", n)
			}
			for _, f := range t.From {
				if !states[f] {
					add(IssueError, tw, "transisi %d: %q: status asal %q tidak ada di siklus %s", n, t.Action, f, lc.Entity)
				}
			}
			if !states[t.To] {
				add(IssueError, tw, "transisi %d: %q: status tujuan %q tidak ada di siklus %s", n, t.Action, t.To, lc.Entity)
			}
			if t.Kind != "" && !slices.Contains(TransitionKinds, t.Kind) {
				add(IssueError, tw, "transisi %d: %q: jenis %q tidak dikenal (%s)", n, t.Action, t.Kind, strings.Join(TransitionKinds, ", "))
			}
			switch actor := strings.TrimSpace(t.Actor); {
			case actor == "":
				add(IssueError, tw, "transisi %d: %q: aktor belum diisi (peran, atau Sistem)", n, t.Action)
			case !SystemActor(actor) && len(roles) > 0 && !knownRole(roles, actor):
				add(IssueWarning, tw, "transisi %d: %q: aktor %q bukan peran di use case atau daftar peran", n, t.Action, actor)
			}
			switch fl := flow(t.Flow); {
			case strings.TrimSpace(t.Flow) == "":
				add(IssueWarning, tw, "transisi %d: %q tidak dapat ditelusuri: sebutkan use case dan langkah yang menjalankannya", n, t.Action)
			case fl == nil:
				add(IssueError, tw, "transisi %d: %q merujuk use case %s yang tidak ada", n, t.Action, t.Flow)
			case t.Step < 0 || t.Step > len(fl.Steps):
				add(IssueError, tw, "transisi %d: %q merujuk langkah %d dari %s yang hanya punya %d langkah", n, t.Action, t.Step, fl.ID, len(fl.Steps))
			case t.Step == 0:
				add(IssueInfo, tw, "transisi %d: %q belum menyebut langkah %s yang menjalankannya", n, t.Action, fl.ID)
			}
		}
		for _, s := range lc.States {
			outs := lc.Out(s.ID)
			switch {
			case s.Kind == StateFinal && len(outs) > 0:
				add(IssueWarning, w, "status akhir %q masih punya transisi keluar (%s)", s.ID, outs[0].Action)
			case s.Kind != StateFinal && len(outs) == 0:
				add(IssueWarning, w, "status %q buntu: tidak ada transisi keluar dan bukan status akhir", s.ID)
			}
			approve, refuse := "", false
			for _, t := range outs {
				switch t.Kind {
				case TransApprove:
					approve = t.Action
				case TransReject, TransRevise, TransCancel:
					refuse = true
				}
			}
			if approve != "" && !refuse {
				add(IssueWarning, w, "status %q bisa disetujui (%s) tetapi tidak bisa ditolak, dikembalikan, atau dibatalkan", s.ID, approve)
			}
		}
		reach := map[string]bool{lc.Initial(): true}
		for changed := true; changed; {
			changed = false
			for _, t := range lc.Transitions {
				if !reach[t.To] && slices.ContainsFunc(t.From, func(f string) bool { return reach[f] }) {
					reach[t.To], changed = true, true
				}
			}
		}
		for _, s := range lc.States {
			if !reach[s.ID] {
				add(IssueWarning, w, "status %q tidak bisa dicapai dari status awal %q", s.ID, lc.Initial())
			}
		}
	}
	return out
}

// knownRoles are the actors the project names: discovery roles, use case
// actors and the users of the design.
func (p *Project) knownRoles(flows []Flow) []string {
	var out []string
	if p.Discovery != nil {
		for _, r := range p.Discovery.Roles {
			out = append(out, roleKey(r.Name))
		}
	}
	for _, f := range flows {
		out = append(out, roleKey(f.Role))
	}
	if p.UI != nil && p.UI.Layout != nil {
		for _, u := range p.UI.Layout.Users {
			out = append(out, roleKey(u.Role))
		}
	}
	return slices.DeleteFunc(out, func(s string) bool { return s == "" })
}

// knownRole reports whether an actor is one of the roles; a role written
// longer or shorter ("Supervisor" and "supervisor gudang") still matches.
func knownRole(roles []string, actor string) bool {
	k := roleKey(actor)
	return slices.ContainsFunc(roles, func(r string) bool { return strings.Contains(r, k) || strings.Contains(k, r) })
}

// parallelIssues validates the parallel groups of a use case.
func parallelIssues(f Flow, add func(level, where, format string, a ...any)) {
	used := map[int]bool{}
	for i, g := range f.Parallel {
		w := f.ID
		add := func(level, where, format string, a ...any) {
			add(level, where, "langkah paralel %d: "+format, append([]any{i + 1}, a...)...)
		}
		if len(g.Steps) < 2 {
			add(IssueError, w, "butuh paling sedikit dua langkah")
		}
		last := 0
		for _, n := range g.Steps {
			switch {
			case n < 1 || n > len(f.Steps):
				add(IssueError, w, "langkah %d tidak ada (use case punya %d langkah)", n, len(f.Steps))
			case used[n]:
				add(IssueError, w, "langkah %d sudah ada di kelompok paralel lain", n)
			}
			used[n], last = true, max(last, n)
		}
		switch j := strings.TrimSpace(g.Join); j {
		case "", "all", "any":
		default:
			if k, err := strconv.Atoi(j); err != nil || k < 1 || k > len(g.Steps) {
				add(IssueError, w, "join %q harus all, any, atau jumlah 1-%d", g.Join, len(g.Steps))
			}
		}
		if g.Then != 0 && (g.Then <= last || g.Then > len(f.Steps)) {
			add(IssueError, w, "then %d harus langkah sesudah kelompok paralel", g.Then)
		}
		if strings.TrimSpace(g.OnTimeout) != "" && strings.TrimSpace(g.Timeout) == "" {
			add(IssueWarning, w, "on_timeout diisi tetapi batas waktunya (timeout) belum")
		}
	}
}

// ParallelGroup is the parallel group (1-based) a step belongs to, or 0.
func ParallelGroup(f Flow, step int) int {
	for i, g := range f.Parallel {
		if slices.Contains(g.Steps, step) {
			return i + 1
		}
	}
	return 0
}

// JoinText says in words when a parallel group lets the flow go on.
func JoinText(join string) string {
	switch j := strings.TrimSpace(join); j {
	case "", "all":
		return "semua harus selesai"
	case "any":
		return "cukup salah satu"
	default:
		return "cukup " + j + " di antaranya"
	}
}
