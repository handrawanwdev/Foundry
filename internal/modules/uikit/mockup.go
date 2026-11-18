package uikit

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A mockup is a static, self-contained HTML page rendered from the design:
// the tokens become CSS variables, the page pattern's template places each
// section in its slot, and every section carries data-foundry-section so
// the implementation and the visual review can be matched to it. Mockups
// show structure, hierarchy, density and tokens with sample data; they are
// the reference the coding agent builds from, not pixel-final art.

var (
	ErrNoDesign = errors.New("rancangan UI belum berisi halaman")
	ErrNoPage   = errors.New("halaman tidak ada di layout spec")
)

// Doc is what a mockup is rendered from.
type Doc struct {
	Name   string
	Rules  *core.DesignRules
	Layout *core.LayoutSpec
	// Walks are the use cases placed on the pages: they lead the user from
	// page to page and order the steps a page hosts.
	Walks []core.FlowWalk
	// Link is the href of another page's mockup; nil links nowhere.
	Link func(pageID string) string
	// Project is the source of lifecycles and roles; nil for a pattern
	// reference. Sim is the prototype role and document status chosen in
	// the URL. Self is this page's mockup URL (form action); empty on a
	// static export, which cannot change status.
	Project *core.Project
	Sim     Sim
	Self    string
	// trans are the project's lifecycle transitions by page, worked out
	// once for every page DocOf renders.
	trans map[string][]core.PageTransition
}

// DocOf is the design of a project.
func DocOf(p *core.Project) Doc {
	d := Doc{Name: p.Name, Project: p}
	if p.UI != nil {
		d.Rules, d.Layout = p.UI.Rules, p.UI.Layout
		d.Walks = core.FlowWalks(p, p.UI.Layout)
		d.trans = p.TransitionsByPage(p.UI.Layout)
	}
	return d
}

func (d Doc) page(id string) (*core.LayoutPage, error) {
	if d.Layout == nil || len(d.Layout.Pages) == 0 {
		return nil, ErrNoDesign
	}
	for i := range d.Layout.Pages {
		if d.Layout.Pages[i].ID == id {
			return &d.Layout.Pages[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrNoPage, id)
}

func (d Doc) shell(id string) *core.LayoutShell {
	if d.Layout == nil || id == "" {
		return nil
	}
	for i := range d.Layout.Shells {
		if d.Layout.Shells[i].ID == id {
			return &d.Layout.Shells[i]
		}
	}
	return nil
}

func (d Doc) href(id string) string {
	h := "#"
	if d.Link != nil {
		h = d.Link(id)
	}
	return WithSim(h, d.Sim.Nav())
}

// hrefAt is a prototype navigation that arrives on id in status, flashing
// feedback. Static exports ignore the query; the live mockup re-renders.
func (d Doc) hrefAt(id, status, feedback string) string {
	sim := d.Sim.Nav()
	sim.Status, sim.Feedback = status, feedback
	h := "#"
	if d.Link != nil {
		h = d.Link(id)
	}
	return WithSim(h, sim)
}

// hrefSim is a prototype navigation to id with sim.
func (d Doc) hrefSim(id string, sim Sim) string {
	h := "#"
	if d.Link != nil {
		h = d.Link(id)
	}
	return WithSim(h, sim)
}

func (d Doc) live() bool { return strings.TrimSpace(d.Self) != "" }

// RenderPage renders one page of the design in a state ("" or "default"
// is the populated page; loading, empty, error, ... show that state).
func (k *Kit) RenderPage(d Doc, id, state string) (string, error) {
	pg, err := d.page(id)
	if err != nil {
		return "", err
	}
	state = strings.TrimSpace(state)
	if state == "default" {
		state = ""
	}
	if state != "" && !slices.Contains(core.UIStates, state) {
		return "", fmt.Errorf("state %q tidak dikenal (%s)", state, strings.Join(core.UIStates, ", "))
	}
	r := &render{kit: k, doc: d, page: pg, state: state}
	return r.html(), nil
}

// PageHash identifies the populated mockup of a page: an approval holds
// while the hash does.
func (k *Kit) PageHash(d Doc, id string) string {
	d.Link, d.Sim, d.Self = nil, Sim{}, ""
	out, err := k.RenderPage(d, id, "")
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(k.Version + "\n" + out))
	return hex.EncodeToString(sum[:])[:16]
}

// RenderReference renders the example page of a pattern template with the
// default preset: the reference of what the pattern looks like.
func (k *Kit) RenderReference(pattern string) (string, error) {
	p, ok := k.Pattern(pattern)
	if !ok || p.Example == nil {
		return "", fmt.Errorf("pattern %q tidak punya contoh", pattern)
	}
	ex := *p.Example
	d := Doc{Name: "Contoh " + p.Name, Rules: &core.DesignRules{Preset: DefaultPreset},
		Layout: &core.LayoutSpec{Pages: []core.LayoutPage{ex}}}
	return k.RenderPage(d, ex.ID, "")
}

// MockupStatus is a page's mockup against the user's approval.
type MockupStatus struct {
	Page       string `json:"page"`
	Name       string `json:"name"`
	Platform   string `json:"platform"`
	Pattern    string `json:"pattern"`
	Template   bool   `json:"template"` // the pattern has a kit template
	Hash       string `json:"hash"`
	Status     string `json:"status"` // approved | changed | pending
	ApprovedAt string `json:"approved_at,omitempty"`
	// Frames are the moments of the page the workspace lays out.
	Frames []Frame `json:"frames,omitempty"`
}

// Mockup approval statuses.
const (
	MockupApproved = "approved"
	MockupChanged  = "changed"
	MockupPending  = "pending"
)

// Statuses lists every page's mockup with its approval.
func (k *Kit) Statuses(p *core.Project) []MockupStatus {
	if p.UI == nil || p.UI.Layout == nil {
		return []MockupStatus{}
	}
	d := DocOf(p)
	out := make([]MockupStatus, 0, len(p.UI.Layout.Pages))
	for _, pg := range p.UI.Layout.Pages {
		_, tmpl := k.Pattern(pg.Pattern)
		s := MockupStatus{Page: pg.ID, Name: pg.Name, Platform: pg.Platform, Pattern: pg.Pattern, Template: tmpl,
			Hash: k.PageHash(d, pg.ID), Status: MockupPending}
		if a, ok := p.UI.Mockups[pg.ID]; ok {
			s.ApprovedAt = a.ApprovedAt.UTC().Format("2006-01-02T15:04:05Z")
			s.Status = MockupChanged
			if a.Hash == s.Hash {
				s.Status = MockupApproved
			}
		}
		out = append(out, s)
	}
	return out
}

// WithFrames adds each page's frames to list: the mockup workspace lays
// them out; an agent reading the design has no use for them.
func (k *Kit) WithFrames(p *core.Project, list []MockupStatus) []MockupStatus {
	d := DocOf(p)
	for i := range list {
		list[i].Frames = k.Frames(d, list[i].Page)
	}
	return list
}

// RenderIndex is the page list of the exported mockups.
func (k *Kit) RenderIndex(d Doc, statuses []MockupStatus) string {
	var b strings.Builder
	tokens := k.Resolve(d.Rules)
	b.WriteString(`<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>`)
	b.WriteString(esc("Mockup · " + d.Name))
	b.WriteString(`</title><style>` + RootCSS(tokens) + k.CSS + `</style></head><body class="fk-frame-plain"><main class="fk-main"><div class="fk-page">`)
	b.WriteString(`<header class="fk-head"><div><p class="fk-crumb">Foundry · mockup halaman</p><h1>` + esc(d.Name) + `</h1>`)
	b.WriteString(`<p>Acuan struktur, hierarki, kepadatan, dan token tiap halaman. Data di dalamnya contoh.</p></div></header>`)
	b.WriteString(`<section class="fk-sec fk-flush"><h2>Halaman</h2><div class="fk-table-wrap"><table class="fk-table"><thead><tr><th>Halaman</th><th>Platform</th><th>Pattern</th><th>Persetujuan</th></tr></thead><tbody>`)
	for _, s := range statuses {
		label, cls := "Belum disetujui", "warning"
		switch s.Status {
		case MockupApproved:
			label, cls = "Disetujui", "success"
		case MockupChanged:
			label, cls = "Berubah sejak disetujui", "error"
		}
		fmt.Fprintf(&b, `<tr><td><a class="fk-link" href="%s">%s</a> <span class="fk-meta">%s</span></td><td>%s</td><td>%s</td><td><span class="fk-badge fk-badge-%s">%s</span></td></tr>`,
			esc(d.href(s.Page)), esc(s.Name), esc(s.Page), esc(s.Platform), esc(s.Pattern), cls, label)
	}
	b.WriteString(`</tbody></table></div></section>`)
	writeWalks(&b, d)
	b.WriteString(`</div></main></body></html>`)
	return b.String()
}

func esc(s string) string { return html.EscapeString(s) }

// render is one page being rendered.
type render struct {
	// inForm is set while the controls of an open <form> are drawn: a
	// button there submits it, it leads nowhere else.
	inForm bool
	// curSec is the id of the section being drawn.
	curSec string
	// cmdkItems are the pages the global search lists; set once the top
	// bar drew its search.
	cmdkItems []navItem
	// account is set once the top bar drew the account menu: its
	// dialogs follow the page.
	account bool
	// secLinks are the links of the section being drawn (sectionLinks).
	secLinks []secLink
	kit      *Kit
	doc      Doc
	page     *core.LayoutPage
	state    string
	centered bool
	// panelOpen is set while a data panel renders; panelHead is the
	// collection whose header the panel shows.
	panelOpen bool
	panelHead string
	// colls are the page's collections and where their controls are.
	colls map[string]core.Collection
	// docActs are the document transitions the header already offers;
	// the page body does not repeat them.
	docActs []string
	// dialogs are the sections the page opens as dialogs, drawn after
	// its content.
	dialogs []*dialog
	// forms are the forms drawn, asks the confirmations a control opens:
	// the frames of the page.
	forms []formRec
	// fplan is the page's form, mapped once (plan).
	fplan *formPlan
	// heroUsed marks the page fields the record's header shows; headActs
	// are the actions the page header offers.
	heroUsed []bool
	headActs []string
	asks     []ask
	// steppers and collections count the page's steppers and tables/lists;
	// contentIssues are what they show wrongly, for the quality gate.
	steppers, collections int
	contentIssues         []string
	b                     strings.Builder
}

func (r *render) w(s string)                { r.b.WriteString(s) }
func (r *render) f(format string, a ...any) { fmt.Fprintf(&r.b, format, a...) }

func (r *render) html() string {
	pg := r.page
	tokens := r.kit.Resolve(r.doc.Rules)
	pat, hasPat := r.kit.Pattern(pg.Pattern)
	hero := r.heroSection() != ""
	secs := slices.DeleteFunc(r.kit.AssignSlots(pg.Pattern, pg.Sections), func(s core.Section) bool {
		if core.DialogSection(pg.Pattern, s) {
			r.dialogs = append(r.dialogs, parseDialog(s))
			return true
		}
		// A record's buttons stand beside its title.
		return hero && actionOnly(s) && r.state == ""
	})
	placed := *pg
	placed.Sections = secs
	r.colls = map[string]core.Collection{}
	for _, c := range core.Collections(&placed) {
		r.colls[c.Section] = c
	}
	shell := r.doc.shell(pg.Shell)
	tablet := breakpoint(tokens, 768)

	r.w(`<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>`)
	r.w(esc(r.formTitle() + " · " + r.doc.Name))
	r.w(`</title><style>`)
	r.w(RootCSS(tokens))
	r.w(r.kit.CSS)
	body := ""
	if hasPat {
		body = r.panelBody(pat, secs)
	}
	r.w(gridCSS(pat, hasPat, secs, body, tablet))
	r.f(`@media (max-width:%dpx){%s}`, tablet-1, mobileCSS)
	r.w(`</style></head>`)

	centered := hasPat && pat.Frame == "centered" && (shell == nil || shell.Pattern == "auth")
	switch {
	case centered:
		r.w(`<body>`)
		r.simBanner()
		r.w(`<div class="fk-centered">`)
		r.centered = true
		r.pageBody(pat, hasPat, secs)
		r.w(`</div>`)
	case shell != nil && (shell.Pattern == "sidebar" || shell.Pattern == "navbar"):
		r.w(`<body>`)
		r.simBanner()
		r.shell(shell, pat, hasPat, secs)
	default:
		r.w(`<body class="fk-frame-plain">`)
		r.simBanner()
		r.w(`<main class="fk-main">`)
		r.pageBody(pat, hasPat, secs)
		r.w(`</main>`)
	}
	state := "default"
	if r.state != "" {
		state = r.state
	}
	r.accountDialogs()
	r.commandPalette()
	r.f(`<div class="fk-mark" aria-hidden="true">mockup · %s · %s · data contoh</div></body></html>`, esc(pg.ID), esc(state))
	return r.b.String()
}

// mobileCSS applies below the tablet breakpoint: the navigation folds into
// the menu button and table rows become label/value cards.
const mobileCSS = `.fk-side{display:none}.fk-burger{display:inline-grid}.fk-top-brand{display:flex;flex:1;min-width:0;overflow:hidden}.fk-top-brand .fk-brand{min-width:0;overflow:hidden;text-overflow:ellipsis}.fk-main{padding:var(--space-md)}.fk-top{padding:0 var(--space-md)}` +
	`.fk-top .fk-search{flex:none;width:var(--size-control);padding:0;justify-content:center}.fk-top .fk-search>span{display:none}.fk-top-nav,.fk-user-text,.fk-user>.fk-i,.fk-top-end .fk-chip{display:none}.fk-head{align-items:flex-start}` +
	`.fk-head .fk-actions{width:100%}.fk-head .fk-actions .fk-btn{flex:1}.fk-tools-wrap,.fk-panel>section.fk-tools-wrap{padding:var(--space-sm) var(--space-md)}` +
	`.fk-table thead{display:none}.fk-table,.fk-table tbody,.fk-table tr{display:block;width:100%}` +
	`.fk-table tr{padding:var(--space-sm) var(--space-md);border-bottom:1px solid var(--border-default)}.fk-table tr:last-child{border-bottom:0}` +
	`.fk-table td,.fk-table td:first-child,.fk-table td:last-child{display:flex;justify-content:space-between;align-items:center;gap:var(--space-md);height:auto;padding:4px 0;border:0;white-space:normal;text-align:right}` +
	`.fk-table td::before{content:attr(data-label);font:var(--type-caption);color:var(--text-muted);text-align:left}` +
	`.fk-table td.fk-act{position:static;box-shadow:none;background:none;justify-content:flex-end;width:auto;margin-top:var(--space-xs);padding-top:var(--space-sm);border-top:1px solid var(--border-default)}.fk-table td.fk-act::before{content:none}` +
	`.fk-row-acts{width:100%}.fk-row-btn{flex:1;min-height:var(--size-control)}.fk-row-more{width:var(--size-control);height:var(--size-control);border-color:var(--border-default)}` +
	`.fk-detail-actions>.fk-btn{flex:1 1 auto}.fk-actions-end{margin-left:0;width:100%}.fk-actions-end>.fk-btn{flex:1}` +
	`.fk-pager{padding:var(--space-sm) var(--space-md)}.fk-pager-size{display:none}.fk-pages .fk-pg:not(.fk-nav-pg):not(.on){display:none}.fk-gap{display:none}` +
	`.fk-sec-head{padding:var(--space-md)}.fk-list li{padding:var(--space-sm) var(--space-md)}` +
	`.fk-form-bar .fk-form-note{display:none}.fk-form-bar>.fk-btn{flex:1}` +
	`.fk-hero-acts{width:100%;justify-content:stretch}.fk-hero-acts>.fk-btn{flex:1}.fk-hero-acts>.fk-actions-end{width:100%;margin:0}.fk-hero-acts>.fk-actions-end>.fk-btn{flex:1}.fk-feed li{flex-direction:column;gap:4px}.fk-feed-when{text-align:left}.fk-figures{grid-template-columns:repeat(2,minmax(0,1fr))}`

func breakpoint(t core.DesignTokens, def int) int {
	for _, tok := range t.Breakpoints {
		if tok.Name == "breakpoint.tablet" {
			if n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(tok.Value), "px")); err == nil && n > 200 {
				return n
			}
		}
	}
	return def
}

var cssIdent = regexp.MustCompile(`[^a-z0-9-]`)

func ident(s string) string { return cssIdent.ReplaceAllString(strings.ToLower(s), "-") }

// gridCSS lays the slots of the pattern out: the template's desktop areas
// with empty slots collapsed, then one column in slot order below the
// tablet breakpoint. secs are the sections placed in the grid itself (not
// inside a data panel).
func gridCSS(pat *Pattern, ok bool, secs []core.Section, body string, tablet int) string {
	if !ok || pat.Shell || len(pat.Areas) == 0 {
		return ""
	}
	used := map[string]bool{}
	for _, s := range secs {
		used[s.Slot] = true
	}
	// A data panel fills the area of every slot it holds.
	inPanel := func(id string) bool { return body != "" && id != body && slices.Contains(pat.Panel, id) }
	grid := make([][]string, len(pat.Areas))
	for i, row := range pat.Areas {
		grid[i] = slices.Clone(row)
		for j, c := range row {
			if inPanel(c) {
				grid[i][j] = body
			}
		}
	}
	rows := collapse(grid, used, false)
	if !rectangular(rows) {
		rows = collapse(grid, used, true)
	}
	if !rectangular(rows) {
		rows = nil
		for _, sl := range pat.Slots {
			if used[sl.ID] && !inPanel(sl.ID) {
				rows = append(rows, []string{sl.ID})
			}
		}
	}
	if len(rows) == 0 {
		return ""
	}
	single := true
	for _, row := range rows {
		for _, c := range row {
			if c != row[0] {
				single = false
			}
		}
	}
	cols := pat.Columns
	if single || cols == "" {
		cols = "minmax(0,1fr)"
		for i := range rows {
			rows[i] = rows[i][:1]
		}
	}
	var areas []string
	for _, row := range rows {
		var names []string
		for _, c := range row {
			names = append(names, ident(c))
		}
		areas = append(areas, `"`+strings.Join(names, " ")+`"`)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `.fk-grid{grid-template-columns:%s;grid-template-areas:%s}`, unsafeCSS.ReplaceAllString(cols, " "), strings.Join(areas, " "))
	var stack []string
	for _, s := range pat.Slots {
		if used[s.ID] && !inPanel(s.ID) {
			fmt.Fprintf(&b, `.fk-a-%s{grid-area:%s}`, ident(s.ID), ident(s.ID))
			stack = append(stack, `"`+ident(s.ID)+`"`)
		}
	}
	fmt.Fprintf(&b, `@media (max-width:%dpx){.fk-grid{grid-template-columns:minmax(0,1fr);grid-template-areas:%s}}`, tablet-1, strings.Join(stack, " "))
	return b.String()
}

// shell wraps the page in its app frame: navigation, topbar, content.
// collapse gives every empty slot's cell to a filled neighbour: one in the
// same row (vertical: the same column first), and drops rows left empty
// or repeating the row above.
func collapse(areas [][]string, used map[string]bool, vertical bool) [][]string {
	var rows [][]string
	for r, row := range areas {
		cells := slices.Clone(row)
		for i, c := range row {
			if used[c] {
				continue
			}
			cells[i] = ""
			if vertical {
				for k := r - 1; k >= 0 && cells[i] == ""; k-- {
					if used[areas[k][i]] {
						cells[i] = areas[k][i]
					}
				}
				for k := r + 1; k < len(areas) && cells[i] == ""; k++ {
					if used[areas[k][i]] {
						cells[i] = areas[k][i]
					}
				}
			}
			for j := i - 1; j >= 0 && cells[i] == ""; j-- {
				if used[row[j]] {
					cells[i] = row[j]
				}
			}
			for j := i + 1; j < len(row) && cells[i] == ""; j++ {
				if used[row[j]] {
					cells[i] = row[j]
				}
			}
		}
		if cells[0] == "" || len(rows) > 0 && slices.Equal(rows[len(rows)-1], cells) {
			continue
		}
		rows = append(rows, cells)
	}
	return rows
}

// rectangular reports whether every name of the areas covers one
// rectangle, as grid-template-areas requires.
func rectangular(rows [][]string) bool {
	type box struct{ r0, r1, c0, c1, n int }
	boxes := map[string]*box{}
	for r, row := range rows {
		for c, name := range row {
			b := boxes[name]
			if b == nil {
				b = &box{r, r, c, c, 0}
				boxes[name] = b
			}
			b.r0, b.r1, b.c0, b.c1 = min(b.r0, r), max(b.r1, r), min(b.c0, c), max(b.c1, c)
			b.n++
		}
	}
	for _, b := range boxes {
		if b.n != (b.r1-b.r0+1)*(b.c1-b.c0+1) {
			return false
		}
	}
	return true
}

func (r *render) shell(sh *core.LayoutShell, pat *Pattern, ok bool, secs []core.Section) {
	items := r.navItems(sh)
	brand := r.brand()
	if sh.Pattern == "navbar" {
		r.w(`<div class="fk-body" data-foundry-shell="` + esc(sh.ID) + `"><header class="fk-top">`)
		r.drawer(items)
		r.w(brand + `<nav class="fk-top-nav" aria-label="Navigasi utama">`)
		for _, it := range items {
			r.navLink(it)
		}
		r.w(`</nav>`)
		r.topEnd(sh)
		r.w(`</header><main class="fk-main">`)
		r.pageBody(pat, ok, secs)
		r.w(`</main></div>`)
		return
	}
	r.w(`<div class="fk-shell" data-foundry-shell="` + esc(sh.ID) + `"><aside class="fk-side">` + brand + `<nav class="fk-nav" aria-label="Navigasi utama">`)
	group := ""
	for _, it := range items {
		if it.group != group && it.group != "" {
			group = it.group
			r.w(`<div class="fk-nav-group">` + esc(group) + `</div>`)
		}
		r.navLink(it)
	}
	r.w(`</nav></aside><div class="fk-body"><header class="fk-top">`)
	r.drawer(items)
	r.w(`<span class="fk-top-brand">` + r.brand() + `</span>`)
	if regionMentions(sh, "cari", "search", "pencarian") || r.signedIn() {
		r.searchButton(items)
	}
	r.topEnd(sh)
	r.w(`</header><main class="fk-main">`)
	r.pageBody(pat, ok, secs)
	r.w(`</main></div></div>`)
}

// drawer is the menu button of a small screen and the navigation it
// slides in.
func (r *render) drawer(items []navItem) {
	r.w(`<button class="fk-icon-btn fk-burger" aria-label="Menu" aria-haspopup="dialog" popovertarget="fk-drawer">` + icon("menu") + `</button>`)
	r.w(`<div id="fk-drawer" popover class="fk-drawer" role="dialog" aria-label="Navigasi"><div class="fk-drawer-head">` + r.brand() +
		`<button class="fk-icon-btn" aria-label="Tutup" popovertarget="fk-drawer" popovertargetaction="hide">` + icon("close") + `</button></div><nav class="fk-nav" aria-label="Navigasi utama">`)
	group := ""
	for _, it := range items {
		if it.group != group && it.group != "" {
			group = it.group
			r.w(`<div class="fk-nav-group">` + esc(group) + `</div>`)
		}
		r.navLink(it)
	}
	r.w(`</nav></div>`)
}

func (r *render) brand() string {
	return fmt.Sprintf(`<div class="fk-brand" title="%s"><span class="fk-logo">%s</span><span class="fk-brand-name">%s</span></div>`, esc(r.doc.Name), esc(strings.ToUpper(string(firstRune(r.doc.Name)))), esc(r.doc.Name))
}

// topEnd is the right side of the topbar: what the shell's header region
// names (a context selector, a status indicator, notifications), then the
// user menu.
func (r *render) topEnd(sh *core.LayoutShell) {
	r.w(`<div class="fk-top-end">`)
	if sh.Pattern == "navbar" && (regionMentions(sh, "cari", "search", "pencarian") || r.signedIn()) {
		r.cmdkItems = r.navItems(sh)
		r.w(`<button class="fk-icon-btn" aria-label="Cari" aria-haspopup="dialog" aria-keyshortcuts="Meta+K Control+K" popovertarget="fk-cmdk">` + icon("search") + `</button>`)
	}
	for _, it := range topbarItems(sh) {
		l := strings.ToLower(it)
		switch {
		case has(l, "indikator", "online", "offline", "koneksi", "sinkron", "status"):
			r.w(`<span class="fk-badge fk-badge-success">Online</span>`)
		case has(l, "notifikasi", "notif", "pemberitahuan", "inbox"):
			r.w(`<button class="fk-icon-btn" aria-label="Notifikasi">` + icon("bell") + `<span class="fk-ping"></span></button>`)
		case has(l, "pilih", "lokasi", "cabang", "tenant", "organisasi", "workspace", "proyek", "outlet", "gudang", "periode"):
			noun := contextNoun(l)
			r.w(`<span class="fk-chip fk-chip-solid">` + icon(iconFor(noun)) + `Semua ` + esc(noun) + icon("chevron") + `</span>`)
		}
	}
	r.userMenu()
	r.w(`</div>`)
}

// topbarItems are the content of the shell's header region.
func topbarItems(sh *core.LayoutShell) []string {
	for _, reg := range sh.Regions {
		t := strings.ToLower(reg.ID + " " + reg.Name + " " + reg.Slot)
		if has(t, "top", "header", "atas", "bar") && !has(t, "side", "nav") {
			return reg.Content
		}
	}
	return nil
}

// contextNoun is what a context selector chooses: "Pilihan lokasi sesuai
// lingkup" chooses a lokasi.
func contextNoun(l string) string {
	for _, w := range []string{"lokasi", "cabang", "tenant", "organisasi", "workspace", "proyek", "outlet", "gudang", "periode"} {
		if strings.Contains(l, w) {
			return w
		}
	}
	return "konteks"
}

func initials(s string) string {
	var out []rune
	for _, f := range strings.Fields(s) {
		out = append(out, []rune(strings.ToUpper(f))[0])
		if len(out) == 2 {
			break
		}
	}
	if len(out) == 0 {
		return "U"
	}
	return string(out)
}

func firstRune(s string) rune {
	for _, r := range strings.TrimSpace(s) {
		return r
	}
	return 'F'
}

func regionMentions(sh *core.LayoutShell, words ...string) bool {
	for _, reg := range sh.Regions {
		text := strings.ToLower(reg.Name + " " + strings.Join(reg.Content, " "))
		for _, w := range words {
			if strings.Contains(text, w) {
				return true
			}
		}
	}
	return false
}

type navItem struct{ group, label, page string }

// navItems are the shell's navigation: the design's navigation items whose
// pages sit in this shell, else the shell's list pages.
func (r *render) navItems(sh *core.LayoutShell) []navItem {
	l := r.doc.Layout
	inShell := map[string]bool{}
	for _, pg := range l.Pages {
		if pg.Shell == sh.ID {
			inShell[pg.ID] = true
		}
	}
	var out []navItem
	for _, g := range l.Navigation {
		for _, it := range g.Items {
			if inShell[it.Page] {
				name := g.Name
				if len(l.Navigation) == 1 {
					name = ""
				}
				out = append(out, navItem{name, it.Label, it.Page})
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	nested := map[string]bool{"detail": true, "form": true, "wizard": true, "modal": true, "auth": true}
	for _, pg := range l.Pages {
		if inShell[pg.ID] && !nested[pg.Pattern] {
			out = append(out, navItem{"", pg.Name, pg.ID})
		}
	}
	return out
}

func (r *render) navLink(it navItem) {
	cls := ""
	if it.page == r.page.ID || r.linksTo(it.page) {
		cls = ` class="on" aria-current="page"`
	}
	r.f(`<a href="%s"%s>%s%s</a>`, esc(r.doc.href(it.page)), cls, icon(iconFor(it.label+" "+it.page)), esc(it.label))
}

// linksTo reports whether the nav item's page leads to this page (a form
// or detail page keeps its list page highlighted).
func (r *render) linksTo(page string) bool {
	if page == r.page.ID {
		return true
	}
	if !r.inNav(r.page.ID) {
		// One menu item is where a page outside the menu sits.
		if parent := core.ParentPage(r.doc.Layout, r.page.ID); parent != "" {
			return parent == page
		}
	}
	for _, pg := range r.doc.Layout.Pages {
		if pg.ID != page {
			continue
		}
		for _, ln := range pg.Links {
			if ln.To == r.page.ID && !r.inNav(r.page.ID) {
				return true
			}
		}
	}
	return false
}

func (r *render) inNav(page string) bool {
	for _, g := range r.doc.Layout.Navigation {
		for _, it := range g.Items {
			if it.Page == page {
				return true
			}
		}
	}
	return false
}

// pageBody is the page header and the slot grid.
func (r *render) pageBody(pat *Pattern, ok bool, secs []core.Section) {
	pg := r.page
	cls := "fk-page"
	if r.formPage() {
		cls += " fk-form-page"
	}
	r.f(`<div class="%s" data-foundry-page="%s">`, cls, esc(pg.ID))
	if r.centered {
		r.w(r.brand())
	}
	r.w(`<header class="fk-head"><div>`)
	if crumb := r.crumb(); len(crumb) > 0 && r.doc.shell(pg.Shell) != nil && !(ok && pat.Frame == "centered") {
		r.w(`<p class="fk-crumb">`)
		for i, c := range crumb {
			if i > 0 {
				r.w(icon("right"))
			}
			if c.page != "" {
				r.f(`<a href="%s" data-foundry-link="%s">%s</a>`, esc(r.doc.href(c.page)), esc(c.page), esc(c.label))
				continue
			}
			r.w(`<span>` + esc(c.label) + `</span>`)
		}
		r.w(`</p>`)
	}
	r.w(`<h1>` + esc(r.formTitle()) + `</h1>`)
	if lc, st := r.simLifecycle(), r.simState(); lc != nil && st != nil {
		r.f(`<p class="fk-note"><span class="fk-badge" data-foundry-sim="status">%s</span> %s</p>`, esc(lc.StateLabel(st.ID)), esc(lc.Entity))
	}
	if r.doc.Sim.Role != "" && !core.RoleAllowed(pg.Role, r.doc.Sim.Role) {
		r.f(`<p class="fk-note" data-foundry-sim="role">Peran %s tidak memakai halaman ini.</p>`, esc(r.doc.Sim.Role))
	}
	if pg.Purpose != "" {
		r.w(`<p>` + esc(pg.Purpose) + `</p>`)
	}
	r.w(`</div>`)
	r.headActions()
	r.w(`</header>`)
	if r.state == "success" {
		r.toast("ok", "", r.stateText(nil, "success"))
	}
	if r.state == "disabled" {
		r.w(`<p class="fk-note">` + esc(r.stateText(nil, "disabled")) + `</p>`)
	}
	if r.state == "permission_denied" {
		r.statePanel("permission_denied", nil)
		r.w(`</div>`)
		return
	}
	if !ok || pat.Shell || len(pat.Areas) == 0 {
		r.w(`<div class="fk-grid">`)
		for i := range secs {
			r.section(secs[i], Slot{Kind: guessKind(secs[i])}, false)
		}
		r.w(`</div>`)
		r.pageDialogs()
		r.w(`</div>`)
		return
	}
	body := r.panelBody(pat, secs)
	r.w(`<div class="fk-grid">`)
	for _, sl := range pat.Slots {
		in := inSlot(secs, sl.ID)
		if len(in) == 0 || body != "" && sl.ID != body && slices.Contains(pat.Panel, sl.ID) {
			continue
		}
		r.f(`<div class="fk-area fk-a-%s">`, ident(sl.ID))
		if sl.ID == body {
			r.panel(pat, secs, sl, in)
		} else {
			for _, s := range in {
				r.section(s, sl, true)
			}
		}
		r.w(`</div>`)
	}
	r.w(`</div>`)
	r.pageDialogs()
	r.w(`</div>`)
}

func inSlot(secs []core.Section, slot string) []core.Section {
	var in []core.Section
	for _, s := range secs {
		if s.Slot == slot {
			in = append(in, s)
		}
	}
	return in
}

// panelBody is the collection slot of the pattern's data panel when it
// holds sections; "" when the page has no panel.
func (r *render) panelBody(pat *Pattern, secs []core.Section) string {
	for _, id := range pat.Panel {
		sl, ok := pat.Slot(id)
		if ok && (sl.Kind == "table" || sl.Kind == "list" || sl.Kind == "cards") {
			for _, s := range inSlot(secs, id) {
				switch r.kindOf(s, sl) {
				case "table", "list", "cards":
					return id
				}
			}
			return ""
		}
	}
	return ""
}

// panel renders the data panel: the collection's header, the toolbar
// sections, the collection, then the pagination sections, in one card.
// Other sections of the collection slot (a form, a second table) follow
// the panel as cards of their own.
func (r *render) panel(pat *Pattern, secs []core.Section, body Slot, in []core.Section) {
	r.panelOpen = true
	var inside, after []core.Section
	coll := false
	for _, s := range in {
		k := r.kindOf(s, body)
		switch {
		case r.collectionKind(k) && !coll:
			coll = true
			inside = append(inside, s)
		case k == "tabbar" && !coll:
			inside = append(inside, s)
		default:
			after = append(after, s)
		}
	}
	r.w(`<div class="fk-panel">`)
	for _, s := range inside {
		if r.collectionKind(r.kindOf(s, body)) {
			r.collHead(s)
			r.panelHead = s.ID
			break
		}
	}
	for _, id := range pat.Panel {
		sl, _ := pat.Slot(id)
		list := inSlot(secs, id)
		if id == body.ID {
			list = inside
		}
		for _, s := range list {
			r.section(s, sl, true)
			if s.ID == r.panelHead {
				r.panelHead = ""
			}
		}
	}
	r.panelOpen, r.panelHead = false, ""
	r.w(`</div>`)
	for _, s := range after {
		r.section(s, body, true)
	}
}

// actionIcon is the icon of a primary action label.
func actionIcon(label string) string {
	l := strings.ToLower(label)
	switch {
	case has(l, "ubah", "edit", "perbarui", "update"):
		return icon("edit")
	case has(l, "hapus", "delete", "remove"):
		return icon("trash")
	case has(l, "detail", "lihat", "view"):
		return icon("eye")
	case has(l, "tambah", "buat", "baru", "add", "new", "create", "daftarkan", "catat"):
		return icon("plus")
	case has(l, "ekspor", "unduh", "export", "download"):
		return icon("download")
	}
	return ""
}

func formLike(pattern string) bool {
	switch pattern {
	case "form", "wizard", "settings", "auth":
		return true
	}
	return false
}

// crumbItem is a step of the breadcrumb; page is where it leads ("" for
// a menu group or the page itself).
type crumbItem struct{ label, page string }

// crumb is where the page sits: its menu group, or the page it is entered
// from (core.ParentPage) under that page's group.
func (r *render) crumb() []crumbItem {
	l := r.doc.Layout
	group := func(id string) string {
		if len(l.Navigation) < 2 {
			return ""
		}
		for _, g := range l.Navigation {
			for _, it := range g.Items {
				if it.Page == id {
					return g.Name
				}
			}
		}
		return ""
	}
	var out []crumbItem
	add := func(label, page string) {
		if label != "" {
			out = append(out, crumbItem{label, page})
		}
	}
	if parent := core.ParentPage(l, r.page.ID); parent != "" {
		pp, _ := r.doc.page(parent)
		if pp == nil {
			return nil
		}
		add(group(parent), "")
		add(pp.Name, parent)
		add(r.formTitle(), "")
		return out
	}
	if g := group(r.page.ID); g != "" {
		return []crumbItem{{g, ""}, {r.page.Name, ""}}
	}
	return nil
}

// parentHref leads back to the page this one is entered from; "" when
// there is none.
func (r *render) parentHref() string {
	if p := core.ParentPage(r.doc.Layout, r.page.ID); p != "" {
		return r.doc.href(p)
	}
	return ""
}

func (r *render) disabled() string {
	if r.state == "disabled" || r.state == "loading" {
		return " disabled"
	}
	return ""
}

// guessKind is the block of a section of a pattern without a template.
func guessKind(s core.Section) string {
	if k := sectionKinds[s.Pattern]; k != "" {
		return k
	}
	text := strings.ToLower(s.ID + " " + s.Name + " " + strings.Join(s.Content, " "))
	for _, c := range []struct{ kind, words string }{
		{"table", "tabel table"}, {"chart", "grafik chart tren"}, {"form", "form isian"},
		{"metrics", "kpi metrik ringkasan"}, {"list", "daftar list riwayat"}, {"actions", "tombol aksi"},
		{"toolbar", "filter cari search"}, {"steps", "langkah step"},
	} {
		for _, w := range strings.Fields(c.words) {
			if strings.Contains(text, w) {
				return c.kind
			}
		}
	}
	return "text"
}
