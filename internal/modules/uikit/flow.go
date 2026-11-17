package uikit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// The mockups follow the use cases: a page leads on to the next page of its
// flows, a page that hosts several steps of one flow shows them in order,
// and the mockup index lists every flow step by step with its page.

// MaxNextSteps caps the next-step links in a page header.
const MaxNextSteps = 2

type nextStep struct{ page, label string }

// headActions are the links to the next pages of the page's flows, then
// the primary action (the page's own, else the create step a flow starts
// with there).
func (r *render) headActions() {
	pg := r.page
	if formLike(pg.Pattern) || r.centered {
		return
	}
	pa := pg.PrimaryAction
	if pa == "" && !pg.Existing && slices.Contains(core.ActionPatterns, pg.Pattern) {
		pa = core.FlowPrimaryAction(r.doc.Walks, pg.ID)
	}
	next := r.nextSteps()
	docActs := r.documentActions(pa)
	if pa == "" && len(next) == 0 && len(docActs) == 0 {
		return
	}
	r.w(`<div class="fk-actions">`)
	for _, n := range next {
		// A next step that is a document transition follows its status:
		// in a status it is not available in, it is no way forward.
		if why := r.unavailableStep(n.label); why != "" {
			r.f(`<button class="fk-btn" disabled title="%s" data-foundry-sim="unavailable">%s</button>`, esc(why), esc(n.label))
			continue
		}
		r.f(`<a class="fk-btn" href="%s" data-foundry-link="%s">%s%s</a>`, esc(r.doc.href(n.page)), esc(n.page), esc(n.label), icon("right"))
	}
	if pa != "" {
		cls := "fk-btn fk-btn-primary"
		if core.DangerAction(pa) {
			cls = "fk-btn fk-btn-danger"
		}
		r.actionControl(pa, cls, true)
	}
	for _, a := range docActs {
		cls := "fk-btn"
		if core.DangerAction(a) || actionKindOf(r, a) == core.TransReject {
			cls = "fk-btn fk-btn-danger"
		}
		r.actionControl(a, cls, false)
	}
	r.w(`</div>`)
}

// unavailableStep says why a next-step link that is a transition of the
// simulated document cannot be taken in its status or by the role; ""
// when it can, or when it is no transition. The transition may belong to
// the page the link leads to.
func (r *render) unavailableStep(label string) string {
	lc, st := r.simLifecycle(), r.simState()
	if !r.simulating() || lc == nil || st == nil {
		return ""
	}
	var found *core.LifeTransition
	for i, t := range lc.Transitions {
		// By its words only: a link is no transition just because it is
		// of the same kind ("Kirim ke gudang" is not "Ajukan PO").
		a, b := strings.ToLower(strings.TrimSpace(label)), strings.ToLower(strings.TrimSpace(t.Action))
		if core.SystemActor(t.Actor) || b == "" || !(a == b || strings.Contains(a, b) || strings.Contains(b, a)) {
			continue
		}
		if r.available(t, st.ID) {
			return ""
		}
		if found == nil {
			found = &lc.Transitions[i]
		}
	}
	switch {
	case found == nil:
		return ""
	case slices.Contains(found.From, st.ID):
		return "Aksi ini untuk " + found.Actor
	}
	return "Tidak tersedia pada status " + lc.StateLabel(st.ID)
}

// simulating reports whether the prototype runs a document scenario (a
// status or document chosen); a plain mockup stays as designed.
func (r *render) simulating() bool {
	return r.doc.Sim.Status != "" || r.doc.Sim.Entity != ""
}

// documentActions are the lifecycle transitions people take on this page
// for the simulated document, besides the primary action: each becomes a
// control enabled only in the statuses and for the roles that allow it.
// The page's text may describe them in any words; the lifecycle decides.
func (r *render) documentActions(primary string) []string {
	lc := r.simLifecycle()
	if !r.simulating() || lc == nil {
		return nil
	}
	var out []string
	for _, pt := range r.pageTransitions() {
		if !strings.EqualFold(pt.Entity, lc.Entity) || primary != "" && core.MatchAction(primary, pt.T) {
			continue
		}
		if !slices.ContainsFunc(out, func(a string) bool { return strings.EqualFold(a, pt.T.Action) }) {
			out = append(out, pt.T.Action)
		}
	}
	if len(out) > 0 {
		r.docActs = out
	}
	return out
}

func actionKindOf(r *render, label string) string {
	if t := r.matchTransition(label); t != nil {
		return t.T.Kind
	}
	return ""
}

// nextSteps are the pages the page's flows continue on, labelled by the
// page link that leads there, else by the step taken there.
func (r *render) nextSteps() []nextStep {
	var out []nextStep
	for _, w := range r.doc.Walks {
		if w.Inferred {
			continue // a guessed page is no place for a real link
		}
		i := slices.Index(w.Pages, r.page.ID)
		if i < 0 || i+1 >= len(w.Pages) {
			continue
		}
		to := w.Pages[i+1]
		if to == r.page.ID || slices.ContainsFunc(out, func(n nextStep) bool { return n.page == to }) {
			continue
		}
		out = append(out, nextStep{page: to, label: r.nextLabel(w, to)})
		if len(out) == MaxNextSteps {
			break
		}
	}
	return out
}

func (r *render) nextLabel(w core.FlowWalk, to string) string {
	for _, ln := range r.page.Links {
		if ln.To != to {
			continue
		}
		via := strings.TrimSpace(ln.Via)
		for _, p := range []string{"klik ", "tautan ", "tombol ", "aksi "} {
			if len(via) > len(p) && strings.EqualFold(via[:len(p)], p) {
				via = via[len(p):]
				break
			}
		}
		if l := core.StepAction(via); l != "" && len([]rune(l)) <= 32 {
			return l
		}
	}
	for _, st := range w.Steps {
		if st.Page == to && !st.System && st.Placed {
			return core.StepAction(st.Text)
		}
	}
	if pg, err := r.doc.page(to); err == nil {
		return "Ke " + pg.Name
	}
	return "Lanjut"
}

// writeWalks lists every use case step by step with the page it happens
// on, then where its alternatives and failures branch off and which page
// shows them: the storyboard of the mockups.
func writeWalks(b *strings.Builder, d Doc) {
	if len(d.Walks) == 0 {
		return
	}
	name := map[string]string{}
	if d.Layout != nil {
		for _, pg := range d.Layout.Pages {
			name[pg.ID] = pg.Name
		}
	}
	pageLink := func(id string) string {
		return fmt.Sprintf(`<a class="fk-link" href="%s">%s</a>`, esc(d.href(id)), esc(name[id]))
	}
	b.WriteString(`<section class="fk-sec"><h2>Alur use case</h2><p class="fk-note">Tiap langkah pengguna dan halaman tempatnya, berurutan, lalu cabang alternatif dan kegagalannya. Langkah sistem tampil sebagai umpan balik di halaman yang sedang dibuka.</p><div class="fk-walks">`)
	for _, w := range d.Walks {
		fmt.Fprintf(b, `<article class="fk-walk" data-foundry-flow="%s"><header><strong>%s %s</strong>`, esc(w.ID), esc(w.ID), esc(w.Name))
		if w.Role != "" {
			fmt.Fprintf(b, `<span class="fk-meta">%s</span>`, esc(w.Role))
		}
		if w.Inferred {
			b.WriteString(`<span class="fk-badge fk-badge-warning">Halaman ditebak</span>`)
		}
		b.WriteString(`</header>`)
		if w.Trigger != "" || len(w.Pre) > 0 {
			b.WriteString(`<p class="fk-walk-meta">`)
			if w.Trigger != "" {
				fmt.Fprintf(b, `<b>Pemicu</b> %s `, esc(w.Trigger))
			}
			if len(w.Pre) > 0 {
				fmt.Fprintf(b, `<b>Syarat</b> %s`, esc(strings.Join(w.Pre, "; ")))
			}
			b.WriteString(`</p>`)
		}
		b.WriteString(`<ol>`)
		for _, st := range w.Steps {
			cls := ""
			if st.System {
				cls = ` class="sys"`
			}
			fmt.Fprintf(b, `<li%s>%s`, cls, esc(st.Text))
			switch {
			case st.Page == "":
				b.WriteString(` <span class="fk-badge fk-badge-error">Belum ada halaman</span>`)
			case st.System:
				fmt.Fprintf(b, ` <span class="fk-badge">Otomatis · %s</span>`, esc(name[st.Page]))
			default:
				b.WriteString(" " + pageLink(st.Page))
				if !st.Placed {
					b.WriteString(` <span class="fk-badge fk-badge-warning">Belum tergambar</span>`)
				}
			}
			if st.Decision {
				b.WriteString(` <span class="fk-badge fk-badge-info">Keputusan</span>`)
			}
			if st.Risky {
				b.WriteString(` <span class="fk-badge fk-badge-error">Perlu konfirmasi</span>`)
			}
			if st.Group > 0 {
				fmt.Fprintf(b, ` <span class="fk-badge">Paralel %d</span>`, st.Group)
			}
			for _, s := range st.Status {
				fmt.Fprintf(b, ` <span class="fk-meta">%s</span>`, esc(s))
			}
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ol>`)
		if len(w.Branches) > 0 {
			b.WriteString(`<ul class="fk-walk-branches">`)
			for _, br := range w.Branches {
				kind, badge := "Alternatif", "fk-badge-info"
				if br.Kind == core.BranchFailure {
					kind, badge = "Gagal", "fk-badge-error"
				}
				fmt.Fprintf(b, `<li><span class="fk-badge %s">%s</span>`, badge, kind)
				if br.From > 0 {
					fmt.Fprintf(b, ` <span class="fk-meta">di langkah %d</span>`, br.From)
				}
				fmt.Fprintf(b, ` %s`, esc(br.Text))
				switch {
				case br.System:
					b.WriteString(` <span class="fk-badge">Otomatis</span>`)
				case br.Placed:
					b.WriteString(" " + pageLink(br.Page))
				default:
					b.WriteString(` <span class="fk-badge fk-badge-warning">Belum terlihat</span>`)
				}
				for _, id := range br.Flows {
					fmt.Fprintf(b, ` <span class="fk-meta">→ %s</span>`, esc(id))
				}
				b.WriteString(`</li>`)
			}
			b.WriteString(`</ul>`)
		}
		if len(w.Post) > 0 {
			fmt.Fprintf(b, `<p class="fk-walk-meta"><b>Hasil</b> %s</p>`, esc(strings.Join(w.Post, "; ")))
		}
		b.WriteString(`</article>`)
	}
	b.WriteString(`</div></section>`)
}
