package uikit

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// dataKinds are the blocks that show records; a loading, empty or error
// state replaces them.
var dataKinds = map[string]bool{"table": true, "list": true, "cards": true, "chart": true, "metrics": true,
	"detail": true, "activity": true, "summary": true}

// bareKinds render without a card frame.
var bareKinds = map[string]bool{"toolbar": true, "actions": true, "pagination": true, "metrics": true, "steps": true, "tabbar": true, "notice": true}

var defaultStates = map[string]string{
	"loading":           "Memuat data…",
	"empty":             "Belum ada data. Langkah berikutnya ditampilkan di sini.",
	"no_results":        "Tidak ada hasil yang cocok. Ubah kata kunci atau filter.",
	"error":             "Data gagal dimuat. Coba lagi.",
	"success":           "Berhasil disimpan.",
	"disabled":          "Aksi belum tersedia.",
	"permission_denied": "Anda tidak punya akses ke halaman ini.",
}

// kindOf is the block a section renders as in its slot.
func (r *render) kindOf(s core.Section, sl Slot) string {
	kind := sl.Kind
	fixed := bareKinds[kind] || kind == "steps" || kind == "tabs" || kind == "facets"
	if k := sectionKinds[s.Pattern]; k != "" && !fixed {
		kind = k
	} else if k := core.ContentKind(s); k != "" && k != "metrics" && (!fixed || kind == "actions" && dataKinds[k]) && !(k == "list" && kind == "activity") && !(k == "list" && tableSection(s, sl)) {
		kind = k // the section names its block: "Tabel …" in a chart slot is a table
	}
	if kind == "" {
		kind = guessKind(s)
	}
	if _, ok := r.colls[s.ID]; ok && kind != "table" && kind != "list" && kind != "cards" {
		kind = "list" // a list of records in an activity slot still pages
		if core.ContentKind(s) == "table" {
			kind = "table"
		}
	}
	if (core.ContentKind(s) == "tabs" || s.Pattern == "tabs") && sl.Kind != "tabs" && sl.Kind != "nav" {
		kind = "tabbar"
	}
	if kind == "form" || kind == "text" {
		if has(strings.ToLower(s.Name), "galat", "error", "kesalahan", "gagal") {
			kind = "notice"
		}
	}
	// A section of a form page built from display components only (a
	// StatCard of the stock in context) shows values; it takes no input.
	if kind == "form" && !takesInput(s) {
		kind = "detail"
	}
	return kind
}

// tableSection reports a section drawn as a table: its slot is a table or
// it names a table component. "Daftar item" in such a section is a table
// of the items, not a list.
func tableSection(s core.Section, sl Slot) bool {
	return sl.Kind == "table" || slices.ContainsFunc(s.Components, func(c string) bool {
		return has(strings.ToLower(c), "table", "tabel")
	})
}

// takesInput reports whether a section takes input: it names no
// components, an input one, or a line naming a choice ("Pilihan segmen:
// Pendapatan atau Biaya" drawn with buttons).
func takesInput(s core.Section) bool {
	if len(s.Components) == 0 || slices.ContainsFunc(s.Components, inputComponent) {
		return true
	}
	return slices.ContainsFunc(s.Content, func(c string) bool {
		l := strings.ToLower(c)
		return has(l, segWords...) && (strings.HasPrefix(l, "pilihan") || strings.HasPrefix(l, "segmen"))
	})
}

// inputComponent reports whether a design component takes input.
func inputComponent(c string) bool {
	return has(strings.ToLower(c), "field", "input", "select", "textarea", "form", "check", "radio", "picker", "segmented", "toggle", "switch", "upload", "combobox", "dropdown")
}

// collectionKind reports whether a block renders as a collection with its
// search/filter and pagination: tables and lists always, cards inside a
// data panel.
func (r *render) collectionKind(kind string) bool {
	return kind == "table" || kind == "list" || kind == "cards" && r.panelOpen
}

// section renders one section in its slot.
func (r *render) section(s core.Section, sl Slot, slotted bool) {
	s, r.secLinks = r.sectionLinks(s)
	prev := r.curSec
	r.curSec = s.ID
	defer func() { r.secLinks, r.curSec = nil, prev }()
	kind := r.kindOf(s, sl)
	if r.page.Pattern == "detail" && r.state == "" {
		switch {
		case s.ID == r.heroSection():
			kind = "hero"
		case actionOnly(s) && r.heroSection() != "":
			return // its buttons stand beside the record's title
		case kind == "tabbar" || slices.ContainsFunc(s.Components, func(c string) bool { return strings.EqualFold(c, "Tabs") }):
			kind = "recordtabs"
		case (kind == "activity" || kind == "list" || kind == "table" || kind == "detail") && r.historyColumns(s) != nil:
			kind = "history"
		case kind == "activity" && r.namesShown(s):
			kind = "detail" // the record's values, not a history
		}
	}
	slot := ""
	if slotted {
		slot = fmt.Sprintf(` data-slot="%s"`, esc(s.Slot))
	}
	if r.collectionKind(kind) {
		r.collection(s, kind, slot, sl.Required || !slotted)
		return
	}
	cls := "fk-sec fk-k-" + ident(kind)
	switch {
	case r.panelOpen && kind == "toolbar":
		cls = "fk-tools-wrap"
	case r.panelOpen && kind == "pagination":
		cls = "fk-pager-sec"
	case bareKinds[kind]:
		cls += " fk-bare"
	case r.state != "" && dataKinds[kind] && kind != "metrics":
		cls += " fk-flush"
	}
	h2 := `<h2>`
	if kind == "hero" || r.panelOpen && (kind == "toolbar" || kind == "pagination" || kind == "tabbar") {
		h2 = `<h2 class="fk-sr">`
	}
	if len(r.secLinks) > 0 && r.state != "loading" {
		r.f(`<section class="%s" id="sec-%s" data-foundry-section="%s"%s aria-label="%s"><div class="fk-sec-top">%s%s</h2>`, cls, ident(s.ID), esc(s.ID), slot, esc(s.Name), h2, esc(s.Name))
		r.sectionMore()
		r.w(`</div>`)
	} else {
		r.f(`<section class="%s" id="sec-%s" data-foundry-section="%s"%s aria-label="%s">%s%s</h2>`, cls, ident(s.ID), esc(s.ID), slot, esc(s.Name), h2, esc(s.Name))
	}
	if dataKinds[kind] && r.dataState(s, sl.Required || !slotted, kind) {
		r.w(`</section>`)
		return
	}
	switch kind {
	case "hero":
		r.hero(s)
	case "recordtabs":
		tabs := tabItems(s)
		if len(tabs) == 0 {
			tabs = items(s)
		}
		r.tabPanel(s, tabs)
	case "history":
		_, _, notes := r.recordFacts(core.Section{Content: slices.DeleteFunc(slices.Clone(s.Content), func(c string) bool { return strings.Contains(c, ":") })}, make([]bool, len(r.fields(shownPres...))))
		r.feed(r.historyColumns(s), 5)
		for _, n := range notes {
			r.f(`<p class="fk-help">%s</p>`, esc(n))
		}
	case "toolbar":
		var els []string
		for _, c := range s.Content {
			els = append(els, filterEntry(c)...)
		}
		if len(els) < 2 {
			els = r.labels(s, 8, "filter")
		}
		if len(els) > 8 {
			els = els[:8]
		}
		r.tools(els)
	case "metrics":
		r.metrics(s)
	case "detail":
		r.detail(s)
	case "summary":
		r.summary(s)
	case "form":
		r.form(s)
	case "actions":
		r.actions(s)
	case "chart":
		r.chart(s)
	case "activity":
		r.activity(s)
	case "steps":
		r.steps(s)
	case "tabs", "nav":
		r.tabs(s)
	case "tabbar":
		r.tabBar(s)
	case "notice":
		r.notice(s)
	case "facets":
		r.facets(s)
	case "pagination":
		r.pager(s, false)
	case "cards":
		r.cards(s)
	default:
		r.text(s)
	}
	r.w(`</section>`)
}

// secLink is a link a section names to another page.
type secLink struct{ label, page string }

// linkWords start a content line that is a link: "Tautan ke riwayat
// mutasi", "Lihat semua barang menipis".
var linkWords = []string{"tautan ", "link ", "lihat semua ", "see all "}

// sectionLinks takes the links out of s's content: those that lead to a
// page (core.LinkTarget) become the section's footer links.
func (r *render) sectionLinks(s core.Section) (core.Section, []secLink) {
	var keep []string
	var out []secLink
	for _, c := range s.Content {
		t := strings.TrimSpace(c)
		l := strings.ToLower(t)
		i := slices.IndexFunc(linkWords, func(w string) bool { return strings.HasPrefix(l, w) })
		if i < 0 {
			keep = append(keep, c)
			continue
		}
		rest := strings.TrimSpace(t[len(linkWords[i]):])
		if strings.HasPrefix(linkWords[i], "lihat") || strings.HasPrefix(linkWords[i], "see") {
			rest = t
		}
		to := core.LinkTarget(r.doc.Layout, rest, r.page.ID)
		if to == "" {
			keep = append(keep, c)
			continue
		}
		label := rest
		if f := strings.Fields(rest); len(f) > 1 && has(strings.ToLower(f[0]), "ke", "to") && len(f[0]) <= 2 {
			label = "Lihat " + strings.Join(f[1:], " ")
		}
		out = append(out, secLink{capital(label), to})
	}
	s.Content = keep
	return s, out
}

// sectionMore draws the section's links at the end of its header ("Lihat
// semua ›"), where a card says where its full list is.
func (r *render) sectionMore() {
	if len(r.secLinks) == 0 || r.state == "loading" {
		return
	}
	r.w(`<div class="fk-sec-links">`)
	for _, ln := range r.secLinks {
		r.f(`<a href="%s" data-foundry-link="%s">%s%s</a>`, esc(r.doc.href(ln.page)), esc(ln.page), esc(ln.label), icon("right"))
	}
	r.w(`</div>`)
}

// dataState renders the loading, empty or error state in place of a data
// block; it reports whether it did.
func (r *render) dataState(s core.Section, main bool, kind string) bool {
	switch r.state {
	case "loading":
		r.skeleton(kind)
		return true
	case "empty", "no_results", "error":
		if _, own := s.States[r.state]; own || r.page.States[r.state] != "" || main {
			r.statePanel(r.state, &s)
			return true
		}
	}
	return false
}

// collection renders a table, list or cards section with its controls:
// header (name, count, tabs), search and filters, the records, then the
// pagination. Controls a page section provides are rendered by that
// section; missing ones are drawn with defaults, since every collection
// must have them.
func (r *render) collection(s core.Section, kind, slot string, main bool) {
	r.collections++
	if is := formContentIssue(r.page, s, kind); is != "" {
		r.contentIssues = append(r.contentIssues, is)
	}
	c, ok := r.colls[s.ID]
	if !ok {
		c = r.pageControls()
	}
	compact := kind == "list" && !r.panelOpen
	r.f(`<section class="fk-sec fk-coll fk-k-%s" id="sec-%s" data-foundry-section="%s"%s aria-label="%s">`, ident(kind), ident(s.ID), esc(s.ID), slot, esc(s.Name))
	if r.panelHead != s.ID {
		r.collHead(s)
	}
	if c.Filter == "" || c.Filter == "section" {
		r.w(`<div class="fk-tools-wrap">`)
		r.tools(filterElements(s, c.Filter == "", r.defaultFilters(s, kind)))
		r.w(`</div>`)
	}
	data := r.dataContent(s)
	if !r.dataState(s, main, kind) {
		switch kind {
		case "table":
			r.table(data)
		case "list":
			r.list(data, compact)
		case "cards":
			r.cards(data)
		}
	}
	if c.Pagination == "" || c.Pagination == "section" {
		r.pager(s, compact)
	}
	r.w(`</section>`)
}

// pageControls are the page's own toolbar and pagination sections.
func (r *render) pageControls() core.Collection {
	var c core.Collection
	for _, s := range r.kit.AssignSlots(r.page.Pattern, r.page.Sections) {
		switch core.ControlSection(r.page.Pattern, s) {
		case core.ControlFilter:
			c.Filter = s.ID
		case core.ControlPagination:
			c.Pagination = s.ID
		}
	}
	return c
}

// tabBar is a section of tabs above content: one line of segments.
func (r *render) tabBar(s core.Section) {
	tabs := tabItems(s)
	if len(tabs) == 0 {
		tabs = items(s)
	}
	r.tabNav("fk-tabs-line", s.Name, tabs)
}

// collHead is a collection's header: name, record count and its tabs.
func (r *render) collHead(s core.Section) {
	r.f(`<header class="fk-sec-head"><h2>%s</h2><span class="fk-count">%d</span>`, esc(s.Name), total(s))
	if tabs := tabItems(s); len(tabs) > 0 {
		r.tabNav("fk-tabs-line", s.Name, tabs)
	}
	r.sectionMore()
	r.w(`</header>`)
}

// total is the sample record count of a collection.
func total(s core.Section) int {
	if core.ContentKind(s) == "list" || s.Pattern == "list" || s.Pattern == "feed" {
		return 24
	}
	return 248
}

// entryKind is what a content entry is besides the records: a filter,
// pagination or tabs.
func entryKind(c string) string {
	if k := core.ControlEntry(c); k != "" {
		return k
	}
	l := strings.ToLower(strings.TrimSpace(c))
	if strings.HasPrefix(l, "tab ") || strings.HasPrefix(l, "tab:") || strings.HasPrefix(l, "tabs") {
		return "tabs"
	}
	return ""
}

// dataContent is the section with only the entries that describe its
// records.
func (r *render) dataContent(s core.Section) core.Section {
	out := s
	out.Content = nil
	var tabs []string
	for _, c := range s.Content {
		if entryKind(c) == "" {
			out.Content = append(out.Content, c)
		} else if _, tc, ok := core.TabColumns(c); ok {
			tabs = append(tabs, beforeNote(tc))
		}
	}
	cols := ""
	if len(tabs) > 0 {
		cols = tabs[r.curTab(len(tabs))] // the open tab is the one shown
	}
	if cols != "" {
		out.Content = append([]string{"Kolom: " + cols}, out.Content...)
	}
	return out
}

// beforeNote is a column list without the note after it: "No, Status;
// urut terbaru" lists No and Status.
func beforeNote(c string) string {
	depth := 0
	for i, r := range c {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ';':
			if depth == 0 {
				return strings.TrimSpace(c[:i])
			}
		}
	}
	return c
}

// tabItems are the tabs a section names: "Tab Aktif dan Selesai", or one
// "Tab <nama>: <kolom>" entry per tab.
func tabItems(s core.Section) []string {
	var named []string
	for _, c := range s.Content {
		if tab, _, ok := core.TabColumns(c); ok {
			named = append(named, capital(tab))
		}
	}
	if len(named) > 1 {
		return named
	}
	for _, c := range s.Content {
		if entryKind(c) != "tabs" {
			continue
		}
		c = strings.TrimSpace(c)
		if i := strings.IndexAny(c, ": "); i >= 0 {
			c = c[i+1:]
		}
		var out []string
		for _, p := range splitList(c) {
			out = append(out, capital(p))
		}
		if len(out) > 1 {
			return out
		}
	}
	return nil
}

// splitList splits "a, b dan c" into its elements.
func splitList(c string) []string {
	c = strings.NewReplacer(" dan ", ",", " and ", ",", " / ", ",", " atau ", ",").Replace(c)
	var out []string
	for _, p := range strings.Split(c, ",") {
		if p = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(p), ".")); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func capital(s string) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) == 0 {
		return ""
	}
	return strings.ToUpper(string(rs[:1])) + string(rs[1:])
}

// defaultFilters are the search and filter drawn for a collection whose
// design names none: a search for what it lists and its status.
func (r *render) defaultFilters(s core.Section, kind string) []string {
	what, st := entityNoun(s), "Status"
	if kind == "table" {
		cols := r.tableColumns(r.dataContent(s))
		if what == "" && len(cols) > 0 && len(strings.Fields(cols[0])) <= 3 {
			what = strings.ToLower(cols[0])
		}
		st = ""
		for _, c := range cols {
			if status(strings.ToLower(c)) || r.presentationOf(c) == "badge" {
				st = capital(c)
				break
			}
		}
	}
	search := "Cari"
	if what != "" {
		search += " " + what
	}
	if st == "" {
		return []string{search}
	}
	return []string{search, st}
}

// filterElements are the search and filters a collection names in its
// content, else the defaults.
func filterElements(s core.Section, defaults bool, def []string) []string {
	var out []string
	for _, c := range s.Content {
		if core.ControlEntry(c) != core.ControlFilter {
			continue
		}
		out = append(out, filterEntry(c)...)
	}
	if len(out) == 0 && defaults {
		out = def
	}
	return out
}

// filterEntry is what one toolbar entry filters by. "Filter status stok:
// semua, menipis" is one filter (status stok) with its options; "Filter:
// status, tanggal" and "status, tanggal" are two.
func filterEntry(c string) []string {
	c = strings.TrimSpace(c)
	if parts := strings.Split(c, ";"); len(parts) > 1 {
		var out []string
		for _, p := range parts {
			out = append(out, filterEntry(p)...)
		}
		return out
	}
	list := false // "Filter status dan tanggal" lists; "Cari SKU atau nama" is one search
	for _, p := range []string{"filter", "saring"} {
		if strings.HasPrefix(strings.ToLower(c), p) {
			c, list = strings.TrimLeft(c[len(p):], " "), true
		}
	}
	if name, opts, ok := strings.Cut(c, ":"); ok {
		if name = strings.TrimSpace(name); name != "" && !strings.EqualFold(name, "toolbar") {
			return []string{name}
		}
		c, list = opts, true
	}
	if list {
		return splitList(strings.TrimSpace(c))
	}
	return items(core.Section{Content: []string{c}})
}

// tools is a search box, filter chips and toolbar buttons. Every toolbar
// has a search; more than three filters fold into "Filter lainnya".
func (r *render) tools(els []string) {
	search := ""
	var filters, buttons []string
	for _, e := range els {
		l := strings.ToLower(e)
		switch {
		case has(l, "tombol", "tambah", "ekspor", "unduh", "impor", "buat", "export", "add", "download"):
			b, _ := buttonLabel(e)
			for _, a := range buttonActions(b) {
				// The header already offers the page's main action.
				if !slices.ContainsFunc(r.headActs, func(h string) bool { return h != "" && strings.EqualFold(actionName(h), actionName(a)) }) {
					buttons = append(buttons, a)
				}
			}
		case search == "" && has(l, "cari", "search", "pencarian", "kata kunci"):
			search = capital(strings.TrimRight(e, ".… "))
		default:
			if f := filterLabel(e); f != "" {
				filters = append(filters, f)
			}
		}
	}
	if search == "" {
		search = "Cari"
	}
	r.w(`<div class="fk-tools"><div class="fk-search" role="searchbox">` + icon("search") + `<span>` + esc(search) + `…</span></div>`)
	shown := filters
	if len(filters) > 3 {
		shown = filters[:2]
	}
	for _, f := range shown {
		ic := ""
		if has(strings.ToLower(f), "tanggal", "waktu", "periode", "rentang", "date", "jadwal", "bulan") {
			ic = icon("calendar")
		}
		r.w(`<span class="fk-chip" role="button">` + ic + esc(f) + icon("chevron") + `</span>`)
	}
	switch {
	case len(filters) > 3:
		r.f(`<span class="fk-chip" role="button">%sFilter lainnya<b>%d</b></span>`, icon("filter"), len(filters)-2)
	case len(filters) == 0:
		r.w(`<span class="fk-chip" role="button">` + icon("filter") + `Filter</span>`)
	}
	if len(buttons) > 0 {
		r.w(`<div class="fk-tools-end">`)
		for _, b := range buttons {
			r.actionControl(b, "fk-btn", true)
		}
		r.w(`</div>`)
	}
	r.w(`</div>`)
}

// filterLabel is the name a filter chip shows: what it filters, without
// its default, range or options ("Rentang tanggal dengan default 30 hari
// terakhir" is Rentang tanggal, "Kategori opsional" is Kategori).
func filterLabel(e string) string {
	e = strings.TrimSpace(e)
	for _, p := range []string{"filter ", "saring "} {
		if strings.HasPrefix(strings.ToLower(e), p) {
			e = e[len(p):]
		}
	}
	l := strings.ToLower(e)
	cut := len(e)
	for _, q := range []string{" dengan ", " default ", " dari ", " (", "; ", ": "} {
		if i := strings.Index(l, q); i > 0 && i < cut {
			cut = i
		}
	}
	e = strings.TrimSpace(e[:cut])
	for _, q := range []string{" opsional", " optional", " wajib"} {
		if strings.HasSuffix(strings.ToLower(e), q) {
			e = strings.TrimSpace(e[:len(e)-len(q)])
		}
	}
	return capital(e)
}

var firstNumber = regexp.MustCompile(`\d+`)

// pager is the pagination of a collection: page size, range, pages; a
// "muat lebih" design gets a load-more button. compact keeps the range
// and previous/next.
func (r *render) pager(s core.Section, compact bool) {
	size, more := 20, false
	for _, c := range s.Content {
		l := strings.ToLower(c)
		if core.ControlEntry(c) != core.ControlPagination && !has(l, "halaman", "muat", "page") {
			continue
		}
		if has(l, "muat lebih", "load more", "infinite", "gulir") {
			more = true
		}
		if n, err := strconv.Atoi(firstNumber.FindString(l)); err == nil && n > 0 && n <= 200 {
			size = n
		}
	}
	all := total(s)
	if compact {
		size = 5
	}
	pages := max((all+size-1)/size, 1)
	cur := min(max(r.doc.Sim.Page, 1), pages)
	if more {
		// Loading more shows the next batch below the ones shown.
		shown := min(cur*size, all)
		r.w(`<div class="fk-pager fk-pager-more">`)
		if shown < all {
			r.f(`<a class="fk-btn" href="%s">Muat lebih banyak</a>`, esc(r.pageHref(cur+1)))
		}
		r.f(`<span>%d dari %d</span></div>`, shown, all)
		return
	}
	r.w(`<div class="fk-pager">`)
	if !compact {
		r.f(`<div class="fk-pager-size">Baris per halaman <span class="fk-chip fk-chip-solid">%d%s</span></div>`, size, icon("chevron"))
	}
	// Each page number opens that page; the current one is marked.
	pg := func(p int, cls, label, inner string) {
		switch {
		case p < 1 || p > pages:
			r.f(`<button class="%s" disabled%s>%s</button>`, cls, label, inner)
		case p == cur && !strings.Contains(cls, "fk-nav-pg"):
			r.f(`<a class="%s on" aria-current="page" href="%s"%s>%s</a>`, cls, esc(r.pageHref(p)), label, inner)
		default:
			r.f(`<a class="%s" href="%s"%s>%s</a>`, cls, esc(r.pageHref(p)), label, inner)
		}
	}
	r.f(`<span>%d–%d dari %d</span><nav class="fk-pages" aria-label="Paginasi">`, (cur-1)*size+1, min(cur*size, all), all)
	pg(cur-1, "fk-pg fk-nav-pg", ` aria-label="Sebelumnya"`, icon("left"))
	if !compact {
		shown := []int{1, 2, 3, pages}
		if cur > 3 && cur < pages {
			shown = []int{1, cur - 1, cur, cur + 1, pages}
		}
		last := 0
		for _, p := range shown {
			if p <= last || p > pages {
				continue
			}
			if p > last+1 {
				r.w(`<span class="fk-gap">…</span>`)
			}
			pg(p, "fk-pg", "", strconv.Itoa(p))
			last = p
		}
	}
	pg(cur+1, "fk-pg fk-nav-pg", ` aria-label="Berikutnya"`, icon("right"))
	r.w(`</nav></div>`)
}

// pageHref opens page p of this page's collection.
func (r *render) pageHref(p int) string {
	sim := r.doc.Sim.Nav()
	sim.Tab, sim.Page = r.doc.Sim.Tab, p
	return r.doc.hrefSim(r.page.ID, sim)
}

func (r *render) stateText(s *core.Section, state string) string {
	if s != nil {
		if t := strings.TrimSpace(s.States[state]); t != "" {
			return t
		}
	}
	if t := strings.TrimSpace(r.page.States[state]); t != "" {
		return t
	}
	return defaultStates[state]
}

func (r *render) statePanel(state string, s *core.Section) {
	ic, title := "box", "Belum ada data"
	switch state {
	case "error":
		ic, title = "alert", "Terjadi kesalahan"
	case "no_results":
		ic, title = "search", "Tidak ada hasil"
	case "permission_denied":
		ic, title = "shield", "Akses ditolak"
	}
	r.f(`<div class="fk-state fk-state-%s" role="status"><span class="fk-state-icon">%s</span><strong>%s</strong><p>%s</p>`,
		ident(state), icon(ic), title, esc(r.stateText(s, state)))
	switch state {
	case "error":
		// Trying again loads the page with its data.
		r.f(`<a class="fk-btn" href="%s">Coba lagi</a>`, esc(r.doc.hrefSim(r.page.ID, r.doc.Sim.Nav())))
	case "empty":
		// An empty list invites making its first record; any other main
		// action is in the header already.
		if a := r.page.PrimaryAction; a != "" && core.StartsCreate(a) {
			if pg := r.leadsTo(a, nil); pg != "" {
				r.f(`<a class="fk-btn fk-btn-primary" href="%s" data-foundry-link="%s">%s%s</a>`, esc(r.doc.href(pg)), esc(pg), actionIcon(a), esc(a))
			} else {
				r.w(`<button class="fk-btn fk-btn-primary">` + actionIcon(a) + esc(a) + `</button>`)
			}
		}
	}
	r.w(`</div>`)
}

func (r *render) skeleton(kind string) {
	n := 5
	if kind == "metrics" || kind == "summary" {
		n = 2
	}
	r.w(`<div class="fk-skel" aria-busy="true" style="padding:var(--space-lg)">`)
	for i := 0; i < n; i++ {
		r.w(`<span></span>`)
	}
	r.w(`</div>`)
}

// items splits the section content into elements: "Tabel: No, Nama" lists
// No and Nama; "a, b" lists a and b; anything else is one element.
func items(s core.Section) []string {
	var out []string
	for _, c := range s.Content {
		c = strings.TrimSpace(c)
		if i := strings.Index(c, ":"); i >= 0 && i < len(c)-1 {
			c = c[i+1:]
		}
		if strings.Contains(c, ",") {
			// "Aksi (Lihat, Arsipkan)" is one element; "…, dan Status"
			// lists Status.
			for _, p := range splitOutside(c) {
				p = strings.TrimSpace(leadWords.ReplaceAllString(strings.TrimSpace(p), ""))
				if p = strings.TrimSpace(strings.TrimSuffix(p, ".")); p != "" {
					out = append(out, p)
				}
			}
			continue
		}
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}

// splitOutside splits c at the commas outside parentheses.
func splitOutside(c string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range c {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, c[start:i])
				start = i + 1
			}
		}
	}
	return append(out, c[start:])
}

// fields are the page's fields shown as one of the presentations.
func (r *render) fields(pres ...string) []core.FieldRule {
	var out []core.FieldRule
	for _, f := range r.page.Fields {
		for _, p := range pres {
			if f.Presentation == p {
				out = append(out, f)
			}
		}
	}
	return out
}

func fieldLabel(f core.FieldRule) string {
	if f.Label != "" {
		return f.Label
	}
	name := f.Field
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return strings.ReplaceAll(name, "_", " ")
}

// labels are the section's own elements, else the labels of the page
// fields of those presentations.
func (r *render) labels(s core.Section, max int, pres ...string) []string {
	out := items(s)
	if len(out) < 2 {
		var fs []string
		for _, f := range r.fields(pres...) {
			fs = append(fs, fieldLabel(f))
		}
		if len(fs) > len(out) {
			out = fs
		}
	}
	if len(out) > max {
		out = out[:max]
	}
	return out
}

func (r *render) presentationOf(label string) string {
	for _, f := range r.page.Fields {
		if strings.EqualFold(fieldLabel(f), label) || strings.EqualFold(f.Field, label) {
			return f.Presentation
		}
	}
	return ""
}

// metricItems are the metrics a section names: "Label: value" stays whole,
// "a, b" lists a and b.
func metricItems(s core.Section) []string {
	var out []string
	for _, c := range s.Content {
		c = strings.TrimSpace(c)
		switch {
		case c == "":
		case strings.Contains(c, ":"):
			// "Label: value" when the value is one short figure, else a
			// list named by its label.
			label, value, _ := strings.Cut(c, ":")
			if slices.Contains(componentWords, strings.ToLower(strings.TrimSpace(label))) {
				// "StatCard: kehadiran, tagihan" lists the metrics.
				for _, p := range splitOutside(fontNoteRe.ReplaceAllString(value, "")) {
					if p = strings.TrimSpace(leadWords.ReplaceAllString(strings.TrimSpace(p), "")); p != "" {
						out = append(out, capital(p))
					}
				}
				continue
			}
			if strings.Contains(value, ",") && !strings.ContainsAny(value, "0123456789") {
				out = append(out, strings.TrimSpace(label))
			} else {
				out = append(out, c)
			}
		case !strings.Contains(c, ","):
			out = append(out, c)
		default:
			for _, p := range splitOutside(c) {
				if p = strings.TrimSpace(leadWords.ReplaceAllString(strings.TrimSpace(p), "")); p != "" {
					out = append(out, p)
				}
			}
		}
	}
	return out
}

func (r *render) metrics(s core.Section) {
	els := metricItems(s)
	if len(els) < 2 {
		for _, f := range r.fields("card-value", "summary") {
			els = append(els, fieldLabel(f))
		}
	}
	if len(els) > 4 {
		els = els[:4]
	}
	if len(els) == 0 {
		els = []string{s.Name}
	}
	r.w(`<div class="fk-metrics">`)
	for i, it := range els {
		// "StatCard Total barang" names the component, not the metric.
		it = stripWord(it, componentWords)
		label, value := it, metricValue(it, i)
		if j := strings.Index(it, ":"); j > 0 {
			label, value = strings.TrimSpace(it[:j]), strings.TrimSpace(it[j+1:])
		}
		hint := "vs periode lalu"
		if a, b, ok := strings.Cut(label, " dengan "); ok {
			label, hint = a, b
		}
		delta, cls, arrow := 3+i*2, "", "↑"
		if i == 2 {
			cls, arrow = " down", "↓"
		}
		r.f(`<div class="fk-metric"><div class="fk-metric-top"><span class="fk-metric-label">%s</span><span class="fk-metric-icon">%s</span></div><span class="fk-metric-value">%s</span><div class="fk-metric-foot"><span class="fk-delta%s">%s %d%%</span><span>%s</span></div></div>`,
			esc(capital(label)), icon(metricIcon(label)), esc(value), cls, arrow, delta, esc(hint))
	}
	r.w(`</div>`)
}

func metricIcon(label string) string {
	l := strings.ToLower(label)
	switch {
	case money(l):
		return "card"
	case has(l, "waktu", "durasi", "lama", "time"):
		return "clock"
	case has(l, "persen", "%", "tingkat", "rate", "rasio", "konversi", "okupansi"):
		return "trend"
	}
	if ic := iconFor(l); ic != "file" {
		return ic
	}
	return "chart"
}

// metricValue is a placeholder value for a metric label.
func metricValue(label string, i int) string {
	l := strings.ToLower(label)
	switch {
	case has(l, "persen", "%", "tingkat", "rate", "rasio", "konversi"):
		return []string{"86%", "4,2%", "92%", "12%"}[i%4]
	case money(l):
		return []string{"Rp 48,2 jt", "Rp 1,3 jt", "Rp 12,8 jt", "Rp 640 rb"}[i%4]
	case has(l, "waktu", "durasi", "lama", "time"):
		return []string{"14 mnt", "2,5 jam", "38 dtk", "1 hari"}[i%4]
	}
	return []string{"1.284", "312", "57", "9"}[i%4]
}

// columnList is the content entry that lists the most elements: the
// table's columns ("Tabel: No, Plat, Status").
func columnList(s core.Section) []string {
	var best []string
	for _, c := range s.Content {
		got := items(core.Section{Content: []string{beforeNote(c)}})
		if len(got) > len(best) {
			best = got
		}
	}
	if len(best) > 0 {
		f := strings.Fields(best[0])
		if len(f) > 1 && has(strings.ToLower(f[0]), "tabel", "table", "kolom") {
			best[0] = capital(strings.Join(f[1:], " "))
		}
	}
	return best
}

// tableColumns are the columns a table section names, else the page's
// table fields, plus an action column when the content mentions actions.
func (r *render) tableColumns(s core.Section) []string {
	cols := columnList(s)
	if len(cols) < 2 {
		var fs []string
		for _, f := range r.fields("table-column", "badge", "row-action") {
			fs = append(fs, fieldLabel(f))
		}
		if len(fs) >= 2 {
			cols = fs
		} else {
			cols = r.labels(s, 7)
		}
	}
	if !slices.ContainsFunc(cols, r.isAction) && slices.ContainsFunc(s.Content, func(c string) bool {
		return has(strings.ToLower(c), "aksi", "action")
	}) {
		cols = append(cols, "Aksi")
	}
	if len(cols) > 7 {
		// The row actions stay: they are the way to each record.
		if i := slices.IndexFunc(cols[7:], r.isAction); i >= 0 {
			cols = append(cols[:6:6], cols[7+i])
		} else {
			cols = cols[:7]
		}
	}
	if len(cols) == 0 {
		cols = []string{s.Name}
	}
	return cols
}

func (r *render) table(s core.Section) {
	cols := r.tableColumns(s)
	r.w(`<div class="fk-table-wrap"><table class="fk-table"><thead><tr>`)
	for _, c := range cols {
		l := strings.ToLower(c)
		switch {
		case r.isAction(c):
			r.f(`<th scope="col" class="fk-act"><span class="fk-sr">%s</span></th>`, esc(c))
		case numeric(l) && !status(l):
			r.f(`<th scope="col" class="fk-num">%s</th>`, esc(c))
		default:
			if h := r.hintOf(c); h != "" {
				// What the column means, for a reader who does not know the data.
				r.f(`<th scope="col"><span class="fk-hint" title="%s">%s</span></th>`, esc(h), esc(c))
				continue
			}
			r.f(`<th scope="col">%s</th>`, esc(c))
		}
	}
	r.w(`</tr></thead><tbody>`)
	noun := entityNoun(s)
	acts := rowActions(s)
	// A row opens the record's detail page: its first cell links there.
	detail := core.DetailPage(r.doc.Layout, r.page.ID)
	if r.state != "" {
		detail = ""
	}
	// Another page or tab of the collection shows other records.
	base := (max(r.doc.Sim.Page, 1)-1)*5 + r.doc.Sim.Tab
	for row := 0; row < 5; row++ {
		r.w(`<tr>`)
		for i, c := range cols {
			if l := strings.ToLower(c); i == 0 && (l == "nama" || l == "name") && noun != "" && !person(noun) {
				r.w(r.rowLink(fmt.Sprintf(`<td data-label="%s"><span class="fk-strong">%s %s</span></td>`, esc(c), esc(capital(noun)), []string{"A1", "A2", "B1", "B2", "C1", "C2"}[row]), detail))
				continue
			}
			if i == 0 && detail != "" {
				r.w(r.rowLink(r.cell(c, row+base, true), detail))
				continue
			}
			if r.isAction(c) && has(strings.ToLower(c), "aksi", "action", "opsi") {
				// "Aksi Catat masuk" names the actions of its own column.
				ca := acts
				if f := strings.Fields(c); len(ca) == 0 && len(f) > 1 {
					for _, a := range splitList(strings.Trim(strings.Join(f[1:], " "), "() ")) {
						ca = append(ca, capital(a))
					}
				}
				if len(ca) == 0 {
					ca = r.rowDefaults() // a bare "Aksi" opens and edits the row
				}
				if len(ca) > 0 {
					r.rowMenu(s, c, row, ca)
				} else {
					r.f(`<td data-label="%s" class="fk-act"></td>`, esc(c))
				}
				continue
			}
			if r.isAction(c) && r.presentationOf(c) == "row-action" {
				r.rowMenu(s, c, row, []string{c}) // the column is the action
				continue
			}
			r.w(r.cell(c, row+base, i == 0))
		}
		r.w(`</tr>`)
	}
	r.w(`</tbody></table></div>`)
}

// rowActions are the actions a table section names for each row: "Aksi
// baris: Ubah, Hapus untuk Admin" names Ubah and Hapus.
func rowActions(s core.Section) []string {
	var out []string
	for _, c := range s.Content {
		c = strings.TrimSpace(c)
		l := strings.ToLower(c)
		p := ""
		for _, w := range []string{"aksi baris", "row action"} {
			if strings.HasPrefix(l, w) {
				p = w
			}
		}
		if p == "" {
			continue
		}
		rest := c[len(p):]
		if p == "row action" {
			rest = strings.TrimPrefix(rest, "s")
		}
		for _, a := range splitList(strings.TrimPrefix(strings.TrimSpace(rest), ":")) {
			a = actionName(a)
			f := strings.Fields(strings.ToLower(a))
			if len(f) == 0 || slices.Contains([]string{"untuk", "hanya", "bila", "jika", "sesuai", "for", "only"}, f[0]) {
				continue
			}
			if a = capital(strings.TrimSpace(a)); !slices.Contains(out, a) {
				out = append(out, a)
			}
		}
	}
	return out
}

// actionName is an action without what follows it to say who or when
// ("Hapus untuk Admin" is Hapus).
func actionName(a string) string {
	cut := len(a)
	for _, q := range []string{"(", " untuk ", " hanya ", " bila ", " jika ", " dengan ", " for ", " only "} {
		if i := strings.Index(strings.ToLower(a), q); i >= 0 && i < cut {
			cut = i
		}
	}
	return a[:cut]
}

// rowMenu is a row's actions: the first one that is not destructive as
// a button in the row, the others in the menu its more button opens,
// destructive ones last. Each opens the dialog or confirmation it names,
// leads to the page it names, else does nothing (closes the menu).
func (r *render) rowMenu(s core.Section, label string, row int, acts []string) {
	id := fmt.Sprintf("m-%s-%d", ident(s.ID), row)
	var inline string
	var safe, danger []string
	for _, a := range acts {
		switch {
		case core.DangerAction(a):
			danger = append(danger, a)
		case inline == "":
			inline = a
		default:
			safe = append(safe, a)
		}
	}
	var confirms [][2]string
	// item draws action a: a menu item when menu, else a row button.
	item := func(a string, menu bool) {
		cls, role := "fk-btn fk-btn-sm fk-row-btn", ""
		if menu {
			cls, role = "fk-menu-item", ` role="menuitem"`
			if core.DangerAction(a) {
				cls += " fk-menu-danger"
			}
		}
		text := actionIcon(a) + esc(a)
		target, action := r.dialogFor(a), "show"
		if target == "" {
			if pg := r.leadsTo(a, &s); pg != "" {
				r.f(`<a class="%s"%s href="%s" data-foundry-link="%s">%s</a>`, cls, role, esc(r.rowHref(pg, a)), esc(pg), text)
				return
			}
			target, action = "", ""
			if menu {
				target, action = id, "hide"
			}
			// An action that needs confirming asks it, as the header does.
			var lt *core.LifeTransition
			if t := r.matchTransition(a); t != nil {
				lt = &t.T
			}
			if core.ConfirmAction(a, lt) {
				target, action = fmt.Sprintf("c-%s-%s", ident(s.ID), ident(a)), "show"
				confirms = append(confirms, [2]string{target, a})
			}
		}
		if target == "" || action == "hide" {
			// Nothing to open: the prototype answers the action itself.
			if h := r.simHref(a); h != "" {
				r.f(`<a class="%s"%s href="%s" data-foundry-sim="act">%s</a>`, cls, role, esc(h), text)
				return
			}
		}
		if target == "" {
			r.f(`<button class="%s" type="button">%s</button>`, cls, text)
			return
		}
		r.f(`<button class="%s" type="button"%s popovertarget="%s" popovertargetaction="%s">%s</button>`, cls, role, target, action, text)
	}
	r.f(`<td data-label="%s" class="fk-act"><div class="fk-row-acts">`, esc(label))
	if inline != "" {
		item(inline, false)
	}
	if len(safe)+len(danger) > 0 {
		r.f(`<button class="fk-icon-btn fk-row-more" aria-label="%s lainnya" aria-haspopup="menu" popovertarget="%s" style="anchor-name:--%s">%s</button>`, esc(label), id, id, icon("more"))
		r.f(`<div id="%s" popover class="fk-menu" role="menu" style="position-anchor:--%s">`, id, id)
		for _, a := range safe {
			item(a, true)
		}
		if len(safe) > 0 && len(danger) > 0 {
			r.w(`<hr class="fk-menu-sep" role="separator">`)
		}
		for _, a := range danger {
			item(a, true)
		}
		r.w(`</div>`)
	}
	r.w(`</div>`)
	// Every row asks the same confirmation; it is drawn once.
	for _, c := range confirms {
		if row == 0 {
			r.asked(c[0], c[1], c[1])
			r.confirmBox(c[0], c[1], r.matchTransition(c[1]))
		}
	}
	r.w(`</td>`)
}

// rowLink makes the title of a row's first cell td a link to page.
func (r *render) rowLink(td, page string) string {
	const open = `<span class="fk-strong">`
	i := strings.Index(td, open)
	if page == "" || i < 0 {
		return td
	}
	j := strings.Index(td[i:], "</span>")
	if j < 0 {
		return td
	}
	j += i
	return td[:i] + fmt.Sprintf(`<a class="fk-strong fk-row-link" href="%s" data-foundry-link="%s">`, esc(r.doc.href(page)), esc(page)) + td[i+len(open):j] + `</a>` + td[j+len("</span>"):]
}

// actionPage is the one other page a row action of section s leads to:
// the page whose name holds every word of the action ("Detail" leads to
// "Detail Barang"). An action of one word must also meet the page or the
// section in that name, so "Ubah" on Pengguna does not lead to "Ubah
// Barang". "" when no page, or more than one, fits.
func (r *render) actionPage(a string, s core.Section) string {
	found := ""
	for _, pg := range r.doc.Layout.Pages {
		if pg.ID == r.page.ID || !core.StemsCovered(a, pg.Name) {
			continue
		}
		if len(strings.Fields(a)) < 2 && !core.SharesStem(pg.Name, r.page.Name+" "+s.Name) {
			continue
		}
		if found != "" {
			return ""
		}
		found = pg.ID
	}
	return found
}

// entityNoun is what a table lists when its content or name says so:
// "Tabel gerbang: nama, lokasi" lists gerbang.
func entityNoun(s core.Section) string {
	cands := []string{s.Name}
	for _, c := range s.Content {
		if label, _, ok := strings.Cut(c, ":"); ok {
			cands = append([]string{label}, cands...)
		}
	}
	for _, c := range cands {
		f := strings.Fields(strings.ToLower(c))
		if len(f) >= 2 && (f[0] == "tabel" || f[0] == "daftar" || f[0] == "table" || f[0] == "list") {
			switch f[1] {
			case "data", "aktif", "utama", "hasil", "semua", "terbaru":
				return ""
			}
			return f[1]
		}
	}
	return ""
}

func (r *render) isAction(label string) bool {
	return r.presentationOf(label) == "row-action" || has(strings.ToLower(label), "aksi", "action", "opsi")
}

func (r *render) cell(label string, row int, first bool) string {
	l := strings.ToLower(label)
	pres := r.presentationOf(label)
	td := `<td data-label="` + esc(label) + `"`
	switch {
	case r.isAction(label):
		if pres == "row-action" && !has(l, "aksi", "action", "opsi") {
			return td + ` class="fk-act"><button class="fk-btn fk-btn-sm">` + esc(label) + `</button></td>`
		}
		return td + ` class="fk-act"><button class="fk-icon-btn" aria-label="` + esc(label) + `">` + icon("more") + `</button></td>`
	}
	v, tone := r.cellValue(label, row)
	switch {
	case pres == "badge" || status(l):
		if tone == "" {
			tone = "info"
		}
		return fmt.Sprintf(`%s><span class="fk-badge fk-badge-%s">%s</span></td>`, td, tone, esc(v))
	case numeric(l) || digitsOnly(v):
		return td + ` class="fk-num">` + esc(v) + `</td>`
	case first && person(l):
		return td + `><span class="fk-who"><span class="fk-avatar">` + esc(initials(v)) + `</span><span class="fk-strong">` + esc(v) + `</span></span></td>`
	case first:
		return td + `><span class="fk-strong">` + esc(v) + `</span></td>`
	}
	return td + `>` + esc(v) + `</td>`
}

// list renders records as rows: icon or avatar, title, meta, status and
// time. Example values the content names ("Peringatan: gerbang offline,
// transaksi gagal") become the row titles.
func (r *render) list(s core.Section, compact bool) {
	els := items(s)
	title, meta := "", ""
	if fs := r.fields("card-title", "list-item"); len(fs) > 0 {
		title = fieldLabel(fs[0])
	} else if len(els) > 0 {
		title = els[0]
	}
	text := strings.ToLower(s.Name + " " + strings.Join(s.Content, " "))
	if fs := r.fields("card-meta"); len(fs) > 0 && strings.Contains(text, strings.ToLower(fieldLabel(fs[0]))) {
		meta = fieldLabel(fs[0])
	} else {
		for _, w := range []string{"lokasi", "pelanggan", "nama", "tanggal", "email"} {
			if strings.Contains(text, w) {
				meta = w
				break
			}
		}
	}
	examples := exampleValues(s)
	badgeLabel := ""
	if fs := r.fields("badge"); len(fs) > 0 {
		badgeLabel = fieldLabel(fs[0])
	}
	badge := badgeLabel != "" || hasAny(els, status)
	rows := 5
	if compact {
		rows = 4
	}
	r.w(`<ul class="fk-list">`)
	for row := 0; row < rows; row++ {
		cls := ""
		if row == 0 && r.page.Pattern == "master-detail" {
			cls = ` class="on"`
		}
		t, _ := r.cellValue(title, row)
		if len(examples) > 0 {
			t = capital(examples[row%len(examples)])
		}
		lead := icon(iconFor(s.Name + " " + title))
		if person(strings.ToLower(title)) {
			lead = `<span class="fk-avatar">` + esc(initials(t)) + `</span>`
		} else {
			lead = `<span class="fk-thumb">` + lead + `</span>`
		}
		r.f(`<li%s>%s<div class="fk-list-main"><span class="fk-list-title">%s</span>`, cls, lead, esc(t))
		m := sample("waktu", row)
		if meta != "" && !compact {
			mv, _ := r.cellValue(meta, row)
			m = mv + " · " + m
		}
		r.w(`<span class="fk-meta">` + esc(m) + `</span></div>`)
		if badge {
			w, c := statusFor(s.Name+" "+badgeLabel, row)
			if v, t, ok := r.valueOf(r.ruleFor(badgeLabel), badgeLabel, row); ok && badgeLabel != "" {
				w, c = esc(v), t
				if c == "" {
					c = "info"
				}
			}
			r.f(`<span class="fk-badge fk-badge-%s">%s</span>`, c, w)
		}
		r.w(icon("right") + `</li>`)
	}
	r.w(`</ul>`)
}

// exampleValues are the example records a content entry lists after its
// label: "Daftar peringatan: gerbang offline, transaksi gagal".
func exampleValues(s core.Section) []string {
	for _, c := range s.Content {
		label, values, ok := strings.Cut(c, ":")
		if !ok || strings.ContainsAny(label, ",") {
			continue
		}
		if vs := splitList(values); len(vs) >= 2 && len(vs) <= 6 {
			return vs
		}
	}
	return nil
}

func person(l string) bool {
	return has(l, "nama", "name", "pelanggan", "customer", "pasien", "pengguna", "user", "penulis", "pemilik", "karyawan", "siswa", "member", "anggota", "kontak")
}

func (r *render) cards(s core.Section) {
	els := items(s)
	pick := func(pres string, i int) string {
		if fs := r.fields(pres); len(fs) > 0 {
			return fieldLabel(fs[0])
		}
		if i < len(els) {
			return els[i]
		}
		return ""
	}
	title, meta, value := pick("card-title", 0), pick("card-meta", 1), pick("card-value", 2)
	badge := len(r.fields("badge")) > 0
	r.w(`<div class="fk-cards">`)
	for row := 0; row < 6; row++ {
		r.f(`<article class="fk-card"><div class="fk-card-media"></div><div class="fk-card-body"><span class="fk-list-title">%s</span>`, esc(textOf(r.cellValue(title, row))))
		if meta != "" {
			r.w(`<span class="fk-meta">` + esc(textOf(r.cellValue(meta, row))) + `</span>`)
		}
		if value != "" {
			r.w(`<span class="fk-card-value">` + esc(textOf(r.cellValue(value, row))) + `</span>`)
		}
		r.w(`</div><div class="fk-card-foot">`)
		if badge {
			w, c := statusFor(s.Name, row)
			r.f(`<span class="fk-badge fk-badge-%s">%s</span>`, c, w)
		} else {
			r.w(`<span class="fk-meta">` + esc(sample("tanggal", row)) + `</span>`)
		}
		// The card's actions: open its record; none when it has nowhere to go.
		if acts := r.rowDefaults(); len(acts) > 0 {
			if pg := r.leadsTo(acts[0], &s); pg != "" {
				r.f(`<a class="fk-icon-btn" href="%s" data-foundry-link="%s" aria-label="%s">%s</a>`, esc(r.actionHref(pg, acts[0])), esc(pg), esc(acts[0]), icon("more"))
			}
		}
		r.w(`</div></article>`)
	}
	r.w(`</div>`)
}

func (r *render) detail(s core.Section) {
	if r.page.Pattern == "detail" {
		// A detail page names its record's values by its own fields.
		used := r.heroUsed
		if used == nil {
			used = make([]bool, len(r.fields(shownPres...)))
		}
		facts, acts, notes := r.recordFacts(s, used)
		for _, n := range notes {
			r.f(`<p class="fk-sec-desc">%s</p>`, esc(n))
		}
		r.factList(facts, true)
		r.recordActions(acts, "fk-detail-actions")
		return
	}
	// "Aksi sesuai status: Konfirmasi, Batalkan" lists the record's
	// actions, not its fields.
	var acts []string
	var content []string
	for _, c := range s.Content {
		if b, ok := buttonLabel(strings.TrimSpace(c)); ok {
			// "Tombol Buat Cadangan: pilih lokasi file" names one button;
			// what follows the colon describes it.
			acts = append(acts, buttonActions(b)...)
			continue
		}
		if label, list, ok := strings.Cut(c, ":"); ok && actionLabel(label) {
			for _, a := range splitList(list) {
				acts = append(acts, capital(a))
			}
			continue
		}
		content = append(content, c)
	}
	var warns []string
	if r.formPage() {
		// Beside a form the section shows the record the form works on:
		// its values by the page's names, its warnings as warnings.
		content, warns = splitWarnings(content)
	}
	s.Content = content
	els := r.labels(s, 12, "detail")
	if r.formPage() {
		els = r.contextLabels(els)
	}
	facts := 0
	for _, it := range els {
		if a, ok := actionEntry(it); ok {
			acts = append(acts, a...)
		} else {
			facts++
		}
	}
	if facts > 0 {
		r.w(`<dl class="fk-dl">`)
	}
	for i, it := range els {
		if _, ok := actionEntry(it); ok {
			continue
		}
		if v, c := r.cellValue(it, i); status(strings.ToLower(it)) || c != "" && r.presentationOf(it) == "badge" {
			if c == "" {
				c = "info"
			}
			r.f(`<div><dt>%s</dt><dd><span class="fk-badge fk-badge-%s">%s</span></dd></div>`, esc(capital(it)), c, esc(v))
		} else {
			r.f(`<div><dt>%s</dt><dd>%s</dd></div>`, esc(capital(it)), esc(v))
		}
	}
	if facts > 0 {
		r.w(`</dl>`)
	}
	for _, w := range warns {
		r.callout(w)
	}
	var safe, danger []string
	for _, a := range acts {
		switch {
		case r.offeredAbove(a):
		case core.DangerAction(a):
			danger = append(danger, a)
		default:
			safe = append(safe, a)
		}
	}
	if len(safe)+len(danger) == 0 {
		return
	}
	// The record's actions: the first primary, the destructive ones
	// apart at the end.
	cls := "fk-actions fk-detail-actions"
	if facts > 0 {
		cls += " fk-detail-actions-sep"
	}
	r.f(`<div class="%s">`, cls)
	for i, a := range safe {
		c := "fk-btn"
		if i == 0 {
			c = "fk-btn fk-btn-primary"
		}
		r.actionControl(a, c, true)
	}
	if len(danger) > 0 {
		r.w(`<div class="fk-actions fk-actions-end">`)
		for _, a := range danger {
			r.actionControl(a, "fk-btn fk-btn-danger", true)
		}
		r.w(`</div>`)
	}
	r.w(`</div>`)
}

// actionLabel reports whether label names a list of actions ("Aksi
// sesuai status", "Tombol").
func actionLabel(label string) bool {
	f := strings.Fields(strings.ToLower(label))
	return len(f) > 0 && slices.Contains([]string{"aksi", "tombol", "button", "buttons", "action", "actions"}, f[0])
}

// actionEntry reads "Aksi ajukan atau kirim" as the buttons Ajukan and
// Kirim: a record's next actions, not a field.
func actionEntry(it string) ([]string, bool) {
	l := strings.ToLower(strings.TrimSpace(it))
	for _, p := range []string{"aksi ", "tombol ", "button "} {
		if !strings.HasPrefix(l, p) {
			continue
		}
		rest := strings.TrimSpace(it)[len(p):]
		for _, sep := range []string{" atau ", " dan ", " / ", "/"} {
			rest = strings.ReplaceAll(rest, sep, "|")
		}
		var out []string
		for _, a := range strings.Split(rest, "|") {
			if a = strings.TrimSpace(a); a != "" {
				out = append(out, capital(a))
			}
		}
		return out, len(out) > 0
	}
	return nil, false
}

func (r *render) summary(s core.Section) {
	els := r.labels(s, 5, "summary", "card-value", "badge")
	r.w(`<div class="fk-summary">`)
	for i, it := range els {
		v, c := r.cellValue(it, i)
		if status(strings.ToLower(it)) {
			if c == "" {
				c = "info"
			}
			r.f(`<div><span class="fk-meta">%s</span><span class="fk-badge fk-badge-%s">%s</span></div>`, esc(it), c, esc(v))
			continue
		}
		r.f(`<div><span class="fk-meta">%s</span><strong>%s</strong></div>`, esc(it), esc(v))
	}
	r.w(`</div>`)
}

func (r *render) form(s core.Section) {
	p := r.plan()
	for _, n := range p.desc[s.ID] {
		r.f(`<p class="fk-sec-desc">%s</p>`, esc(n))
	}
	r.formWith(s, p.bySec[s.ID], nil)
}

// formFields are the fields a form section names, and its texts. A line
// that is a text or the place of an alert is no field: the text is shown
// above the form, the alert where the form fails. A button line is the
// page's submit button (see pageSubmit).
func (r *render) formFields(s core.Section) (els, notes []string) {
	fields := s
	fields.Content = nil
	for _, c := range s.Content {
		switch kind, text := formLine(c); kind {
		case "note":
			notes = append(notes, text)
		case "alert":
		default:
			fields.Content = append(fields.Content, c)
		}
	}
	for _, it := range r.labels(fields, 10, "form-input") {
		if _, ok := buttonLabel(it); !ok {
			els = append(els, capital(it))
		}
	}
	return els, notes
}

// pageFail is the failure of the page's form, over all its sections: one
// part of the error state, marking at most one field.
func (r *render) pageFail(act string) failure {
	var labels []string
	p := r.plan()
	for _, s := range r.formSections() {
		for _, f := range p.bySec[s.ID] {
			labels = append(labels, f.label)
		}
	}
	for _, f := range p.unplaced {
		labels = append(labels, f.label)
	}
	return r.failure(act, labels)
}

// componentWords name the component that shows a value.
var componentWords = []string{"statcard", "kpicard", "metriccard", "kpi", "card", "kartu", "stat"}

// controlWords name the control of a field ("Textarea catatan" is the
// field Catatan).
var controlWords = []string{"input", "field", "textarea", "pilihan", "searchselect", "select", "dropdown", "combobox"}

// stripWord drops the first word of s when it is one of words.
func stripWord(s string, words []string) string {
	s = strings.TrimSpace(s)
	if w, rest, ok := strings.Cut(s, " "); ok && slices.Contains(words, strings.ToLower(w)) {
		return strings.TrimSpace(rest)
	}
	return s
}

// formLine says what a line of a form section is besides a field: a text
// ("Teks 'Akun dibuat oleh Admin'", with its words) or the place of an
// alert ("Alert error generik").
func formLine(c string) (kind, text string) {
	c = strings.TrimSpace(c)
	w, rest, _ := strings.Cut(c, " ")
	switch strings.ToLower(w) {
	case "teks", "text", "info", "informasi", "keterangan", "helper":
		return "note", capital(strings.Trim(strings.TrimSpace(rest), `'"‘’“”`))
	case "alert", "banner", "peringatan", "warning":
		return "alert", ""
	}
	return "", ""
}

// formWith renders a form of the inputs fs. in is the dialog it is
// drawn in, which holds the buttons; nil for (a section of) the page's
// form, sent as pageSubmit says. A
// required field is marked *, an optional one "opsional"; the browser
// checks them before the form is sent. The form the URL shows sent
// incomplete marks its empty required fields; the one whose action
// failed says why and marks the field the error state names.
func (r *render) formWith(s core.Section, fs []formField, in *dialog) {
	action := r.doc.Self
	if action == "" {
		action = "#"
	}
	view := ""
	if in != nil {
		view = in.id
	}
	submitLabel, sendHere, formAttr, formID := "", true, "", ""
	if in != nil {
		submitLabel = submitOf(strings.Join(in.buttons, ", "))
	} else {
		var at string
		at, submitLabel = r.pageSubmit()
		sendHere = at == s.ID
		formAttr = ` form="` + r.pageFormID() + `"`
		if sendHere || at == "" && s.ID == r.mainForm() {
			formID = ` id="` + r.pageFormID() + `"`
		}
	}
	if submitLabel == "" {
		submitLabel = "Simpan"
	}
	var labels []string
	required := 0
	for _, f := range fs {
		labels = append(labels, f.label)
		if f.required() && f.kind != "check" && f.kind != "bool" {
			required++
		}
	}
	r.forms = append(r.forms, formRec{view: view, submit: submitLabel, required: required})
	here := r.doc.Sim.View == view
	fb := ""
	if here {
		fb = r.doc.Sim.Feedback
	}
	var fail failure
	switch {
	case fb == "error" && in != nil:
		fail = r.failure(submitLabel, labels)
	case fb == "error":
		fail = r.pageFail(submitLabel)
	}
	fill := here && (r.doc.Sim.Fill || fb == "error")

	sim := r.outcome(submitLabel, view)
	if t := r.matchTransition(submitLabel); t != nil && sim.Feedback == "ok" {
		sim.Status = t.T.To
	}
	// Signing in leads to the first page of the menu.
	if home := r.homePage(); r.page.Pattern == "auth" && sim.Feedback == "ok" && r.doc.live() && home != "" && r.doc.Link != nil {
		action, sim = r.doc.Link(home), r.doc.Sim.Nav()
	}
	r.f(`<form class="fk-form"%s method="get" action="%s" data-foundry-sim="form">`, formID, esc(action))
	if in != nil || formID != "" {
		r.hidden(sim)
	}
	// One alert per form: the page's spread over several sections says it
	// in its main one.
	if in != nil || s.ID == r.mainForm() {
		switch fb {
		case "invalid":
			r.alert("invalid", "Data belum lengkap", "Lengkapi field bertanda * lalu kirim lagi.", "", "")
		case "error":
			// The field the failure names says it; the alert points there.
			r.alert("error", fail.title, fail.body, fail.code, fail.spec)
		}
	}
	lock := r.formLocked()
	for i, f := range fs {
		id := fmt.Sprintf("f-%s-%d", ident(s.ID), i)
		r.field(f, id, formAttr, lock, fb, fail, fill)
		if r.state == "error" && i == 0 {
			r.f(`<p class="fk-help" style="color:var(--status-error)">%s</p>`, esc(r.stateText(&s, "error")))
		}
	}
	r.inForm = true
	defer func() { r.inForm = false }()
	switch {
	case in != nil:
		r.dialogButtons(in.buttons, false, in.pop, in.open, true)
	case sendHere && r.page.Pattern == "auth":
		r.w(`<div class="fk-actions">`)
		r.actionControl(submitLabel, "fk-btn fk-btn-primary", true)
		r.w(`</div>`)
	case sendHere:
		r.w(`<div class="fk-actions fk-form-bar">`)
		r.requiredNote()
		r.back("Batal", "fk-btn")
		r.actionControl(submitLabel, "fk-btn fk-btn-primary", true)
		r.w(`</div>`)
	}
	r.w(`</form>`)
}

// barSections are the sections a form page's buttons may stand in: its
// actions, and an aside beside the form.
func (r *render) barSections() []core.Section {
	var out []core.Section
	pat, ok := r.kit.Pattern(r.page.Pattern)
	for _, s := range r.kit.AssignSlots(r.page.Pattern, r.page.Sections) {
		if core.DialogSection(r.page.Pattern, s) {
			continue
		}
		if s.Slot == "actions" {
			out = append(out, s)
			continue
		}
		if !ok || r.page.Pattern != "form" {
			continue // a page without a template has no aside
		}
		if sl, found := pat.Slot(s.Slot); found && sl.Kind == "text" {
			out = append(out, s)
		}
	}
	return out
}

func (r *render) hasActionsSlot() bool {
	for _, s := range r.kit.AssignSlots(r.page.Pattern, r.page.Sections) {
		if s.Slot == "actions" {
			return true
		}
	}
	return false
}

// formSections are the sections drawn as the page's own form, in order.
func (r *render) formSections() []core.Section {
	var out []core.Section
	pat, hasPat := r.kit.Pattern(r.page.Pattern)
	for _, s := range r.kit.AssignSlots(r.page.Pattern, r.page.Sections) {
		form := guessKind(s) == "form" && !core.DialogSection(r.page.Pattern, s)
		if hasPat {
			sl, ok := pat.Slot(s.Slot)
			form = ok && sl.Kind == "form"
		}
		if form && takesInput(s) {
			out = append(out, s)
		}
	}
	return out
}

// mainForm is the page's first form section: it says what failed.
func (r *render) mainForm() string {
	if fs := r.formSections(); len(fs) > 0 {
		return fs[0].ID
	}
	return ""
}

// pageSubmit is where the page's form is sent from and with what: the
// form section naming its button, else the actions section's first
// button (id ""), else the last form section with the primary action.
// The page's form spans its sections: every field belongs to the form
// with that button, so the browser checks them all.
func (r *render) pageSubmit() (id, label string) {
	fs := r.formSections()
	if len(fs) == 0 {
		return "", ""
	}
	for _, s := range fs {
		for _, c := range s.Content {
			for _, seg := range strings.Split(c, ";") {
				if b, ok := buttonLabel(strings.TrimSpace(seg)); ok {
					return s.ID, submitOf(b)
				}
			}
		}
	}
	id = fs[len(fs)-1].ID
	// The buttons of the actions or an aside of a form page send it.
	for _, s := range r.barSections() {
		var all []string
		for _, c := range s.Content {
			if b, ok := buttonLabel(strings.TrimSpace(c)); ok {
				all = append(all, buttonActions(b)...)
			}
		}
		if len(all) > 0 {
			if l := sendButton(all); l != "" {
				return "", capital(l)
			}
			return "", capital(all[0])
		}
	}
	if r.hasActionsSlot() {
		id = ""
	}
	if label = r.page.PrimaryAction; label == "" {
		label = "Simpan"
	}
	return id, label
}

// submitOf is the sending button a button line names: "Simpan mutasi dan
// Batal" is Simpan mutasi.
func submitOf(b string) string {
	acts := buttonActions(b)
	if l := sendButton(acts); l != "" {
		return capital(l)
	}
	if len(acts) > 0 {
		return capital(acts[0])
	}
	return capital(b)
}

// buttonEnd ends the names of a button line; what follows says what the
// button needs, opens or limits ("Tombol Ubah data (item:update), membuka
// barang-ubah", "Tombol Hapus hanya untuk barang tanpa riwayat").
var buttonEnd = regexp.MustCompile(`(?i)\s*\(|\s*[,;:]|\s+(hanya|untuk|dengan|bila|jika|saat|ketika|membuka|menuju|sebagai|setelah|sebelum|bagi|oleh)\b`)

// limitWords start a part of a button line that limits who or when: it is
// kept as the button's tooltip.
var limitWords = regexp.MustCompile(`(?i)^\s+(hanya|untuk|bagi)\b[^,;(]*`)

// buttonActions are the actions a button line names: "Simpan draf dan
// Konfirmasi pesanan" names two, "Aktifkan atau Nonaktifkan" one that
// turns the record's state (Nonaktifkan), and what follows the names (a
// permission, the page it opens, "dengan konfirmasi") names none.
func buttonActions(b string) []string {
	b = strings.TrimSpace(b)
	names, rest := b, ""
	if loc := buttonEnd.FindStringIndex(b); loc != nil && loc[0] > 0 {
		names, rest = b[:loc[0]], b[loc[0]:]
	}
	limit := ""
	if m := limitWords.FindString(fontNoteRe.ReplaceAllString(rest, "")); m != "" {
		limit = " " + strings.TrimSpace(m)
	}
	var out []string
	for _, n := range strings.Split(strings.NewReplacer(" atau ", "|", " dan ", "|", " or ", "|", " and ", "|", " / ", "|").Replace(names), "|") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, capital(n)+limit)
		}
	}
	// A pair that turns a state shows the way out of the usual one.
	if len(out) == 2 {
		a, c := strings.ToLower(actionName(out[0])), strings.ToLower(actionName(out[1]))
		switch {
		case c == "non"+a || c == "non-"+a:
			out = out[1:]
		case a == "non"+c || a == "non-"+c:
			out = out[:1]
		}
	}
	return out
}

// fontNoteRe is a note in parentheses.
var fontNoteRe = regexp.MustCompile(`\s*\([^)]*\)`)

// sendButton is the button of a list that sends the form: the first that
// neither leaves it nor keeps a draft ("Simpan draf" is the other way to
// send). "" when every button leaves.
func sendButton(buttons []string) string {
	first := ""
	for _, l := range buttons {
		low := strings.ToLower(l)
		if has(low, "batal", "tutup", "kembali", "cancel", "close") {
			continue
		}
		if !has(low, "draf", "draft") {
			return l
		}
		if first == "" {
			first = l
		}
	}
	return first
}

// pageFormID is the id of the page's form element.
func (r *render) pageFormID() string { return "pf-" + ident(r.page.ID) }

// placeholder is the hint inside an empty input.
func placeholder(label string) string {
	l := strings.ToLower(label)
	switch {
	case has(l, "email", "surel"):
		return "nama@contoh.id"
	case has(l, "sandi", "password"):
		return "••••••••"
	case has(l, "telepon", "phone", "hp", "whatsapp"):
		return "08xx-xxxx-xxxx"
	}
	return "Masukkan " + l
}

// buttonLabel strips "Tombol"/"Button" from an element that names a
// button; ok reports whether it named one.
func buttonLabel(it string) (string, bool) {
	l := strings.ToLower(it)
	for _, p := range []string{"tombol ", "button "} {
		if strings.HasPrefix(l, p) {
			b := strings.TrimSpace(it[len(p):])
			// "Tombol utama Registrasi siswa" says how it is drawn.
			if w, rest, ok := strings.Cut(b, " "); ok && slices.Contains([]string{"utama", "primer", "primary", "sekunder", "secondary"}, strings.ToLower(w)) {
				b = strings.TrimSpace(rest)
			}
			// "Tombol primer 'Kirim pengajuan' menempel di bawah" is Kirim
			// pengajuan.
			if m := quotedText.FindStringSubmatch(b); m != nil {
				b = strings.TrimSpace(m[1])
			}
			return b, true
		}
	}
	return it, false
}

func (r *render) actions(s core.Section) {
	var buttons, facts []string
	for _, c := range s.Content {
		if b, ok := buttonLabel(strings.TrimSpace(c)); ok {
			buttons = append(buttons, buttonActions(b)...)
			continue
		}
		for _, it := range items(core.Section{Content: []string{c}}) {
			// "Input uang, selisih, tombol Konfirmasi serah terima"
			if b, ok := buttonLabel(it); ok {
				buttons = append(buttons, buttonActions(b)...)
			} else {
				facts = append(facts, it)
			}
		}
	}
	if len(buttons) == 0 {
		buttons, facts = facts, nil
	}
	if len(buttons) == 0 && r.page.PrimaryAction != "" {
		buttons = []string{r.page.PrimaryAction}
	}
	if len(facts) > 0 {
		r.w(`<dl class="fk-dl" style="margin-bottom:var(--space-md)">`)
		for i, it := range facts {
			r.f(`<div><dt>%s</dt><dd>%s</dd></div>`, esc(capital(it)), esc(textOf(r.cellValue(it, i))))
		}
		r.w(`</dl>`)
	}
	cancel := func(b string) bool { return has(strings.ToLower(b), "batal", "kembali", "cancel") }
	primary := ""
	if len(buttons) > 0 {
		primary = buttons[0]
	}
	bar := ""
	if r.formPage() {
		// A form's bar: what leaves it first, what sends it last.
		if b := sendButton(buttons); b != "" {
			primary = b
		}
		var back, rest []string
		for _, b := range buttons {
			if cancel(b) {
				back = append(back, b)
			} else {
				rest = append(rest, b)
			}
		}
		buttons, bar = append(back, rest...), " fk-form-bar"
	}
	r.f(`<div class="fk-actions%s">`, bar)
	if bar != "" {
		r.requiredNote()
	}
	for _, it := range buttons {
		cls := "fk-btn"
		switch {
		case it == primary && (bar == "" || !cancel(it)):
			cls += " fk-btn-primary"
		case cancel(it):
			if bar == "" {
				cls += " fk-btn-ghost"
			}
		case core.DangerAction(it):
			cls += " fk-btn-danger"
		}
		opens := ""
		if has(strings.ToLower(it), "batal", "kembali", "cancel") && r.disabled() == "" && r.parentHref() != "" {
			r.back(it, cls)
			continue
		}
		if r.disabled() == "" {
			if t := r.dialogFor(it); t != "" {
				opens = ` popovertarget="` + t + `"`
			} else if at, _ := r.pageSubmit(); it == primary && at == "" && r.mainForm() != "" {
				opens = ` type="submit" form="` + r.pageFormID() + `"`
			}
		}
		if opens == "" && r.disabled() == "" {
			// An action no dialog or form takes leads where it says, or
			// the prototype answers it.
			text := capital(strings.TrimSpace(actionName(it)))
			if f := strings.Fields(it); cancel(it) && len(f) > 1 {
				text = capital(f[0])
			}
			if text == "" {
				text = it
			}
			title := ""
			if text != it {
				title = ` title="` + esc(capital(it)) + `"`
			}
			if r.actionLink(it, nil, cls, title, actionIcon(text)+esc(text)) {
				continue
			}
		}
		if f := strings.Fields(it); cancel(it) && len(f) > 1 {
			// "Batal kembali ke daftar" is Batal, as on the live page.
			opens += ` title="` + esc(capital(it)) + `"`
			it = capital(f[0])
		}
		// The button names the action; when it holds is its tooltip.
		if text := capital(strings.TrimSpace(actionName(it))); text != "" && text != it && r.disabled() == "" {
			opens += ` title="` + esc(it) + `"`
			it = text
		}
		r.f(`<button class="%s"%s%s>%s%s</button>`, cls, r.disabled(), opens, actionIcon(it), esc(it))
	}
	r.w(`</div>`)
}

// chart is a sample trend: an area line against the previous period.
func (r *render) chart(s core.Section) {
	cur := []int{42, 55, 48, 62, 58, 71, 66, 74, 69, 81, 77, 88}
	prev := []int{38, 41, 45, 44, 50, 52, 49, 56, 58, 57, 62, 64}
	path := func(v []int) string {
		var b strings.Builder
		for i, y := range v {
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(&b, "%s%d,%d ", cmd, i*600/(len(v)-1), 200-y*2)
		}
		return strings.TrimSpace(b.String())
	}
	r.w(`<div class="fk-chart-wrap">`)
	if len(s.Content) > 0 {
		r.w(`<p class="fk-meta">` + esc(strings.Join(s.Content, " · ")) + `</p>`)
	}
	r.w(`<div class="fk-legend"><span><i></i>Periode ini</span><span><i class="alt"></i>Periode lalu</span></div>`)
	r.w(`<svg class="fk-chart" viewBox="0 0 600 200" preserveAspectRatio="none" role="img" aria-label="` + esc(s.Name) + `">`)
	for _, y := range []int{20, 70, 120, 170} {
		r.f(`<line class="grid" x1="0" x2="600" y1="%d" y2="%d" vector-effect="non-scaling-stroke"/>`, y, y)
	}
	r.f(`<path class="area" d="%s L600,200 L0,200 Z"/>`, path(cur))
	r.f(`<path class="line alt" d="%s" vector-effect="non-scaling-stroke"/>`, path(prev))
	r.f(`<path class="line" d="%s" vector-effect="non-scaling-stroke"/>`, path(cur))
	r.w(`</svg><div class="fk-axis"><span>Jan</span><span>Mar</span><span>Mei</span><span>Jul</span><span>Sep</span><span>Nov</span></div></div>`)
}

func (r *render) activity(s core.Section) {
	els := items(s)
	if len(els) == 0 {
		els = []string{s.Name}
	}
	r.w(`<ul class="fk-timeline">`)
	for i, it := range els {
		if i == 6 {
			break
		}
		r.f(`<li><div class="fk-list-main"><span class="fk-list-title">%s</span><span class="fk-meta">%s</span></div></li>`, esc(capital(it)), esc(sample("waktu", i)))
	}
	r.w(`</ul>`)
}

// steps is a stepper of the section's step names, numbered by the
// stepper itself. Content that names no step is left to the quality gate;
// one step is no stepper and the section shows as text.
func (r *render) steps(s core.Section) {
	els, notes := stepList(s)
	r.steppers++
	r.contentIssues = append(r.contentIssues, stepIssues(r.page, s, els, notes)...)
	if len(els) < 2 {
		r.text(s)
		return
	}
	cur, body := 0, r.page.Pattern == "wizard" && len(r.formSections()) == 0 && len(r.plan().unplaced) > 0
	if body {
		cur = r.wizardStep(len(els))
	}
	r.w(`<ol class="fk-steps">`)
	for i, it := range els {
		switch {
		case i == cur:
			r.f(`<li class="on" aria-current="step"><b>%d</b>%s</li>`, i+1, esc(it))
		case i < cur:
			// A step done is one to go back to.
			sim := r.doc.Sim.Nav()
			sim.View = stepView(i)
			r.f(`<li class="done"><a href="%s"><b>%s</b>%s</a></li>`, esc(r.doc.hrefSim(r.page.ID, sim)), icon("check"), esc(it))
		default:
			r.f(`<li><b>%d</b>%s</li>`, i+1, esc(it))
		}
	}
	r.w(`</ol>`)
	if body {
		r.wizardBody(els, cur)
	}
}

// notice is an inline message, such as the error a form shows.
func (r *render) notice(s core.Section) {
	r.w(`<div class="fk-alert" role="alert">` + icon("alert") + `<span>` + esc(strings.Join(s.Content, " · ")) + `</span></div>`)
}

func (r *render) tabs(s core.Section) {
	tabs := tabItems(s)
	if len(tabs) == 0 {
		tabs = items(s)
	}
	r.tabNav("fk-tabs", s.Name, tabs)
}

func (r *render) facets(s core.Section) {
	r.w(`<div class="fk-facets">`)
	for _, it := range r.labels(s, 6, "filter") {
		r.f(`<div class="fk-field"><label>%s</label><label class="fk-check"><span></span>Semua</label><label class="fk-check"><span></span>%s</label></div>`, esc(it), esc(sample(it, 1)))
	}
	r.w(`</div>`)
}

func (r *render) text(s core.Section) {
	// On a form page the buttons a section names send the form.
	var buttons []string
	if r.formPage() {
		var rest []string
		for _, c := range s.Content {
			if _, ok := buttonLabel(strings.TrimSpace(c)); ok {
				buttons = append(buttons, c)
			} else {
				rest = append(rest, c)
			}
		}
		s.Content = rest
	}
	if len(buttons) > 0 && slices.ContainsFunc(s.Components, displayComponent) {
		// Beside a form, a summary of what it will send, then its buttons.
		r.formSummary(s)
		r.actions(core.Section{ID: s.ID, Content: buttons})
		return
	}
	r.w(`<div class="fk-text">`)
	if s.Purpose != "" {
		r.w(`<p class="fk-meta">` + esc(s.Purpose) + `</p>`)
	}
	if len(s.Content) > 0 {
		r.w(`<ul>`)
		for _, c := range s.Content {
			r.w(`<li>` + esc(c) + `</li>`)
		}
		r.w(`</ul>`)
	}
	r.w(`</div>`)
	if len(buttons) > 0 {
		r.actions(core.Section{ID: s.ID, Content: buttons})
	}
}

// Sample data.

func has(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func hasAny(xs []string, pred func(string) bool) bool {
	for _, x := range xs {
		if pred(strings.ToLower(x)) {
			return true
		}
	}
	return false
}

func status(l string) bool { return has(l, "status", "state", "kondisi") }

func money(l string) bool {
	return has(l, "total", "harga", "nominal", "biaya", "price", "amount", "saldo", "pendapatan", "tarif", "subtotal", "omzet", "rp", "bayar", "tagihan", "pajak", "ppn", "tax")
}

func numeric(l string) bool {
	return money(l) || has(l, "jumlah", "qty", "stok", "kuantitas", "count", "skor", "nilai", "poin", "detik", "seconds")
}

// statusFor is a sample status of a field or section: payment, device and
// alert statuses read as such; anything else is a generic record status.
func statusFor(context string, row int) (string, string) {
	l := strings.ToLower(context)
	var w [][2]string
	switch {
	case has(l, "jenis", "tipe", "kategori", "kelas", "type"):
		w = [][2]string{{"Reguler", "info"}, {"Premium", "success"}, {"Reguler", "info"}, {"Khusus", "warning"}, {"Reguler", "info"}}
	case has(l, "peringatan", "alert", "insiden", "severity", "keparahan"):
		w = [][2]string{{"Kritis", "error"}, {"Peringatan", "warning"}, {"Info", "info"}, {"Kritis", "error"}, {"Peringatan", "warning"}}
	case has(l, "bayar", "payment", "tagihan", "invoice", "lunas"):
		w = [][2]string{{"Lunas", "success"}, {"Belum bayar", "warning"}, {"Lunas", "success"}, {"Gagal", "error"}, {"Diproses", "info"}}
	case has(l, "online", "offline", "perangkat", "koneksi", "device", "gerbang", "sinkron"):
		w = [][2]string{{"Online", "success"}, {"Online", "success"}, {"Offline", "error"}, {"Gangguan", "warning"}, {"Online", "success"}}
	case has(l, "stok", "persediaan", "inventori"):
		w = [][2]string{{"Tersedia", "success"}, {"Menipis", "warning"}, {"Tersedia", "success"}, {"Habis", "error"}, {"Tersedia", "success"}}
	case has(l, "job", "eksekusi", "proses"):
		w = [][2]string{{"Selesai", "success"}, {"Berjalan", "info"}, {"Antre", "warning"}, {"Gagal", "error"}, {"Selesai", "success"}}
	case has(l, "persetujuan", "approval", "verifikasi"):
		w = [][2]string{{"Disetujui", "success"}, {"Menunggu", "warning"}, {"Disetujui", "success"}, {"Ditolak", "error"}, {"Menunggu", "warning"}}
	default:
		return statusWord(row)
	}
	return w[row%len(w)][0], w[row%len(w)][1]
}

func statusWord(row int) (string, string) {
	w := [][2]string{{"Aktif", "success"}, {"Menunggu", "warning"}, {"Selesai", "info"}, {"Dibatalkan", "error"}, {"Diproses", "info"}}[row%5]
	return w[0], w[1]
}

// sample is a plausible placeholder for a field label, varied by row.
func sample(label string, row int) string {
	l := strings.ToLower(label)
	pick := func(xs ...string) string { return xs[row%len(xs)] }
	switch {
	case label == "":
		return pick("Item satu", "Item dua", "Item tiga", "Item empat", "Item lima", "Item enam")
	case has(l, "tanggal", "date", "jadwal", "dibuat", "tgl", "berlaku", "sampai", "kedaluwarsa", "expired", "jatuh tempo"):
		return pick("12 Nov 2026", "11 Nov 2026", "10 Nov 2026", "8 Nov 2026", "5 Nov 2026", "1 Nov 2026")
	case has(l, "alamat", "lokasi", "kota", "address"):
		return pick("Bandung", "Jakarta Selatan", "Surabaya", "Yogyakarta", "Medan", "Denpasar")
	case has(l, "plat", "nopol", "polisi", "kendaraan"):
		return pick("B 1234 KJT", "D 4521 AB", "B 7781 SZA", "F 1209 QR", "B 3310 PLM", "L 9012 XY")
	case has(l, "keluar", "selesai", "berakhir", "check-out"):
		return pick("10:42", "—", "11:20", "12:05", "—", "13:48")
	case has(l, "masuk", "mulai", "check-in", "datang", "tiba"):
		return pick("08:15", "08:42", "09:03", "09:27", "10:11", "10:56")
	case has(l, "durasi", "lama"):
		return pick("2 j 27 m", "45 m", "2 j 17 m", "2 j 38 m", "1 j 05 m", "3 j 12 m")
	case has(l, "jenis", "tipe", "kategori", "kelas"):
		return pick("Reguler", "Premium", "Reguler", "Khusus", "Reguler", "Premium")
	case has(l, "zona waktu", "timezone", "time zone"):
		return pick("WIB (UTC+7)", "WITA (UTC+8)", "WIT (UTC+9)")
	case has(l, "prioritas", "priority"):
		return pick("Tinggi", "Sedang", "Rendah")
	case has(l, "timeframe", "time frame", "kerangka waktu"):
		return pick("H1", "M15", "H4", "D1", "M5", "W1")
	case has(l, "simbol", "symbol", "ticker", "pair"):
		return pick("EURUSD", "XAUUSD", "BTCUSD", "GBPJPY", "US500", "USDJPY")
	case has(l, "detik", "seconds", "secs"):
		return pick("60", "300", "900", "3600", "120", "1800")
	case has(l, "menit", "minutes"):
		return pick("5", "15", "30", "60", "10", "45")
	case has(l, "waktu", "jam", "time"):
		return pick("2 menit lalu", "15 menit lalu", "1 jam lalu", "Kemarin", "3 hari lalu", "Minggu lalu")
	case money(l):
		return pick("Rp 1.250.000", "Rp 485.000", "Rp 2.100.000", "Rp 96.000", "Rp 730.000", "Rp 3.400.000")
	case l == "unit" || l == "satuan" || strings.HasPrefix(l, "satuan "):
		return pick("pcs", "kg", "box", "liter", "pak", "rim")
	case has(l, "jumlah", "qty", "stok", "kuantitas", "count"):
		return pick("24", "8", "130", "3", "57", "12")
	case has(l, "email", "surel"):
		return pick("andi@contoh.id", "sari@contoh.id", "budi@contoh.id", "rina@contoh.id", "dewi@contoh.id", "eko@contoh.id")
	case has(l, "telepon", "phone", "whatsapp", "no hp", "nomor hp", "ponsel", "kontak"):
		return pick("0812-3456-7890", "0813-2222-1010", "0857-1111-2020", "0819-8765-4321", "0821-5555-0101", "0878-9090-1212")
	case strings.HasPrefix(l, "nama ") && !has(l[5:], "lengkap", "pelanggan", "customer", "pasien", "pengguna", "user", "penulis", "pemilik", "karyawan",
		"siswa", "member", "anggota", "kontak", "pelapor", "wali", "admin", "guru", "pegawai", "petugas", "pengemudi", "staf", "penerima", "pengirim", "orang"):
		// "Nama barang" names a thing, not a person.
		return capital(l[5:]) + " " + pick("Alfa", "Beta", "Gamma", "Delta", "Prima", "Omega")
	case has(l, "nama", "name", "pelanggan", "customer", "pasien", "pengguna", "user", "penulis", "pemilik", "karyawan", "siswa"):
		return pick("Andi Pratama", "Sari Wulandari", "Budi Santoso", "Rina Kusuma", "Dewi Lestari", "Eko Saputra")
	case slices.Contains([]string{"nis", "nisn", "nik", "npsn", "nip", "npwp", "nim"}, l):
		// A national number is digits.
		return fmt.Sprintf("%010d", 51234561+row*7919)
	case l == "no" || has(l, "nomor", "kode", "id", "no.", "ref", "sku"):
		return fmt.Sprintf("#%04d", 1048-row*7)
	case status(l):
		w, _ := statusFor(label, row)
		return w
	}
	return fmt.Sprintf("%s %d", capital(label), row+1)
}

// back is a control that leaves the page for the one it is entered from;
// a plain button when there is none.
func (r *render) back(label, cls string) {
	// "Batal kembali ke daftar" is Batal; the rest is its tooltip.
	text, title := capital(label), ""
	if f := strings.Fields(label); len(f) > 1 && has(strings.ToLower(f[0]), "batal", "kembali", "cancel") {
		text, title = capital(f[0]), ` title="`+esc(capital(label))+`"`
	}
	if h := r.parentHref(); h != "" && r.disabled() == "" {
		r.f(`<a class="%s" href="%s" data-foundry-link="%s"%s>%s</a>`, cls, esc(h), esc(core.ParentPage(r.doc.Layout, r.page.ID)), title, esc(text))
		return
	}
	if r.disabled() == "" {
		// Without a parent it goes where a link says, to the page that
		// leads here, else it discards the changes: the page afresh.
		if pg := r.linkFor(label); pg != "" {
			r.f(`<a class="%s" href="%s" data-foundry-link="%s"%s>%s</a>`, cls, esc(r.doc.href(pg)), esc(pg), title, esc(text))
			return
		}
		if pg := r.cameFrom(); pg != "" {
			r.f(`<a class="%s" href="%s" data-foundry-link="%s"%s>%s</a>`, cls, esc(r.doc.href(pg)), esc(pg), title, esc(text))
			return
		}
		if r.doc.Link != nil {
			r.f(`<a class="%s" href="%s"%s>%s</a>`, cls, esc(r.doc.hrefSim(r.page.ID, r.doc.Sim.Nav())), title, esc(text))
			return
		}
	}
	r.f(`<button class="%s" type="button"%s%s>%s</button>`, cls, title, r.disabled(), esc(text))
}

// cameFrom is the one other page whose links lead here.
func (r *render) cameFrom() string {
	found := ""
	for _, pg := range r.doc.Layout.Pages {
		if pg.ID == r.page.ID || !slices.ContainsFunc(pg.Links, func(l core.PageLink) bool { return l.To == r.page.ID }) {
			continue
		}
		if found != "" {
			return ""
		}
		found = pg.ID
	}
	return found
}

// formPage reports whether the page makes or changes a record: its
// actions are the form's bar.
func (r *render) formPage() bool {
	return (r.page.Pattern == "form" || r.page.Pattern == "wizard") && len(r.formSections()) > 0
}

// requiredNote says what the form's * means, when it marks any.
func (r *render) requiredNote() {
	var all []formField
	for _, s := range r.formSections() {
		all = append(all, r.plan().bySec[s.ID]...)
	}
	r.requiredNoteOf(all)
}

// homePage is the first page of the menu.
func (r *render) homePage() string {
	for _, g := range r.doc.Layout.Navigation {
		for _, it := range g.Items {
			if pg, _ := r.doc.page(strings.TrimSpace(it.Page)); pg != nil && pg.Pattern != "auth" {
				return pg.ID
			}
		}
	}
	return ""
}
