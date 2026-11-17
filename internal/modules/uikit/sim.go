package uikit

import (
	"net/url"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// Sim is the prototype simulation chosen in the mockup URL: a role, a
// document status, and a one-shot feedback flash. Nothing is stored.
type Sim struct {
	Role     string
	Status   string
	Entity   string
	Feedback string // ok | reject | error | ""
}

// Nav is Sim without the flash: carried from page to page.
func (s Sim) Nav() Sim { return Sim{Role: s.Role, Status: s.Status, Entity: s.Entity} }

// ParseSim reads the prototype query of a mockup request.
func ParseSim(q url.Values) Sim {
	s := Sim{Role: strings.TrimSpace(q.Get("role")), Status: strings.TrimSpace(q.Get("status")),
		Entity: strings.TrimSpace(q.Get("entity")), Feedback: strings.TrimSpace(q.Get("sim"))}
	if s.Feedback == "" && q.Has("ok") {
		s.Feedback = "ok"
	}
	return s
}

// WithSim adds the simulation query to href. Empty fields are left out.
func WithSim(href string, sim Sim) string {
	q := url.Values{}
	if sim.Role != "" {
		q.Set("role", sim.Role)
	}
	if sim.Status != "" {
		q.Set("status", sim.Status)
	}
	if sim.Entity != "" {
		q.Set("entity", sim.Entity)
	}
	if sim.Feedback != "" {
		q.Set("sim", sim.Feedback)
	}
	if len(q) == 0 {
		return href
	}
	u, err := url.Parse(href)
	if err != nil || href == "" || href == "#" {
		return href
	}
	got := u.Query()
	for k, vs := range q {
		got.Set(k, vs[0])
	}
	u.RawQuery = got.Encode()
	return u.String()
}

func (r *render) simBanner() {
	r.w(`<aside class="fk-sim" role="status" data-foundry-sim="banner"><b>Simulasi prototype — tidak tersimpan.</b> Bukan transaksi backend.</aside>`)
	switch r.doc.Sim.Feedback {
	case "ok":
		r.w(`<p class="fk-toast" role="status" data-foundry-sim="ok">Simulasi berhasil — perubahan tidak tersimpan.</p>`)
	case "reject":
		r.w(`<p class="fk-toast" role="status" data-foundry-sim="reject">Simulasi penolakan — perubahan tidak tersimpan.</p>`)
	case "error":
		r.w(`<div class="fk-alert" role="status" data-foundry-sim="error">Simulasi gagal — perubahan tidak tersimpan.</div>`)
	}
}

func (r *render) simLifecycle() *core.Lifecycle {
	p := r.doc.Project
	if p == nil || len(p.Lifecycles) == 0 {
		return nil
	}
	if e := strings.TrimSpace(r.doc.Sim.Entity); e != "" {
		return p.LifecycleOf(e)
	}
	if r.doc.Layout != nil {
		for _, pt := range r.pageTransitions() {
			if lc := p.LifecycleOf(pt.Entity); lc != nil {
				return lc
			}
		}
	}
	if r.page != nil {
		for _, name := range r.page.Data {
			if lc := p.LifecycleOf(name); lc != nil {
				return lc
			}
		}
	}
	if len(p.Lifecycles) == 1 {
		return &p.Lifecycles[0]
	}
	return nil
}

func (r *render) simState() *core.LifeState {
	lc := r.simLifecycle()
	if lc == nil {
		return nil
	}
	id := r.doc.Sim.Status
	if id == "" {
		id = lc.Initial()
	}
	return lc.State(id)
}

func (r *render) pageTransitions() []core.PageTransition {
	if r.doc.Project == nil || r.doc.Layout == nil || r.page == nil {
		return nil
	}
	if r.doc.trans == nil {
		r.doc.trans = r.doc.Project.TransitionsByPage(r.doc.Layout)
	}
	return r.doc.trans[r.page.ID]
}

func (r *render) matchTransition(label string) *core.PageTransition {
	ts := r.pageTransitions()
	status := ""
	if st := r.simState(); st != nil {
		status = st.ID
	}
	// A page that decides several documents: the simulated one's
	// transitions answer for it.
	if lc := r.simLifecycle(); lc != nil {
		var own []core.PageTransition
		for _, pt := range ts {
			if strings.EqualFold(pt.Entity, lc.Entity) {
				own = append(own, pt)
			}
		}
		if slices.ContainsFunc(own, func(pt core.PageTransition) bool { return core.MatchAction(label, pt.T) }) {
			ts = own
		}
	}
	// A label that names a transition by its words is that transition;
	// only a label naming none falls back to the kind ("Batalkan sebelum
	// konfirmasi" is not "Batalkan pesanan terkonfirmasi", seen live).
	match := core.MatchActionWords
	if !slices.ContainsFunc(ts, func(pt core.PageTransition) bool { return core.MatchActionWords(label, pt.T) }) {
		match = core.MatchAction
	}
	var fallback *core.PageTransition
	for i := range ts {
		if !match(label, ts[i].T) {
			continue
		}
		if r.available(ts[i].T, status) {
			return &ts[i]
		}
		if fallback == nil {
			fallback = &ts[i]
		}
	}
	return fallback
}

// available reports whether the simulated role may take t in status.
func (r *render) available(t core.LifeTransition, status string) bool {
	return r.doc.Project.AvailableFor(t, status, r.doc.Sim.Role)
}

func (r *render) actionState(label string) (disabled bool, t *core.PageTransition, why string) {
	if r.state == "disabled" || r.state == "loading" {
		return true, nil, ""
	}
	if r.doc.Sim.Role != "" && r.page != nil && !core.RoleAllowed(r.page.Role, r.doc.Sim.Role) {
		return true, nil, "Peran " + r.doc.Sim.Role + " tidak memakai halaman ini"
	}
	t = r.matchTransition(label)
	if t == nil {
		return false, nil, ""
	}
	status := ""
	if st := r.simState(); st != nil {
		status = st.ID
	}
	if !r.available(t.T, status) {
		why = "Tidak tersedia"
		if status != "" && r.simLifecycle() != nil {
			why = "Tidak tersedia pada status " + r.simLifecycle().StateLabel(status)
		}
		if r.doc.Sim.Role != "" && !r.doc.Project.ActorAllows(t.T.Actor, r.doc.Sim.Role) {
			why = "Aksi ini untuk " + t.T.Actor
		}
		return true, t, why
	}
	return false, t, ""
}

func (r *render) actionControl(label, cls string, withIcon bool) {
	dis, t, why := r.actionState(label)
	confirmAttr, pid := "", ""
	var lt *core.LifeTransition
	if t != nil {
		lt = &t.T
	}
	if core.ConfirmAction(label, lt) && !dis {
		pid = "c-" + ident(label)
		confirmAttr = ` data-foundry-confirm="true" popovertarget="` + pid + `"`
	}
	iconHTML := ""
	if withIcon {
		iconHTML = actionIcon(label)
	}
	extra := ""
	if dis {
		extra = " disabled"
		if why != "" {
			extra += ` title="` + esc(why) + `"`
		}
	}
	r.f(`<button class="%s"%s%s>%s%s</button>`, cls, confirmAttr, extra, iconHTML, esc(label))
	if pid != "" {
		r.confirmBox(pid, label, t)
	}
}

func (r *render) confirmBox(id, label string, t *core.PageTransition) {
	r.f(`<div id="%s" popover class="fk-confirm-box" data-foundry-sim="confirm">`, id)
	r.w(`<p><b>Simulasi — tidak tersimpan.</b> Ini bukan transaksi backend.</p>`)
	switch {
	case t != nil && t.T.Kind == core.TransReject && r.doc.live():
		r.f(`<form method="get" action="%s">`, esc(r.doc.Self))
		r.hiddenSim(t.T.To, "reject")
		r.w(`<div class="fk-field fk-field-wide"><label for="reason">Alasan penolakan</label><textarea id="reason" name="reason" class="fk-input" required></textarea></div>`)
		r.f(`<button class="fk-btn fk-btn-danger" type="submit">%s</button></form>`, esc(label))
	case r.doc.live() && t != nil:
		r.f(`<a class="fk-btn fk-btn-primary" href="%s">Lanjut (simulasi)</a>`, esc(r.doc.hrefAt(r.page.ID, t.T.To, "ok")))
	default:
		r.w(`<p class="fk-note">Konfirmasi ini hanya simulasi prototype.</p>`)
	}
	r.w(`</div>`)
}

func (r *render) hiddenSim(status, feedback string) {
	if r.doc.Sim.Role != "" {
		r.f(`<input type="hidden" name="role" value="%s">`, esc(r.doc.Sim.Role))
	}
	if r.doc.Sim.Entity != "" {
		r.f(`<input type="hidden" name="entity" value="%s">`, esc(r.doc.Sim.Entity))
	}
	if status != "" {
		r.f(`<input type="hidden" name="status" value="%s">`, esc(status))
	}
	if feedback != "" {
		r.f(`<input type="hidden" name="sim" value="%s">`, esc(feedback))
	}
}

func (r *render) formLocked() bool {
	st := r.simState()
	return st != nil && st.Locked
}

// offeredAbove reports whether the header already offers this document
// action.
func (r *render) offeredAbove(label string) bool {
	t := r.matchTransition(label)
	return t != nil && slices.ContainsFunc(r.docActs, func(a string) bool { return core.MatchAction(a, t.T) })
}
