package uikit

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A detail page shows one record. Its sections describe the record in
// words ("Nomor, pelanggan, gudang", "PageHeader dengan breadcrumb dan
// badge status", "Stok saat ini sebagai angka dominan"); the page's
// fields say which data each value is. The page maps each named value to
// its page field and shows it as that data reads: the record's name as
// its title, its status as a badge, a figure large, a date as a date. The
// record's actions stand beside its title; tabs hold the values their
// names cover; a history lists records, not column names.

// fact is one value of the record: its label and the page field it maps
// to (nil when the page names none).
type fact struct {
	label string
	rule  *core.FieldRule
}

// shownPres are the presentations a record's values are shown with.
var shownPres = []string{"card-title", "detail", "badge", "card-value", "summary"}

// uiWords name a part of the screen, not a value ("PageHeader dengan
// breadcrumb").
var uiWords = regexp.MustCompile(`(?i)^(pageheader|header|breadcrumb|metadata|meta|kpicard|statcard|card|kartu|badge|statusbadge|avatar|ikon|icon|maskedvalue|chip)\b\s*`)

// recordFacts are the values a section's lines name, mapped to the page's
// fields, with the actions and the lines that describe instead.
func (r *render) recordFacts(s core.Section, used []bool) (facts []fact, acts, notes []string) {
	shown := r.fields(shownPres...)
	for _, c := range s.Content {
		c = strings.TrimSpace(c)
		if b, ok := buttonLabel(c); ok && !strings.Contains(c, ";") {
			acts = append(acts, buttonActions(b)...)
			continue
		}
		if label, list, ok := strings.Cut(c, ":"); ok && actionLabel(label) {
			for _, a := range splitList(list) {
				acts = append(acts, capital(a))
			}
			continue
		}
		for _, seg := range strings.Split(c, ";") {
			if seg = strings.TrimSpace(seg); seg == "" {
				continue
			}
			if b, ok := buttonLabel(seg); ok {
				acts = append(acts, buttonList(b)...)
				continue
			}
			if a, ok := actionEntry(seg); ok {
				acts = append(acts, a...) // "Aksi kirim atau batalkan"
				continue
			}
			head := seg
			if h, _, ok := strings.Cut(seg, ":"); ok {
				head = h // "Status laporan: belum diisi, draf" is Status laporan
			}
			parts := partSplit.Split(fontNote.ReplaceAllString(head, ""), -1)
			named := 0
			for i, part := range parts {
				part = strings.TrimSpace(part)
				if b, ok := buttonLabel(part); ok {
					// "…, tombol Ubah dan Arsipkan": the rest are actions.
					acts = append(acts, capital(b))
					for _, a := range parts[i+1:] {
						acts = append(acts, capital(strings.TrimSpace(a)))
					}
					break
				}
				label := factLabel(part)
				if label == "" {
					continue
				}
				if j := matchShown(label, shown, used); j >= 0 {
					used[j] = true
					facts = append(facts, fact{capital(fieldLabel(shown[j])), &shown[j]})
					named++
					continue
				}
				if len(strings.Fields(label)) <= 4 {
					facts = append(facts, fact{label: label})
					named++
				}
			}
			if named == 0 {
				notes = append(notes, capital(seg))
			}
		}
	}
	return facts, acts, notes
}

// nameLabel reports whether a label is a record's name.
func nameLabel(label string) bool {
	l := strings.ToLower(label)
	return has(l, "nama", "name", "judul", "title") && !has(l, "pelanggan", "customer", "wali", "koordinator", "pengguna", "user", "oleh")
}

// buttonList are the actions a button line names ("Ubah dan Arsipkan").
func buttonList(b string) []string { return buttonActions(b) }

// factLabel is the value a part of a line names: without the screen part
// it sits in, the words joining it to the list, and how it is drawn.
func factLabel(part string) string {
	part = strings.TrimSpace(part)
	for {
		next := strings.TrimSpace(uiWords.ReplaceAllString(strings.TrimSpace(leadWords.ReplaceAllString(part, "")), ""))
		next = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(next, "dengan "), "with "))
		if next == part {
			break
		}
		part = next
	}
	label, _, _ := fieldSpec(part)
	if label == "" || !strings.ContainsFunc(label, func(r rune) bool { return r >= 'A' && r <= 'z' }) {
		return ""
	}
	return capital(label)
}

// matchShown is the page field a label names: one whose words it holds or
// that holds its words. -1 when none.
func matchShown(label string, shown []core.FieldRule, used []bool) int {
	for j, f := range shown {
		lb := fieldLabel(f)
		if !used[j] && (strings.EqualFold(lb, label) || core.StemsCovered(lb, label) || core.StemsCovered(label, lb)) {
			return j
		}
	}
	return -1
}

// factValue is how a value of the record reads: by its data when the
// blueprint says, else by its label.
func (r *render) factValue(f fact, row int) (text, badge string) {
	l := strings.ToLower(f.label)
	if f.rule != nil {
		if df, ok := r.doc.Project.DataField(f.rule.Field); ok {
			switch df.Kind {
			case "bool":
				return map[bool]string{true: "Aktif", false: "Nonaktif"}[row%4 != 3], map[bool]string{true: "success", false: "warning"}[row%4 != 3]
			case "enum":
				o := enumLabel(df.Options[row%len(df.Options)])
				if words := r.choiceWords(f.rule.Field, len(df.Options)); words != nil {
					o = words[row%len(words)]
				}
				return o, "info"
			case "money":
				return sample("nominal", row), ""
			case "number":
				return digits(sample(f.label, row), metricValue(f.label, row)), ""
			case "date":
				return sample("tanggal", row), ""
			case "datetime":
				return sample("tanggal", row) + ", " + sample("masuk", row), ""
			case "email":
				return sample("email", row), ""
			case "phone":
				return sample("telepon", row), ""
			}
		}
		if f.rule.Presentation == "badge" {
			w, c := statusFor(f.label, row)
			return w, c
		}
		if f.rule.Presentation == "card-value" && sample(f.label, row) == fmt.Sprintf("%s %d", capital(f.label), row+1) {
			return metricValue(f.label, row), "" // a figure, whatever it counts
		}
	}
	if status(l) {
		if lc, st := r.simLifecycle(), r.simState(); lc != nil && st != nil {
			return lc.StateLabel(st.ID), "info"
		}
		w, c := statusFor(f.label, row)
		return w, c
	}
	return sample(f.label, row), ""
}

// heroSection is the section that names the record on a detail page: the
// first of its summary or attribute slots.
func (r *render) heroSection() string {
	if r.page.Pattern != "detail" {
		return ""
	}
	pat, ok := r.kit.Pattern(r.page.Pattern)
	if !ok {
		return ""
	}
	secs := r.kit.AssignSlots(r.page.Pattern, r.page.Sections)
	for _, sl := range pat.Slots {
		for _, s := range inSlot(secs, sl.ID) {
			if core.DialogSection(r.page.Pattern, s) {
				continue
			}
			if k := r.kindOf(s, sl); k == "summary" || k == "detail" {
				return s.ID
			}
		}
	}
	return ""
}

// actionOnly reports whether a section only names buttons: the record's
// actions, drawn beside its title on a detail page.
func actionOnly(s core.Section) bool {
	if len(s.Content) == 0 {
		return false
	}
	for _, c := range s.Content {
		if _, ok := buttonLabel(strings.TrimSpace(c)); !ok {
			return false
		}
	}
	return true
}

// heroActions are the actions of the detail page's sections that only
// name buttons.
func (r *render) heroActions() []string {
	var out []string
	for _, s := range r.page.Sections {
		if core.DialogSection(r.page.Pattern, s) || !actionOnly(s) {
			continue
		}
		for _, c := range s.Content {
			b, _ := buttonLabel(strings.TrimSpace(c))
			out = append(out, buttonActions(b)...)
		}
	}
	return out
}

// hero draws the record a detail page shows: its title and status, its
// key figures, its other values, and its actions.
func (r *render) hero(s core.Section) {
	shown := r.fields(shownPres...)
	used := make([]bool, len(shown))
	facts, acts, notes := r.recordFacts(s, used)
	// The record's status and title belong here even when no line names
	// them.
	for j, f := range shown {
		if !used[j] && (f.Presentation == "badge" && status(strings.ToLower(fieldLabel(f))) || f.Presentation == "card-title") {
			used[j] = true
			facts = append(facts, fact{capital(fieldLabel(f)), &shown[j]})
		}
	}
	// Its name names it, mentioned or not.
	if !slices.ContainsFunc(facts, func(f fact) bool { return nameLabel(f.label) }) {
		for j, f := range shown {
			if !used[j] && nameLabel(fieldLabel(f)) {
				used[j] = true
				facts = append([]fact{{capital(fieldLabel(f)), &shown[j]}}, facts...)
				break
			}
		}
	}
	r.heroUsed = used
	acts = append(acts, r.heroActions()...)

	title, sub := "", ""
	var badges, figures, rest []fact
	for _, f := range facts {
		l := strings.ToLower(f.label)
		switch {
		case f.rule != nil && f.rule.Presentation == "card-title" && title == "":
			title, _ = r.factValue(f, 0)
		case title == "" && nameLabel(f.label):
			title, _ = r.factValue(f, 0)
		case status(l) || f.rule != nil && f.rule.Presentation == "badge":
			badges = append(badges, f)
		case f.rule != nil && f.rule.Presentation == "card-value":
			figures = append(figures, f)
		case sub == "" && has(l, "nomor", "kode", "sku", "nis", "no.", "id") && len(strings.Fields(l)) <= 2:
			v, _ := r.factValue(f, 0)
			sub = f.label + " " + v
			if title == "" {
				title, sub = recordNoun(r.page.Name)+" "+v, ""
			}
		default:
			rest = append(rest, f)
		}
	}
	if title == "" {
		title = recordNoun(r.page.Name)
	}
	r.w(`<div class="fk-hero">`)
	r.f(`<span class="fk-hero-icon" aria-hidden="true">%s</span><div class="fk-hero-id"><div class="fk-hero-title"><h3>%s</h3>`, r.heroIcon(title), esc(title))
	for i, b := range badges {
		v, c := r.factValue(b, i+1)
		if c == "" {
			c = "info"
		}
		r.f(`<span class="fk-badge fk-badge-%s" title="%s">%s</span>`, c, esc(b.label), esc(v))
	}
	r.w(`</div>`)
	if sub != "" {
		r.f(`<p class="fk-meta">%s</p>`, esc(sub))
	}
	r.w(`</div>`)
	r.recordActions(acts, "fk-hero-acts")
	r.w(`</div>`)
	for _, n := range notes {
		r.f(`<p class="fk-sec-desc">%s</p>`, esc(n))
	}
	if len(figures) > 0 {
		r.w(`<div class="fk-figures">`)
		for i, f := range figures {
			v, _ := r.factValue(f, i)
			r.f(`<div class="fk-figure"><span>%s</span><strong>%s</strong></div>`, esc(f.label), esc(v))
		}
		r.w(`</div>`)
	}
	r.factList(rest, false)
}

// factList draws values as label and value.
func (r *render) factList(facts []fact, plain bool) {
	if len(facts) == 0 {
		return
	}
	cls := "fk-facts"
	if plain {
		cls += " fk-facts-plain"
	}
	r.f(`<dl class="%s">`, cls)
	for i, f := range facts {
		v, badge := r.factValue(f, i)
		if badge != "" {
			r.f(`<div><dt>%s</dt><dd><span class="fk-badge fk-badge-%s">%s</span></dd></div>`, esc(f.label), badge, esc(v))
			continue
		}
		r.f(`<div><dt>%s</dt><dd>%s</dd></div>`, esc(f.label), esc(v))
	}
	r.w(`</dl>`)
}

// heroIcon is the record's mark: a person's initials, else its page's
// icon.
func (r *render) heroIcon(title string) string {
	if person(strings.ToLower(r.page.Name)) || strings.Contains(title, " ") && !strings.ContainsAny(title, "#0123456789") && r.titleIsPerson() {
		return esc(initials(title))
	}
	return icon(iconFor(strings.ToLower(recordNoun(r.page.Name))))
}

// titleIsPerson reports whether the page's record is a person: its title
// field names one.
func (r *render) titleIsPerson() bool {
	for _, f := range r.fields("card-title") {
		if person(strings.ToLower(fieldLabel(f))) || has(strings.ToLower(f.Field), "student", "user", "customer", "member", "employee") {
			return true
		}
	}
	return false
}

// recordNoun is what a detail page shows: "Detail Pesanan" is Pesanan.
func recordNoun(name string) string {
	f := strings.Fields(name)
	if len(f) > 1 && slices.Contains([]string{"detail", "rincian", "profil"}, strings.ToLower(f[0])) {
		return strings.Join(f[1:], " ")
	}
	return name
}

// recordActions draws a record's actions: the first a primary button, the
// destructive ones apart at the end.
func (r *render) recordActions(acts []string, cls string) {
	var safe, danger []string
	seen := map[string]bool{}
	for _, a := range acts {
		key := strings.ToLower(actionName(a))
		if a == "" || seen[key] || r.offeredAbove(a) || slices.ContainsFunc(r.headActs, func(h string) bool { return h != "" && strings.EqualFold(actionName(h), actionName(a)) }) {
			continue // the page header already offers it
		}
		seen[key] = true
		if core.DangerAction(a) {
			danger = append(danger, a)
		} else {
			safe = append(safe, a)
		}
	}
	if len(safe)+len(danger) == 0 {
		return
	}
	r.f(`<div class="fk-actions %s">`, cls)
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

// tabPanel draws a detail page's tabs and the open tab's values: the
// page's values not shown above, each in the tab whose name it shares a
// word with (else the tab before). The URL opens a tab (view tab-N).
func (r *render) tabPanel(s core.Section, tabs []string) {
	cur := 0
	if _, err := fmt.Sscanf(r.doc.Sim.View, "tab-%d", &cur); err != nil || cur < 1 || cur > len(tabs) {
		cur = 1
	}
	cur--
	r.w(`<nav class="fk-tabs-line" aria-label="` + esc(s.Name) + `">`)
	for i, t := range tabs {
		sim := r.doc.Sim.Nav()
		sim.View = fmt.Sprintf("tab-%d", i+1)
		cls := ""
		if i == cur {
			cls = ` class="on" aria-current="page"`
		}
		r.f(`<a href="%s"%s>%s</a>`, esc(r.doc.hrefSim(r.page.ID, sim)), cls, esc(t))
	}
	r.w(`</nav>`)
	shown := r.fields(shownPres...)
	var left []fact
	for j := range shown {
		if r.heroUsed == nil || j >= len(r.heroUsed) || !r.heroUsed[j] {
			left = append(left, fact{capital(fieldLabel(shown[j])), &shown[j]})
		}
	}
	per := make([][]fact, len(tabs))
	at := 0
	for _, f := range left {
		for k := at; k < len(tabs); k++ {
			if core.SharesStem(f.label, tabs[k]) {
				at = k
				break
			}
		}
		per[at] = append(per[at], f)
	}
	r.f(`<div class="fk-tab-panel" role="tabpanel" aria-label="%s">`, esc(tabs[cur]))
	switch {
	case len(per[cur]) > 0:
		r.factList(per[cur], true)
	case len(r.fields("table-column")) > 0 || slices.ContainsFunc(s.Components, func(c string) bool { return has(strings.ToLower(c), "table", "list") }):
		r.feed(r.columnsFor(tabs[cur]), 4)
	default:
		r.f(`<p class="fk-help">Isi tab %s belum dirinci di desain.</p>`, esc(strings.ToLower(tabs[cur])))
	}
	r.w(`</div>`)
}

// columnsFor are the columns of the records a tab lists: the page's table
// columns, else the tab's own words.
func (r *render) columnsFor(tab string) []fact {
	var out []fact
	for i, f := range r.fields("table-column", "list-item") {
		out = append(out, fact{capital(fieldLabel(f)), &r.fields("table-column", "list-item")[i]})
	}
	if len(out) == 0 {
		out = []fact{{label: capital(tab)}, {label: "Status"}, {label: "Waktu"}}
	}
	return out
}

// historyColumns are the columns a history line lists ("Sepuluh mutasi
// terakhir: Waktu, Tipe, Qty, Catatan, Oleh"), mapped to the page's
// fields; nil when the section lists none.
func (r *render) historyColumns(s core.Section) []fact {
	cols := r.fields("table-column", "list-item", "badge")
	for _, c := range s.Content {
		head, list, ok := strings.Cut(c, ":")
		// "Status laporan: belum diisi, draf, …" lists a value's states.
		if !ok || status(strings.ToLower(head)) {
			continue
		}
		parts := splitList(list)
		if len(parts) < 3 {
			continue
		}
		used := make([]bool, len(cols))
		var out []fact
		for _, p := range parts {
			p = factLabel(p)
			if p == "" {
				continue
			}
			if j := matchShown(p, cols, used); j >= 0 {
				used[j] = true
				out = append(out, fact{capital(fieldLabel(cols[j])), &cols[j]})
				continue
			}
			out = append(out, fact{label: p})
		}
		return out
	}
	return nil
}

// feed lists n records by their columns: what happened as its title (a
// kind as a badge, an amount bold), a note under it, when and by whom on
// the right.
func (r *render) feed(cols []fact, n int) {
	r.w(`<ul class="fk-feed">`)
	for row := 0; row < n; row++ {
		var title, note, when []string
		for _, c := range cols {
			l := strings.ToLower(c.label)
			v, badge := r.factValue(c, row)
			kind := ""
			if c.rule != nil {
				if df, ok := r.doc.Project.DataField(c.rule.Field); ok {
					kind = df.Kind
				}
			}
			switch {
			case kind == "datetime" || kind == "date" || has(l, "waktu", "tanggal", "dibuat", "jam", "time", "date"):
				when = append(when, sample("waktu", row))
			case has(l, "oleh", "pengguna", "user", "petugas", "by"):
				when = append(when, "oleh "+sample("nama", row))
			case badge != "":
				title = append(title, fmt.Sprintf(`<span class="fk-badge fk-badge-%s">%s</span>`, badge, esc(v)))
			case kind == "number" || kind == "money" || has(l, "qty", "jumlah", "nominal", "total"):
				title = append(title, `<b>`+esc(v)+`</b>`)
			case kind == "longtext" || has(l, "catatan", "keterangan", "note", "alasan"):
				note = append(note, "Contoh "+esc(strings.ToLower(c.label))+" "+fmt.Sprint(row+1)+".")
			case has(l, "penanda", "tanda", "flag"):
				if row == 2 {
					title = append(title, `<span class="fk-badge fk-badge-warning">`+esc(capital(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(l, "penanda"), "tanda"))))+`</span>`)
				}
			default:
				title = append(title, esc(v))
			}
		}
		r.w(`<li><div class="fk-feed-main"><span class="fk-feed-title">` + strings.Join(title, " ") + `</span>`)
		if len(note) > 0 {
			r.w(`<span class="fk-meta">` + strings.Join(note, " ") + `</span>`)
		}
		r.w(`</div>`)
		if len(when) > 0 {
			r.w(`<span class="fk-feed-when">` + esc(strings.Join(when, " · ")) + `</span>`)
		}
		r.w(`</li>`)
	}
	r.w(`</ul>`)
}

// digits are the figures of v, else def.
func digits(v, def string) string {
	out := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, v)
	if out == "" {
		return def
	}
	return out
}

// choiceWords are the n values the design writes for a field on any of
// its pages ("Pilihan jenis masuk/keluar" for StockMovement.type), so a
// value reads the same everywhere; nil when none does.
func (r *render) choiceWords(field string, n int) []string {
	for _, pg := range r.doc.Layout.Pages {
		for _, fr := range pg.Fields {
			if fr.Field != field {
				continue
			}
			for _, sec := range pg.Sections {
				for _, c := range sec.Content {
					if core.StemOverlap(c, fieldLabel(fr)) == 0 {
						continue
					}
					if alts := slashChoices(c); len(alts) == n {
						return alts
					}
					if head, list, ok := strings.Cut(c, ":"); ok && core.StemsCovered(fieldLabel(fr), head) {
						if out := choiceList(list); len(out) == n {
							return out
						}
					}
				}
			}
		}
	}
	return nil
}

// namesShown reports whether a section names a value the page shows of
// its record.
func (r *render) namesShown(s core.Section) bool {
	facts, _, _ := r.recordFacts(s, make([]bool, len(r.fields(shownPres...))))
	return slices.ContainsFunc(facts, func(f fact) bool { return f.rule != nil })
}
