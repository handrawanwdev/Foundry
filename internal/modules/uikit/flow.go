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
	if pa == "" && len(next) == 0 {
		return
	}
	r.w(`<div class="fk-actions">`)
	for _, n := range next {
		r.f(`<a class="fk-btn" href="%s" data-foundry-link="%s">%s%s</a>`, esc(r.doc.href(n.page)), esc(n.page), esc(n.label), icon("right"))
	}
	if pa != "" {
		cls, confirm := "fk-btn fk-btn-primary", ""
		if core.DangerAction(pa) {
			cls, confirm = "fk-btn fk-btn-danger", ` data-foundry-confirm="true"`
		}
		r.f(`<button class="%s"%s%s>%s%s</button>`, cls, confirm, r.disabled(), actionIcon(pa), esc(pa))
	}
	r.w(`</div>`)
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

// MinStepperSteps is how many user steps of one flow a page must host for
// the page to show the flow's steps in order.
const MinStepperSteps = 2

// flowSteps shows the user steps of the flow the page hosts most of: the
// steps done here are marked, the others lead to their pages.
func (r *render) flowSteps() {
	if r.centered || formLike(r.page.Pattern) {
		return
	}
	var walk *core.FlowWalk
	most := 0
	for i, w := range r.doc.Walks {
		here, user := 0, 0
		for _, st := range w.Steps {
			if !st.System {
				user++
				if st.Page == r.page.ID {
					here++
				}
			}
		}
		if here >= MinStepperSteps && user >= 3 && here > most {
			walk, most = &r.doc.Walks[i], here
		}
	}
	if walk == nil {
		return
	}
	r.f(`<nav class="fk-flow" aria-label="Alur %s" data-foundry-flow="%s"><span class="fk-flow-name">%s</span><ol>`,
		esc(walk.Name), esc(walk.ID), esc(walk.Name))
	n, first := 0, true
	for _, st := range walk.Steps {
		if st.System {
			continue
		}
		n++
		label := fmt.Sprintf(`<span class="fk-flow-n">%d</span>%s`, n, esc(core.StepAction(st.Text)))
		cls := ""
		if st.Decision {
			cls = "dec"
		}
		switch {
		case st.Page == r.page.ID:
			cls += " here"
			if first {
				cls, first = cls+" on", false
			}
			r.f(`<li class="%s" title="%s">%s</li>`, strings.TrimSpace(cls), esc(st.Text), label)
		case st.Page != "":
			r.f(`<li class="%s" title="%s"><a href="%s">%s</a></li>`, cls, esc(st.Text), esc(r.doc.href(st.Page)), label)
		default:
			r.f(`<li class="%s" title="%s">%s</li>`, cls, esc(st.Text), label)
		}
	}
	r.w(`</ol></nav>`)
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
