package uikit

import (
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// Every control of a mockup does something: it leads to a page, opens a
// dialog or a confirmation, or (on a live prototype) simulates the action
// and answers with its feedback. A control that does nothing on click
// reads as a broken prototype.

// viaPrefixes are the words a link's via starts with that name the control,
// not the action ("tombol Simpan").
var viaPrefixes = []string{"klik ", "tombol ", "tautan ", "link ", "aksi ", "button ", "menu ", "ikon "}

// viaActions are the actions a link's via names: "Tambah item pantau atau
// Ubah" names Tambah item pantau and Ubah.
func viaActions(via string) []string {
	r := strings.NewReplacer(" atau ", "|", " dan ", "|", " or ", "|", " and ", "|", ",", "|", "/", "|", ";", "|")
	var out []string
	for _, p := range strings.Split(r.Replace(" "+strings.TrimSpace(via)+" "), "|") {
		p = strings.Trim(strings.TrimSpace(p), `'"‘’“”`)
		for _, pre := range viaPrefixes {
			if len(p) > len(pre) && strings.EqualFold(p[:len(pre)], pre) {
				p = strings.TrimSpace(p[len(pre):])
				break
			}
		}
		if p = strings.Trim(strings.TrimSpace(actionName(p)), `'"‘’“”`); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// sameAction reports whether two action labels name the same action: the
// same words, or every word of each in the other by its root.
func sameAction(a, b string) bool {
	a, b = strings.TrimSpace(actionName(a)), strings.TrimSpace(actionName(b))
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(a, b) || core.StemsCovered(a, b) && core.StemsCovered(b, a)
}

// linkFor is the page this page's links lead to by action a: the link
// whose via names it ("Daftarkan sumber atau Cek kelayakan" leads Cek
// kelayakan to the form). "" when none, or several pages, do.
func (r *render) linkFor(a string) string {
	found := ""
	for _, ln := range r.page.Links {
		if ln.To == "" || ln.To == r.page.ID {
			continue
		}
		if _, err := r.doc.page(ln.To); err != nil {
			continue
		}
		if !slices.ContainsFunc(viaActions(ln.Via), func(v string) bool { return sameAction(v, a) }) {
			continue
		}
		if found != "" && found != ln.To {
			return ""
		}
		found = ln.To
	}
	return found
}

// viewVerbs open a record or a page to look at it.
var viewVerbs = []string{"buka", "lihat", "detail", "periksa", "tinjau", "tampilkan", "open", "view", "show", "inspect"}

// viewPage is the page an action that opens something leads to: the page
// named by what it opens ("Buka analisis" opens Detail Analisis), else the
// detail page this page's rows open ("Buka detail", "Lihat"). "" when the
// action opens nothing or no single page fits.
func (r *render) viewPage(a string) string {
	f := strings.Fields(strings.ToLower(actionName(a)))
	if len(f) == 0 || !slices.Contains(viewVerbs, f[0]) || r.doc.Layout == nil {
		return ""
	}
	var object []string
	for _, w := range f[1:] {
		if !slices.Contains([]string{"detail", "rincian", "data", "ini", "terkait", "terbaru", "terakhir", "lengkap", "selengkapnya", "baris"}, w) {
			object = append(object, w)
		}
	}
	if len(object) == 0 {
		if pg := core.DetailPage(r.doc.Layout, r.page.ID); pg != "" {
			return pg
		}
		if pg := r.featureDetail(); pg != "" {
			return pg
		}
		return r.linkedDetail()
	}
	what := strings.Join(object, " ")
	var detail, other []string
	for _, pg := range r.doc.Layout.Pages {
		if pg.ID == r.page.ID || pg.Pattern == "auth" || !core.StemsCovered(what, pg.Name+" "+pg.ID) {
			continue
		}
		if pg.Pattern == "detail" {
			detail = append(detail, pg.ID)
		} else {
			other = append(other, pg.ID)
		}
	}
	switch {
	case len(detail) == 1:
		return detail[0]
	case len(detail) == 0 && len(other) == 1:
		return other[0]
	}
	return ""
}

// featureDetail is the one detail page of this page's feature.
func (r *render) featureDetail() string {
	if r.page.FeatureID == "" {
		return ""
	}
	found := ""
	for _, pg := range r.doc.Layout.Pages {
		if pg.ID == r.page.ID || pg.FeatureID != r.page.FeatureID || pg.Pattern != "detail" {
			continue
		}
		if found != "" {
			return ""
		}
		found = pg.ID
	}
	return found
}

// leadsTo is the page action a of section s leads to: the page a link of
// this page names it by, the page its name names, the form it fills, or
// the page it opens. "" when it stays on the page.
func (r *render) leadsTo(a string, s *core.Section) string {
	if pg := r.linkFor(a); pg != "" {
		return pg
	}
	if s != nil {
		if pg := r.actionPage(a, *s); pg != "" {
			return pg
		}
	}
	if pg := r.formPageFor(a); pg != "" {
		return pg
	}
	if pg := r.verbForm(a); pg != "" {
		return pg
	}
	return r.viewPage(a)
}

// verbForm is the one form page in the product whose name starts with the
// verb of action a when that verb fills a form: "Catat masuk" on the
// dashboard leads to "Catat Mutasi". "" on a form page, or when none or
// several do.
func (r *render) verbForm(a string) string {
	f := strings.Fields(strings.ToLower(actionName(a)))
	if len(f) < 2 || formLike(r.page.Pattern) || r.doc.Layout == nil {
		return ""
	}
	found := ""
	for _, pg := range r.doc.Layout.Pages {
		w := strings.Fields(strings.ToLower(pg.Name))
		if pg.ID == r.page.ID || !formLike(pg.Pattern) || pg.Pattern == "auth" || len(w) == 0 || w[0] != f[0] {
			continue
		}
		if found != "" {
			return ""
		}
		found = pg.ID
	}
	return found
}

// sectionFor is the section of this page an action works in: the one its
// name or id names ("Buat permintaan" goes to the section Buat). "" when
// none, or several, do.
func (r *render) sectionFor(a string) string {
	found := ""
	for _, s := range r.page.Sections {
		if s.Slot == "toolbar" || s.Slot == "pagination" || s.Slot == "actions" || s.ID == "" || s.ID == r.curSec {
			continue
		}
		if !core.StemsCovered(s.Name, a) && !core.StemsCovered(strings.ReplaceAll(s.ID, "-", " "), a) {
			continue
		}
		if found != "" {
			return ""
		}
		found = s.ID
	}
	return found
}

// simHref is the live prototype's answer to action a taken on this page:
// its success (or the simulated failure), and the document's next status
// when a is one of its transitions. "" on a static mockup.
func (r *render) simHref(a string) string {
	if !r.doc.live() || r.state != "" {
		return ""
	}
	sim := r.outcome(a, "")
	if t := r.matchTransition(a); t != nil && sim.Feedback == "ok" {
		if dis, _, _ := r.actionState(a); dis {
			return ""
		}
		sim.Status = t.T.To
	}
	return r.doc.hrefSim(r.page.ID, sim)
}

// actionLink writes action a as a link when it leads somewhere: to a page,
// else (live) to its simulated answer. attrs are added to the element;
// false when a leads nowhere and the caller draws it as it would.
func (r *render) actionLink(a string, s *core.Section, cls, attrs, inner string) bool {
	if pg := r.leadsTo(a, s); pg != "" {
		r.f(`<a class="%s" href="%s" data-foundry-link="%s"%s>%s</a>`, cls, esc(r.actionHref(pg, a)), esc(pg), attrs, inner)
		return true
	}
	if sec := r.sectionFor(a); sec != "" && r.matchTransition(a) == nil {
		r.f(`<a class="%s" href="#sec-%s"%s>%s</a>`, cls, ident(sec), attrs, inner)
		return true
	}
	if h := r.simHref(a); h != "" {
		r.f(`<a class="%s" href="%s" data-foundry-sim="act"%s>%s</a>`, cls, esc(h), attrs, inner)
		return true
	}
	return false
}

// rowDefaults are the actions a bare "Aksi" column offers when the design
// names none: open the row's record, change it in its form.
func (r *render) rowDefaults() []string {
	var out []string
	if r.viewPage("Lihat detail") != "" {
		out = append(out, "Lihat detail")
	}
	if r.formPageFor("Ubah") != "" || r.linkFor("Ubah") != "" {
		out = append(out, "Ubah")
	}
	return out
}

// curTab is the open one of n tabs, 0-based: the URL's, else the first.
func (r *render) curTab(n int) int {
	if t := r.doc.Sim.Tab; t >= 1 && t <= n {
		return t - 1
	}
	return 0
}

// tabPage is the other page a tab names: one whose name is the tab's
// words ("Pengguna" on Organisasi leads to the Pengguna page). "" when the
// tab is a view of this page.
func (r *render) tabPage(t string) string {
	found := ""
	for _, pg := range r.doc.Layout.Pages {
		if pg.ID == r.page.ID || formLike(pg.Pattern) || !sameAction(t, pg.Name) {
			continue
		}
		if found != "" {
			return ""
		}
		found = pg.ID
	}
	return found
}

// tabNav draws a tab bar: a tab that is another page leads there; any
// other opens on this page (the URL carries it), the open one marked.
func (r *render) tabNav(cls, name string, tabs []string) {
	cur := r.curTab(len(tabs))
	r.f(`<nav class="%s" aria-label="%s">`, cls, esc(name))
	for i, t := range tabs {
		on := ""
		if i == cur {
			on = ` class="on" aria-current="page"`
		}
		if pg := r.tabPage(t); pg != "" && i != cur {
			r.f(`<a href="%s" data-foundry-link="%s">%s</a>`, esc(r.doc.href(pg)), esc(pg), esc(t))
			continue
		}
		sim := r.doc.Sim.Nav()
		sim.Tab = i + 1
		r.f(`<a href="%s"%s>%s</a>`, esc(r.doc.hrefSim(r.page.ID, sim)), on, esc(t))
	}
	r.w(`</nav>`)
}

// rowHref leads a row's action to page pg: a form opened from a row holds
// that row's record ("Cek kelayakan" checks this source), unless the
// action makes a new one.
func (r *render) rowHref(pg, a string) string {
	if to, _ := r.doc.page(pg); to != nil && formLike(to.Pattern) && to.Pattern != "auth" && !core.StartsCreate(actionName(a)) {
		sim := r.doc.Sim.Nav()
		sim.Fill = true
		return r.doc.hrefSim(pg, sim)
	}
	return r.actionHref(pg, a)
}

// linkedDetail is the one detail page this page's links lead to.
func (r *render) linkedDetail() string {
	found := ""
	for _, ln := range r.page.Links {
		pg, err := r.doc.page(ln.To)
		if err != nil || pg.ID == r.page.ID || pg.Pattern != "detail" || pg.ID == found {
			continue
		}
		if found != "" {
			return ""
		}
		found = pg.ID
	}
	return found
}
