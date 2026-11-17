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
	} else if k := core.ContentKind(s); k != "" && k != "metrics" && (!fixed || kind == "actions" && dataKinds[k]) && !(k == "list" && kind == "activity") {
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
	return kind
}

// collectionKind reports whether a block renders as a collection with its
// search/filter and pagination: tables and lists always, cards inside a
// data panel.
func (r *render) collectionKind(kind string) bool {
	return kind == "table" || kind == "list" || kind == "cards" && r.panelOpen
}

// section renders one section in its slot.
func (r *render) section(s core.Section, sl Slot, slotted bool) {
	kind := r.kindOf(s, sl)
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
	if r.panelOpen && (kind == "toolbar" || kind == "pagination" || kind == "tabbar") {
		h2 = `<h2 class="fk-sr">`
	}
	r.f(`<section class="%s" data-foundry-section="%s"%s aria-label="%s">%s%s</h2>`, cls, esc(s.ID), slot, esc(s.Name), h2, esc(s.Name))
	if dataKinds[kind] && r.dataState(s, sl.Required || !slotted, kind) {
		r.w(`</section>`)
		return
	}
	switch kind {
	case "toolbar":
		els := r.labels(s, 8, "filter")
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
	r.f(`<section class="fk-sec fk-coll fk-k-%s" data-foundry-section="%s"%s aria-label="%s">`, ident(kind), esc(s.ID), slot, esc(s.Name))
	if r.panelHead != s.ID {
		r.collHead(s)
	}
	if c.Filter == "" || c.Filter == "section" {
		r.w(`<div class="fk-tools-wrap">`)
		r.tools(filterElements(s, c.Filter == "", r.defaultFilters(s, kind)))
		r.w(`</div>`)
	}
	data := dataContent(s)
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
	r.w(`<nav class="fk-tabs-line" aria-label="` + esc(s.Name) + `">`)
	for i, t := range tabs {
		cls := ""
		if i == 0 {
			cls = ` class="on" aria-current="page"`
		}
		r.f(`<a href="#"%s>%s</a>`, cls, esc(t))
	}
	r.w(`</nav>`)
}

// collHead is a collection's header: name, record count and its tabs.
func (r *render) collHead(s core.Section) {
	r.f(`<header class="fk-sec-head"><h2>%s</h2><span class="fk-count">%d</span>`, esc(s.Name), total(s))
	if tabs := tabItems(s); len(tabs) > 0 {
		r.w(`<nav class="fk-tabs-line" aria-label="` + esc(s.Name) + `">`)
		for i, t := range tabs {
			cls := ""
			if i == 0 {
				cls = ` class="on" aria-current="page"`
			}
			r.f(`<a href="#"%s>%s</a>`, cls, esc(t))
		}
		r.w(`</nav>`)
	}
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
func dataContent(s core.Section) core.Section {
	out := s
	out.Content = nil
	for _, c := range s.Content {
		if entryKind(c) == "" {
			out.Content = append(out.Content, c)
		}
	}
	return out
}

// tabItems are the tabs a section names: "Tab Aktif dan Selesai".
func tabItems(s core.Section) []string {
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
		cols := r.tableColumns(dataContent(s))
		if what == "" && len(cols) > 0 {
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
		c = strings.TrimSpace(c)
		if l := strings.ToLower(c); strings.HasPrefix(l, "filter") {
			c = strings.TrimLeft(c[len("filter"):], " :")
			if c == "" {
				continue
			}
		}
		if strings.Contains(c, ":") {
			c = c[strings.Index(c, ":")+1:]
		}
		out = append(out, splitList(c)...)
	}
	if len(out) == 0 && defaults {
		out = def
	}
	return out
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
			buttons = append(buttons, capital(b))
		case search == "" && has(l, "cari", "search", "pencarian", "kata kunci"):
			search = capital(strings.TrimRight(e, ".… "))
		default:
			for _, p := range []string{"filter ", "saring ", "Filter ", "Saring "} {
				e = strings.TrimPrefix(e, p)
			}
			filters = append(filters, capital(e))
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
			r.w(`<button class="fk-btn">` + actionIcon(b) + esc(b) + `</button>`)
		}
		r.w(`</div>`)
	}
	r.w(`</div>`)
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
	if more {
		r.f(`<div class="fk-pager fk-pager-more"><button class="fk-btn">Muat lebih banyak</button><span>%d dari %d</span></div>`, size, all)
		return
	}
	pages := (all + size - 1) / size
	r.w(`<div class="fk-pager">`)
	if !compact {
		r.f(`<div class="fk-pager-size">Baris per halaman <span class="fk-chip fk-chip-solid">%d%s</span></div>`, size, icon("chevron"))
	}
	r.f(`<span>1–%d dari %d</span><nav class="fk-pages" aria-label="Paginasi"><button class="fk-pg fk-nav-pg" disabled aria-label="Sebelumnya">%s</button>`, size, all, icon("left"))
	if !compact {
		r.w(`<button class="fk-pg on" aria-current="page">1</button>`)
		for p := 2; p <= pages && p <= 3; p++ {
			r.f(`<button class="fk-pg">%d</button>`, p)
		}
		if pages > 4 {
			r.w(`<span class="fk-gap">…</span>`)
		}
		if pages > 3 {
			r.f(`<button class="fk-pg">%d</button>`, pages)
		}
	}
	r.f(`<button class="fk-pg fk-nav-pg" aria-label="Berikutnya">%s</button></nav></div>`, icon("right"))
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
		r.w(`<button class="fk-btn">Coba lagi</button>`)
	case "empty":
		if a := r.page.PrimaryAction; a != "" {
			r.w(`<button class="fk-btn fk-btn-primary">` + actionIcon(a) + esc(a) + `</button>`)
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
			for _, p := range strings.Split(c, ",") {
				if p = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(p), ".")); p != "" {
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
			if strings.Contains(value, ",") && !strings.ContainsAny(value, "0123456789") {
				out = append(out, strings.TrimSpace(label))
			} else {
				out = append(out, c)
			}
		case !strings.Contains(c, ","):
			out = append(out, c)
		default:
			for _, p := range strings.Split(c, ",") {
				if p = strings.TrimSpace(p); p != "" {
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
		got := items(core.Section{Content: []string{c}})
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
		cols = cols[:7]
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
			r.f(`<th scope="col">%s</th>`, esc(c))
		}
	}
	r.w(`</tr></thead><tbody>`)
	noun := entityNoun(s)
	for row := 0; row < 5; row++ {
		r.w(`<tr>`)
		for i, c := range cols {
			if l := strings.ToLower(c); i == 0 && (l == "nama" || l == "name") && noun != "" && !person(noun) {
				r.f(`<td data-label="%s"><span class="fk-strong">%s %s</span></td>`, esc(c), esc(capital(noun)), []string{"A1", "A2", "B1", "B2", "C1", "C2"}[row])
				continue
			}
			r.w(r.cell(c, row, i == 0))
		}
		r.w(`</tr>`)
	}
	r.w(`</tbody></table></div>`)
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
	case pres == "badge" || status(l):
		w, c := statusFor(label, row)
		return fmt.Sprintf(`%s><span class="fk-badge fk-badge-%s">%s</span></td>`, td, c, w)
	case numeric(l):
		return td + ` class="fk-num">` + esc(sample(label, row)) + `</td>`
	case first && person(l):
		v := sample(label, row)
		return td + `><span class="fk-who"><span class="fk-avatar">` + esc(initials(v)) + `</span><span class="fk-strong">` + esc(v) + `</span></span></td>`
	case first:
		return td + `><span class="fk-strong">` + esc(sample(label, row)) + `</span></td>`
	}
	return td + `>` + esc(sample(label, row)) + `</td>`
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
		t := sample(title, row)
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
			m = sample(meta, row) + " · " + m
		}
		r.w(`<span class="fk-meta">` + esc(m) + `</span></div>`)
		if badge {
			w, c := statusFor(s.Name+" "+badgeLabel, row)
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
		r.f(`<article class="fk-card"><div class="fk-card-media"></div><div class="fk-card-body"><span class="fk-list-title">%s</span>`, esc(sample(title, row)))
		if meta != "" {
			r.w(`<span class="fk-meta">` + esc(sample(meta, row)) + `</span>`)
		}
		if value != "" {
			r.w(`<span class="fk-card-value">` + esc(sample(value, row)) + `</span>`)
		}
		r.w(`</div><div class="fk-card-foot">`)
		if badge {
			w, c := statusFor(s.Name, row)
			r.f(`<span class="fk-badge fk-badge-%s">%s</span>`, c, w)
		} else {
			r.w(`<span class="fk-meta">` + esc(sample("tanggal", row)) + `</span>`)
		}
		r.w(`<button class="fk-icon-btn" aria-label="Aksi">` + icon("more") + `</button></div></article>`)
	}
	r.w(`</div>`)
}

func (r *render) detail(s core.Section) {
	els := r.labels(s, 12, "detail")
	var acts []string
	r.w(`<dl class="fk-dl">`)
	for i, it := range els {
		if a, ok := actionEntry(it); ok {
			acts = append(acts, a...)
			continue
		}
		if status(strings.ToLower(it)) {
			w, c := statusFor(it, i)
			r.f(`<div><dt>%s</dt><dd><span class="fk-badge fk-badge-%s">%s</span></dd></div>`, esc(capital(it)), c, w)
			continue
		}
		r.f(`<div><dt>%s</dt><dd>%s</dd></div>`, esc(capital(it)), esc(sample(it, i)))
	}
	r.w(`</dl>`)
	if len(acts) > 0 {
		r.w(`<div class="fk-actions fk-detail-actions">`)
		primary := false
		for _, a := range acts {
			if r.offeredAbove(a) {
				continue
			}
			cls := "fk-btn"
			switch {
			case core.DangerAction(a):
				cls = "fk-btn fk-btn-danger"
			case !primary:
				cls, primary = "fk-btn fk-btn-primary", true
			}
			r.actionControl(a, cls, false)
		}
		r.w(`</div>`)
	}
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
		if status(strings.ToLower(it)) {
			w, c := statusFor(it, i)
			r.f(`<div><span class="fk-meta">%s</span><span class="fk-badge fk-badge-%s">%s</span></div>`, esc(it), c, w)
			continue
		}
		r.f(`<div><span class="fk-meta">%s</span><strong>%s</strong></div>`, esc(it), esc(sample(it, i)))
	}
	r.w(`</div>`)
}

func (r *render) form(s core.Section) {
	var els []string
	submit := ""
	for _, it := range r.labels(s, 10, "form-input") {
		if b, ok := buttonLabel(it); ok {
			submit = b
			continue
		}
		for _, p := range []string{"Input ", "input ", "Field ", "field "} {
			it = strings.TrimPrefix(it, p)
		}
		if rs := []rune(it); len(rs) > 0 {
			it = strings.ToUpper(string(rs[:1])) + string(rs[1:])
		}
		els = append(els, it)
	}
	r.formWith(s, els, submit)
}

// formWith renders a form of the fields named els; submit is the label of
// its submit button ("" for the page's primary action).
func (r *render) formWith(s core.Section, els []string, submit string) {
	action := r.doc.Self
	if action == "" {
		action = "#"
	}
	r.f(`<form class="fk-form" method="get" action="%s" data-foundry-sim="form">`, esc(action))
	to, flash := r.doc.Sim.Status, "ok"
	submitLabel := submit
	if submitLabel == "" {
		submitLabel = r.page.PrimaryAction
	}
	if t := r.matchTransition(submitLabel); t != nil {
		to, flash = t.T.To, "ok"
	}
	r.hiddenSim(to, flash)
	lock := r.formLocked()
	for i, it := range els {
		l := strings.ToLower(it)
		id := fmt.Sprintf("f-%s-%d", ident(s.ID), i)
		req, dis := "", r.disabled()
		if lock {
			dis = " disabled"
		}
		if !has(l, "opsional", "optional") {
			req = " required"
		}
		switch {
		case has(l, "setuju", "ingat", "aktifkan", "centang"):
			r.f(`<label class="fk-check"><span></span>%s</label>`, esc(it))
		case has(l, "catatan", "deskripsi", "keterangan", "alamat", "pesan", "komentar", "note", "alasan"):
			r.f(`<div class="fk-field fk-field-wide"><label for="%s">%s</label><textarea id="%s" name="%s" class="fk-input" placeholder="Tulis %s"%s%s></textarea></div>`, id, esc(it), id, id, esc(strings.ToLower(it)), dis, req)
		case has(l, "status", "jenis", "kategori", "peran", "tipe", "role", "metode", "pilih", "zona", "cabang", "lokasi"):
			r.f(`<div class="fk-field"><label for="%s">%s</label><div id="%s" class="fk-select" role="combobox">Pilih %s%s</div></div>`, id, esc(it), id, esc(strings.ToLower(it)), icon("chevron"))
		case has(l, "tanggal", "berlaku", "jadwal", "date"):
			r.f(`<div class="fk-field"><label for="%s">%s</label><div id="%s" class="fk-select" role="textbox">%s%s</div></div>`, id, esc(it), id, esc(sample("tanggal", i)), icon("calendar"))
		default:
			typ := "text"
			switch {
			case has(l, "sandi", "password"):
				typ = "password"
			case has(l, "email", "surel"):
				typ = "email"
			}
			r.f(`<div class="fk-field"><label for="%s">%s</label><input id="%s" name="%s" type="%s" class="fk-input" placeholder="%s"%s%s></div>`, id, esc(it), id, id, typ, esc(placeholder(it)), dis, req)
		}
		if r.state == "error" && i == 0 {
			r.f(`<p class="fk-help" style="color:var(--status-error)">%s</p>`, esc(r.stateText(&s, "error")))
		}
	}
	if !r.hasActionsSlot() && s.ID == r.mainForm() {
		label := r.page.PrimaryAction
		if submit != "" {
			label = submit
		}
		if label == "" {
			label = "Simpan"
		}
		r.w(`<div class="fk-actions">`)
		r.actionControl(label, "fk-btn fk-btn-primary", true)
		if r.page.Pattern != "auth" {
			r.w(`<button class="fk-btn fk-btn-ghost" type="button">Batal</button>`)
		}
		r.w(`</div>`)
	}
	r.w(`</form>`)
}

func (r *render) hasActionsSlot() bool {
	for _, s := range r.kit.AssignSlots(r.page.Pattern, r.page.Sections) {
		if s.Slot == "actions" {
			return true
		}
	}
	return false
}

// mainForm is the page's first form section: it carries the submit button
// when no actions section does.
func (r *render) mainForm() string {
	for _, s := range r.kit.AssignSlots(r.page.Pattern, r.page.Sections) {
		if pat, ok := r.kit.Pattern(r.page.Pattern); ok {
			if sl, ok := pat.Slot(s.Slot); ok && sl.Kind == "form" {
				return s.ID
			}
		} else if guessKind(s) == "form" && !core.DialogSection(r.page.Pattern, s) {
			return s.ID
		}
	}
	return ""
}

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
			return strings.TrimSpace(it[len(p):]), true
		}
	}
	return it, false
}

func (r *render) actions(s core.Section) {
	els := items(s)
	var buttons, facts []string
	for _, it := range els {
		if b, ok := buttonLabel(it); ok {
			buttons = append(buttons, b)
		} else {
			facts = append(facts, it)
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
			r.f(`<div><dt>%s</dt><dd>%s</dd></div>`, esc(capital(it)), esc(sample(it, i)))
		}
		r.w(`</dl>`)
	}
	r.w(`<div class="fk-actions">`)
	for i, it := range buttons {
		cls := "fk-btn"
		if i == 0 {
			cls += " fk-btn-primary"
		} else if has(strings.ToLower(it), "batal", "kembali", "cancel") {
			cls += " fk-btn-ghost"
		}
		r.f(`<button class="%s"%s>%s%s</button>`, cls, r.disabled(), actionIcon(it), esc(it))
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
	r.w(`<ol class="fk-steps">`)
	for i, it := range els {
		cls := ""
		if i == 0 {
			cls = ` class="on" aria-current="step"`
		}
		r.f(`<li%s><b>%d</b>%s</li>`, cls, i+1, esc(it))
	}
	r.w(`</ol>`)
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
	r.w(`<nav class="fk-tabs" aria-label="` + esc(s.Name) + `">`)
	for i, it := range tabs {
		cls := ""
		if i == 0 {
			cls = ` class="on" aria-current="page"`
		}
		r.f(`<a href="#"%s>%s</a>`, cls, esc(it))
	}
	r.w(`</nav>`)
}

func (r *render) facets(s core.Section) {
	r.w(`<div class="fk-facets">`)
	for _, it := range r.labels(s, 6, "filter") {
		r.f(`<div class="fk-field"><label>%s</label><label class="fk-check"><span></span>Semua</label><label class="fk-check"><span></span>%s</label></div>`, esc(it), esc(sample(it, 1)))
	}
	r.w(`</div>`)
}

func (r *render) text(s core.Section) {
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
	return has(l, "total", "harga", "nominal", "biaya", "price", "amount", "saldo", "pendapatan", "tarif", "subtotal", "omzet", "rp", "bayar", "tagihan")
}

func numeric(l string) bool {
	return money(l) || has(l, "jumlah", "qty", "stok", "kuantitas", "count", "skor", "nilai", "poin")
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
	case has(l, "waktu", "jam", "time"):
		return pick("2 menit lalu", "15 menit lalu", "1 jam lalu", "Kemarin", "3 hari lalu", "Minggu lalu")
	case money(l):
		return pick("Rp 1.250.000", "Rp 485.000", "Rp 2.100.000", "Rp 96.000", "Rp 730.000", "Rp 3.400.000")
	case has(l, "jumlah", "qty", "stok", "kuantitas", "count"):
		return pick("24", "8", "130", "3", "57", "12")
	case has(l, "email", "surel"):
		return pick("andi@contoh.id", "sari@contoh.id", "budi@contoh.id", "rina@contoh.id", "dewi@contoh.id", "eko@contoh.id")
	case has(l, "telepon", "phone", "whatsapp", "no hp", "nomor hp", "ponsel"):
		return pick("0812-3456-7890", "0813-2222-1010", "0857-1111-2020", "0819-8765-4321", "0821-5555-0101", "0878-9090-1212")
	case has(l, "nama", "name", "pelanggan", "customer", "pasien", "pengguna", "user", "penulis", "pemilik", "karyawan", "siswa"):
		return pick("Andi Pratama", "Sari Wulandari", "Budi Santoso", "Rina Kusuma", "Dewi Lestari", "Eko Saputra")
	case l == "no" || has(l, "nomor", "kode", "id", "no.", "ref"):
		return fmt.Sprintf("#%04d", 1048-row*7)
	case status(l):
		w, _ := statusFor(label, row)
		return w
	}
	return fmt.Sprintf("%s %d", capital(label), row+1)
}
