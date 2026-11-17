package uikit

import (
	"net/url"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// Sim is the prototype simulation chosen in the mockup URL: a role, a
// document status, whether the simulated actions succeed, and a one-shot
// view: the feedback of an action, the dialog drawn open. Nothing is
// stored.
type Sim struct {
	Role     string
	Status   string
	Entity   string
	Feedback string // ok | reject | error | invalid | ""
	// Outcome is what the simulated actions return: "" succeeds, "error"
	// fails.
	Outcome string
	// View is the id of the dialog or confirmation drawn open; Act the
	// action the feedback answers; Fill shows a record in the open form
	// (an edit).
	View string
	Act  string
	Fill bool
}

// Nav is Sim without the one-shot view: carried from page to page.
func (s Sim) Nav() Sim {
	return Sim{Role: s.Role, Status: s.Status, Entity: s.Entity, Outcome: s.Outcome}
}

// OutcomeError makes the simulated actions fail.
const OutcomeError = "error"

// ParseSim reads the prototype query of a mockup request.
func ParseSim(q url.Values) Sim {
	s := Sim{Role: strings.TrimSpace(q.Get("role")), Status: strings.TrimSpace(q.Get("status")),
		Entity: strings.TrimSpace(q.Get("entity")), Feedback: strings.TrimSpace(q.Get("sim")),
		View: strings.TrimSpace(q.Get("view")), Act: strings.TrimSpace(q.Get("act")), Fill: q.Get("fill") == "1"}
	if strings.TrimSpace(q.Get("outcome")) == OutcomeError {
		s.Outcome = OutcomeError
	}
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
	for k, v := range map[string]string{"outcome": sim.Outcome, "view": sim.View, "act": sim.Act} {
		if v != "" {
			q.Set(k, v)
		}
	}
	if sim.Fill {
		q.Set("fill", "1")
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
		r.success()
	case "reject":
		r.toast("reject", "reject", "Simulasi penolakan — perubahan tidak tersimpan.")
	case "error":
		// A failure shows where it happened: in the dialog drawn open, in
		// the page's form; else on its own.
		if r.doc.Sim.View == "" && r.mainForm() == "" {
			f := r.failure(r.doc.Sim.Act, nil)
			r.result(false, f.title, f.body, f.code, f.spec)
		}
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
	opens := ""
	if !dis {
		opens = r.dialogFor(label)
	}
	if opens != "" {
		// A dialog of the page does what the action does.
		confirmAttr = ` popovertarget="` + opens + `"`
	} else if core.ConfirmAction(label, lt) && !dis {
		pid = "c-" + ident(label)
		confirmAttr = ` data-foundry-confirm="true" popovertarget="` + pid + `"`
	}
	iconHTML := ""
	if withIcon {
		iconHTML = actionIcon(label)
	}
	if opens == "" && pid == "" && !dis {
		// An action another page's form does leads there.
		if to := r.formPageFor(label); to != "" {
			text := capital(strings.TrimSpace(actionName(label)))
			r.f(`<a class="%s" href="%s" data-foundry-link="%s">%s%s</a>`, cls, esc(r.actionHref(to, label)), esc(to), iconHTML, esc(text))
			return
		}
	}
	// The button names the action; what limits it ("untuk Admin") is its
	// tooltip.
	text, extra := capital(strings.TrimSpace(actionName(label))), ""
	if text == "" {
		text = label
	}
	switch {
	case dis:
		extra = " disabled"
		if why != "" {
			extra += ` title="` + esc(why) + `"`
		}
	case text != label:
		extra = ` title="` + esc(label) + `"`
	}
	r.f(`<button class="%s"%s%s>%s%s</button>`, cls, confirmAttr, extra, iconHTML, esc(text))
	if pid != "" {
		r.asked(pid, label, label)
		r.confirmBox(pid, label, t)
	}
}

func (r *render) confirmBox(id, label string, t *core.PageTransition) {
	attrs, open := r.popAttrs(id)
	r.confirmHead(attrs, label, ` data-foundry-sim="confirm"`)
	r.w(`<p class="fk-confirm-note">Simulasi — tidak tersimpan. Ini bukan transaksi backend.</p></div>`)
	r.failAlert(id, label)
	switch {
	case t != nil && t.T.Kind == core.TransReject && r.doc.live():
		r.f(`<form method="get" action="%s">`, esc(r.doc.Self))
		sim := r.outcome(label, id)
		if sim.Feedback == "ok" {
			sim.Status, sim.Feedback = t.T.To, "reject"
		}
		r.hidden(sim)
		r.w(`<div class="fk-field fk-field-wide"><label for="reason">Alasan penolakan<span class="fk-req" aria-hidden="true">*</span></label><textarea id="reason" name="reason" class="fk-input" required></textarea><p class="fk-err">` + icon("alert-circle") + `Alasan penolakan wajib diisi.</p></div>`)
		r.w(`<div class="fk-actions">`)
		r.dismiss("Batal", id, open)
		r.f(`<button class="fk-btn fk-btn-danger" type="submit">%s</button></div></form>`, esc(capital(actionName(label))))
	case r.doc.live():
		sim := r.outcome(label, id)
		if t != nil && sim.Feedback == "ok" {
			sim.Status = t.T.To
		}
		cls := "fk-btn fk-btn-primary"
		if core.DangerAction(label) {
			cls = "fk-btn fk-btn-danger"
		}
		r.w(`<div class="fk-actions">`)
		r.dismiss("Batal", id, open)
		r.f(`<a class="%s" href="%s" data-foundry-sim="%s">%s</a></div>`, cls, esc(r.doc.hrefSim(r.page.ID, sim)), sim.Feedback, esc(capital(actionName(label))))
	default:
		r.w(`<p class="fk-note">Konfirmasi ini hanya simulasi prototype.</p>`)
	}
	r.w(`</div>`)
	if open {
		r.w(`</div>`)
	}
}

func (r *render) hiddenSim(status, feedback string) {
	sim := r.doc.Sim.Nav()
	sim.Status, sim.Feedback = status, feedback
	r.hidden(sim)
}

// hidden carries sim in a GET form: the query WithSim would write.
func (r *render) hidden(sim Sim) {
	u, err := url.Parse(WithSim("x", sim))
	if err != nil {
		return
	}
	q := u.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		r.f(`<input type="hidden" name="%s" value="%s">`, esc(k), esc(q.Get(k)))
	}
}

// outcome is the simulation an action leads to: its success, or, when
// the simulated actions fail, the failure shown where the action was
// taken (the open dialog view, "" for the page).
func (r *render) outcome(act, view string) Sim {
	sim := r.doc.Sim.Nav()
	sim.Act = act
	if sim.Outcome == OutcomeError {
		sim.Feedback, sim.View = "error", view
		return sim
	}
	sim.Feedback = "ok"
	return sim
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

// formPageFor is the form page an action of this page leads to: the one
// form page (of this page's feature first) whose name holds the action's
// verb when that verb fills a form ("Ubah data" leads to "Tambah atau
// Ubah Barang", "Catat masuk" to "Catat Mutasi"). "" when none, or more
// than one, fits, and on a form page.
func (r *render) formPageFor(label string) string {
	f := strings.Fields(strings.ToLower(actionName(label)))
	if len(f) == 0 || formLike(r.page.Pattern) || !(slices.Contains(formVerbs, f[0]) || core.StartsCreate(f[0])) {
		return ""
	}
	// What the action works on ("retur" of "Buat retur") must be what the
	// form makes; a bare verb ("Ubah", "Ubah data") works on this page's
	// record.
	var object []string
	for _, w := range f[1:] {
		if !slices.Contains([]string{"data", "ini", "baru", "item", "record"}, w) {
			object = append(object, w)
		}
	}
	bare := len(object) == 0
	var same, other []string
	for _, pg := range r.doc.Layout.Pages {
		if pg.ID == r.page.ID || !formLike(pg.Pattern) || pg.Pattern == "auth" ||
			!slices.Contains(strings.Fields(strings.ToLower(pg.Name)), f[0]) ||
			!bare && !core.SharesStem(strings.Join(object, " "), pg.Name) {
			continue
		}
		if pg.FeatureID != "" && pg.FeatureID == r.page.FeatureID {
			same = append(same, pg.ID)
		} else {
			other = append(other, pg.ID)
		}
	}
	switch {
	case len(same) == 1:
		return same[0]
	case len(same) == 0 && len(other) == 1 && !bare:
		return other[0]
	}
	return ""
}
