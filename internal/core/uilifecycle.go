package core

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The screens follow the document lifecycles: a transition's action sits
// on the page where its use case step happens, approval and rejection of a
// status are decided on the same screen, a form does not edit a document in
// a locked status, and a page shows only statuses its documents have.

// PageTransition is a lifecycle transition a page offers: a person takes
// its use case step on that page.
type PageTransition struct {
	Entity string         `json:"entity"`
	Flow   string         `json:"flow"`
	T      LifeTransition `json:"transition"`
}

// RoleAllowed reports whether a preview role may use a page whose Role
// lists the actors of its use cases (comma-separated). An empty side
// allows every role.
func RoleAllowed(pageRole, simRole string) bool {
	if strings.TrimSpace(simRole) == "" || strings.TrimSpace(pageRole) == "" {
		return true
	}
	var names []string
	for _, r := range strings.Split(pageRole, ",") {
		if k := roleKey(r); k != "" {
			names = append(names, k)
		}
	}
	return knownRole(names, simRole)
}

// ActorAllowed reports whether a preview role may take an action of actor.
func ActorAllowed(actor, simRole string) bool {
	if strings.TrimSpace(simRole) == "" || strings.TrimSpace(actor) == "" || SystemActor(actor) {
		return true
	}
	return knownRole([]string{roleKey(simRole)}, actor)
}

// Available reports whether the transition can fire in status for role.
func (t LifeTransition) Available(status, role string) bool {
	if status != "" && !slices.Contains(t.From, status) {
		return false
	}
	return ActorAllowed(t.Actor, role)
}

// MatchAction reports whether a control label is this transition.
func MatchAction(label string, t LifeTransition) bool {
	if actionMatches(label, t.Action) {
		return true
	}
	return t.Kind != "" && actionKind(label) == t.Kind
}

func actionMatches(label, action string) bool {
	a, b := strings.ToLower(strings.TrimSpace(label)), strings.ToLower(strings.TrimSpace(action))
	return a != "" && b != "" && (a == b || strings.Contains(a, b) || strings.Contains(b, a))
}

func actionKind(label string) string {
	w := stems(label)
	switch {
	case hasStem(w, "setuju", "approv"):
		return TransApprove
	case hasStem(w, "tolak", "reject", "kembali"):
		return TransReject
	case hasStem(w, "batal", "cancel"):
		return TransCancel
	case hasStem(w, "revisi", "revise", "perbaiki"):
		return TransRevise
	case hasStem(w, "ajukan", "submit", "kirim"):
		return TransSubmit
	}
	return ""
}

// ConfirmAction reports whether firing the action needs a confirmation.
func ConfirmAction(label string, t *LifeTransition) bool {
	if DangerAction(label) {
		return true
	}
	if t == nil {
		return false
	}
	return t.Kind == TransApprove || t.Kind == TransReject || t.Kind == TransCancel
}

// TransitionsByPage lists, per page, the transitions people take there.
// Automatic transitions (the system's) and those whose step has no place
// on a page are left out.
func (p *Project) TransitionsByPage(l *LayoutSpec) map[string][]PageTransition {
	out := map[string][]PageTransition{}
	if l == nil {
		return out
	}
	steps := map[string][]FlowStep{}
	for _, lc := range p.Lifecycles {
		for _, t := range lc.Transitions {
			fl := p.flow(t.Flow)
			if fl == nil || t.Step < 1 || t.Step > len(fl.Steps) || SystemActor(t.Actor) || !p.ScreenFlow(*fl) {
				continue
			}
			s, ok := steps[fl.ID]
			if !ok {
				s = p.FlowSteps(l, *fl)
				steps[fl.ID] = s
			}
			if st := s[t.Step-1]; st.Placed && st.Page != "" {
				out[st.Page] = append(out[st.Page], PageTransition{Entity: lc.Entity, Flow: fl.ID, T: t})
			}
		}
	}
	return out
}

// statusList finds a list of statuses in a page text: "Status: Draft,
// Diajukan" or "Status (Pending, Berhasil, Gagal)".
var statusList = regexp.MustCompile(`(?i)\bstatus\b[^:(;]{0,24}[:(]\s*([^);]+)`)

// statusValues are the statuses a page text lists.
func statusValues(text string) []string {
	var out []string
	for _, m := range statusList.FindAllStringSubmatch(text, -1) {
		for _, v := range regexp.MustCompile(`,|/| atau | dan | or | and `).Split(m[1], -1) {
			if v = strings.Trim(strings.TrimSpace(v), ".'\""); v != "" && len([]rune(v)) <= 32 && len(strings.Fields(v)) <= 3 {
				out = append(out, v)
			}
		}
	}
	return out
}

// stateMatches reports whether a status a page shows is the lifecycle
// state: its id or label, word for word by root.
func stateMatches(v string, s LifeState) bool {
	norm := func(x string) string {
		return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(x, "_", " ")), " "))
	}
	v = norm(v)
	if v == norm(s.ID) || v == norm(s.Label) {
		return true
	}
	vw := stems(v)
	if len(vw) == 0 {
		return false
	}
	for _, name := range []string{s.ID, s.Label} {
		nw := stems(norm(name))
		if len(nw) > 0 && !slices.ContainsFunc(vw, func(a string) bool {
			return !slices.ContainsFunc(nw, func(b string) bool { return stemMatch(a, b) })
		}) {
			return true
		}
	}
	return false
}

// lifecycleUIIssues checks the screens against the lifecycles.
func lifecycleUIIssues(p *Project, l *LayoutSpec) []Issue {
	if len(p.Lifecycles) == 0 || l == nil {
		return nil
	}
	var out []Issue
	add := func(level, where, msg string, a ...any) {
		out = append(out, Issue{Level: level, Where: where, Message: fmt.Sprintf(msg, a...)})
	}
	byPage := p.TransitionsByPage(l)
	// Approval and rejection of one status belong on one screen: the
	// approver decides there.
	for _, lc := range p.Lifecycles {
		where := map[string]map[string]string{} // state -> kind -> page
		for page, ts := range byPage {
			for _, pt := range ts {
				if pt.Entity != lc.Entity || pt.T.Kind != TransApprove && pt.T.Kind != TransReject {
					continue
				}
				for _, f := range pt.T.From {
					if where[f] == nil {
						where[f] = map[string]string{}
					}
					where[f][pt.T.Kind] = page
				}
			}
		}
		for state, kinds := range where {
			if a, r := kinds[TransApprove], kinds[TransReject]; a != "" && r != "" && a != r {
				add(IssueWarning, "page:"+a, "%s berstatus %s disetujui di halaman ini tetapi ditolak di %s: satukan keputusan di satu layar",
					lc.Entity, lc.StateLabel(state), r)
			}
		}
	}
	pages := make([]string, 0, len(byPage))
	for id := range byPage {
		pages = append(pages, id)
	}
	slices.Sort(pages)
	for _, id := range pages {
		pg := layoutPage(l, id)
		if pg == nil || pg.Existing {
			continue
		}
		var entities []*Lifecycle
		for _, pt := range byPage[id] {
			lc := p.LifecycleOf(pt.Entity)
			if !slices.Contains(entities, lc) {
				entities = append(entities, lc)
			}
			if pg.Pattern != "form" && pg.Pattern != "wizard" || pt.T.Kind == TransCancel || pt.T.Kind == TransRevise {
				continue
			}
			if !slices.ContainsFunc(pt.T.From, func(f string) bool { s := lc.State(f); return s == nil || !s.Locked }) {
				add(IssueWarning, "page:"+id, "formulir ini menjalankan %q pada %s yang terkunci (%s): dokumen terkunci tidak diubah lewat form",
					pt.T.Action, lc.Entity, strings.Join(pt.T.From, ", "))
			}
		}
		for _, v := range statusValues(pageText(pg)) {
			if !slices.ContainsFunc(entities, func(lc *Lifecycle) bool {
				return slices.ContainsFunc(lc.States, func(s LifeState) bool { return stateMatches(v, s) })
			}) {
				names := make([]string, len(entities))
				for i, lc := range entities {
					names[i] = lc.Entity
				}
				add(IssueWarning, "page:"+id, "status %q tidak ada di siklus %s: tampilkan hanya status yang dimiliki dokumennya", v, strings.Join(names, ", "))
			}
		}
	}
	return out
}

// lifecycleCheck is the gate check of the document lifecycles and the
// screens that follow them.
func lifecycleCheck(p *Project, l *LayoutSpec) GateCheck {
	c := check("lifecycle", "Siklus status dokumen")
	if len(p.Lifecycles) == 0 {
		c.Status, c.Detail = GateSkip, "use case belum menulis siklus status dokumen"
		return c
	}
	trans, traced := 0, 0
	for _, lc := range p.Lifecycles {
		for _, t := range lc.Transitions {
			trans++
			if t.Flow != "" && t.Step > 0 {
				traced++
			}
		}
	}
	c.Detail = fmt.Sprintf("%d dokumen, %d dari %d transisi tertelusur ke langkah use case", len(p.Lifecycles), traced, trans)
	for _, is := range p.LifecycleIssues(p.Lifecycles, p.Flows) {
		if is.Level != IssueInfo {
			c.note(levelStatus(is.Level), "%s: %s", is.Where, is.Message)
		}
	}
	for _, is := range lifecycleUIIssues(p, l) {
		c.note(levelStatus(is.Level), "%s: %s", is.Where, is.Message)
	}
	return c
}
