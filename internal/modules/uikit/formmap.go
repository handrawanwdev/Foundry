package uikit

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A form section of the design describes its inputs in words ("SKU dan
// nama", "SearchSelect barang dengan pencarian SKU atau nama"); the page's
// fields (presentation form-input) say which data they hold ("Item.sku")
// and the blueprint says what that data is ("sku unique", "qty > 0"). The
// form maps each described input to its page field and draws the control
// that data needs: a number with its bounds, a choice of its values, a
// search over the records it points at. A line that names no input is
// the section's description.

// formField is one input of a form.
type formField struct {
	label, hint string
	// kind is the control: text, longtext, number, money, date, datetime,
	// bool, check, email, phone, password, url, select, segmented,
	// combobox or file.
	kind     string
	options  []string
	optional bool
	readonly bool
	unique   bool
	min      string
	step     string
	maxLen   int
	def      string // the value it starts with
	chips    bool   // quick values above a number
	raw      string // the design's words
}

// formPlan is the page's form: the inputs of each form section, in order,
// the lines that describe a section, and the inputs no section places.
type formPlan struct {
	bySec    map[string][]formField
	desc     map[string][]string
	unplaced []formField
}

// control words name the control a line describes.
var (
	checkWords   = []string{"centang", "checkbox", "setuju", "ingat", "aktifkan"}
	switchWords  = []string{"toggle", "switch", "sakelar"}
	segWords     = []string{"segmen", "segmented", "radio"}
	searchWords  = []string{"searchselect", "combobox", "pencarian", "autocomplete", "pemilih", "cari "}
	pickWords    = []string{"pilih", "pilihan", "select", "dropdown"}
	fileWords    = []string{"unggah", "upload", "lampiran"}
	lockWords    = []string{"terkunci", "read-only", "readonly", "tidak bisa diubah", "tidak dapat diubah"}
	nowWords     = []string{"terisi sekarang", "default sekarang", "waktu sekarang", "otomatis sekarang"}
	explicitHead = regexp.MustCompile(`(?i)^(field|input|textarea|kolom|pemilih|pilih|pilihan|searchselect|select|dropdown|combobox|centang|checkbox|toggle|switch|unggah|upload|radio|segmented)\b`)
	defaultWord  = regexp.MustCompile(`(?i)\bdefault\s+([A-Za-z0-9_-]+)`)
	partSplit    = regexp.MustCompile(`(?i)\s*,\s*(?:dan\s+|atau\s+|and\s+|or\s+)?|\s+(?:dan|atau|and|or)\s+`)
)

// pickHeads start a line naming a control before its field ("Pemilih
// kanal pencatatan").
var pickHeads = []string{"pemilih", "pilih", "pilihan", "kolom", "searchselect", "combobox", "centang", "checkbox", "unggah", "upload", "toggle", "switch"}

// plan maps the page's form sections to their inputs, once per page.
func (r *render) plan() *formPlan {
	if r.fplan != nil {
		return r.fplan
	}
	p := &formPlan{bySec: map[string][]formField{}, desc: map[string][]string{}}
	r.fplan = p
	inputs := r.fields("form-input")
	secs := r.formSections()
	if len(inputs) == 0 {
		// The design names the inputs only in its sections.
		for _, s := range secs {
			els, notes := r.formFields(s)
			p.desc[s.ID] = notes
			for _, it := range els {
				p.bySec[s.ID] = append(p.bySec[s.ID], r.fieldOf(it, nil, s))
			}
		}
		return p
	}
	used := make([]bool, len(inputs))
	at := make([]string, len(inputs))     // the section each input is placed in
	placed := make([]string, len(inputs)) // and its label there
	for _, s := range secs {
		for _, c := range s.Content {
			switch kind, text := formLine(c); {
			case kind == "note":
				p.desc[s.ID] = append(p.desc[s.ID], text)
				continue
			case kind == "alert":
				continue
			}
			if _, ok := buttonLabel(strings.TrimSpace(c)); ok {
				continue
			}
			for _, seg := range strings.Split(c, ";") {
				if seg = strings.TrimSpace(seg); seg == "" || buttonSegment(seg) {
					continue
				}
				fs, idx := r.segmentFields(seg, inputs, used, s)
				for i, f := range fs {
					if idx[i] >= 0 {
						used[idx[i]], at[idx[i]], placed[idx[i]] = true, s.ID, f.label
					}
					p.bySec[s.ID] = append(p.bySec[s.ID], f)
				}
				// A line that says how the form behaves describes it; one
				// about the fields' make-up ("Field + helper") says nothing.
				if len(fs) == 0 && !(explicitHead.MatchString(seg) && behaviour(seg) && !has(strings.ToLower(seg), "terisi", "nilai")) {
					p.desc[s.ID] = append(p.desc[s.ID], capital(seg))
				}
			}
		}
	}
	// An input no line names sits after the input before it.
	labels := placed
	for i, in := range inputs {
		if used[i] {
			continue
		}
		f := r.fieldOf(fieldLabel(in), &inputs[i], core.Section{})
		labels[i] = f.label
		prev, after := "", ""
		for j := i - 1; j >= 0 && prev == ""; j-- {
			prev, after = at[j], labels[j]
		}
		before := ""
		if prev == "" {
			// No input before it is placed: it goes before the next one.
			for j := i + 1; j < len(inputs) && prev == ""; j++ {
				prev, before = at[j], labels[j]
			}
		}
		if prev == "" && len(secs) > 0 {
			prev = secs[0].ID
		}
		if prev == "" {
			p.unplaced = append(p.unplaced, f)
			continue
		}
		at[i] = prev
		list := p.bySec[prev]
		pos := len(list)
		if k := slices.IndexFunc(list, func(x formField) bool { return x.label == after }); k >= 0 && after != "" {
			pos = k + 1
		}
		if k := slices.IndexFunc(list, func(x formField) bool { return x.label == before }); k >= 0 && before != "" {
			pos = k
		}
		p.bySec[prev] = slices.Insert(list, pos, f)
	}
	return p
}

// segmentFields are the inputs one part of a content line names, with the
// index of the page field each maps to (-1 for an input only the line
// names). None when the part only describes the section.
func (r *render) segmentFields(seg string, inputs []core.FieldRule, used []bool, s core.Section) ([]formField, []int) {
	head, list := seg, ""
	if h, l, ok := strings.Cut(seg, ":"); ok && strings.TrimSpace(l) != "" {
		head, list = strings.TrimSpace(h), strings.TrimSpace(l)
	}
	// The control a line starts with is the control of every input it
	// lists ("Pilih gudang dan barang").
	ctl := ""
	if explicitHead.MatchString(head) {
		ctl, _ = r.ctlOf(head)
	}
	var fs []formField
	var idx []int
	parts := []string{head}
	if list == "" {
		// A note in parentheses belongs to its part ("Alasan (teks
		// multi-baris, wajib)").
		parts = partSplit.Split(fontNote.ReplaceAllString(head, ""), -1)
	} else if generic(stripWord(head, controlWords)) || generic(head) {
		// "Segmented: Izin, Sakit" is the choice the page's data offers.
		for i, in := range inputs {
			if df, ok := r.doc.Project.DataField(in.Field); ok && !used[i] && df.Kind == "enum" {
				used[i] = true
				f := r.fieldOf(head, &inputs[i], s)
				f.options = choiceList(list)
				f.kind = map[bool]string{true: "segmented", false: choiceKind(len(f.options))}[f.kind == "segmented"]
				return []formField{f}, []int{i}
			}
		}
	}
	for _, part := range parts {
		label, _, _ := fieldSpec(stripWord(stripWord(part, controlWords), pickHeads))
		i := matchInput(label, inputs, used)
		if i < 0 {
			continue
		}
		used[i] = true // held while the line is read; plan keeps it
		// The line's words for this input: its own part, with the control
		// the line names at its start.
		words := part
		if len(parts) == 1 {
			words = seg
		} else if ctl != "" && !strings.Contains(strings.ToLower(part), ctl) {
			words = ctl + " " + part
		}
		f := r.fieldOf(words, &inputs[i], s)
		if list != "" {
			f.options = choiceList(list)
		}
		fs, idx = append(fs, f), append(idx, i)
	}
	if len(fs) > 0 {
		return fs, idx
	}
	// No page field: an input the line names on its own ("Field kontak
	// pelapor", "Kota"), or several short ones ("Pilih gudang dan
	// barang"); a longer line describes the section.
	if list == "" && len(parts) > 1 && explicitHead.MatchString(head) && !slices.ContainsFunc(parts, func(p string) bool { return len(strings.Fields(p)) > 3 }) {
		for i, part := range parts {
			if i > 0 && ctl != "" {
				part = ctl + " " + part
			}
			fs, idx = append(fs, r.fieldOf(part, nil, s)), append(idx, -1)
		}
		return fs, idx
	}
	if behaviour(head) {
		return nil, nil
	}
	if explicitHead.MatchString(head) || len(strings.Fields(head)) <= 3 && list == "" && !strings.ContainsAny(head, `'"‘“`) && !core.StartsCreate(head) && !editAction(head) {
		src := seg
		if list != "" {
			src = head
		}
		f := r.fieldOf(src, nil, s)
		if list != "" {
			f.options = choiceList(list)
			if f.kind != "segmented" && f.kind != "check" {
				f.kind = choiceKind(len(f.options))
			}
		}
		if generic(f.label) {
			f.label = capital(s.Name)
		}
		return []formField{f}, []int{-1}
	}
	return nil, nil
}

// buttonSegment reports whether part of a line names a button: "tombol
// Simpan perubahan", or a lone "Simpan" or "Batal".
func buttonSegment(seg string) bool {
	if _, ok := buttonLabel(seg); ok {
		return true
	}
	f := strings.Fields(strings.ToLower(seg))
	return len(f) > 0 && len(f) <= 2 && slices.Contains([]string{"simpan", "batal", "kirim", "kembali", "tutup", "lanjut", "selesai", "save", "cancel", "submit"}, f[0])
}

// behaviour reports whether a line says how the form behaves rather than
// naming an input ("Field terisi nilai kini", "Field + helper").
func behaviour(head string) bool {
	rest := strings.TrimSpace(stripWord(head, controlWords))
	if rest == "" || !unicode.IsLetter([]rune(rest)[0]) {
		return true
	}
	return has(strings.ToLower(rest), "terisi", "diisi otomatis", "nilai kini", "nilai saat ini", "per langkah")
}

// generic reports whether a label names only a control ("Segmen").
func generic(label string) bool {
	return slices.Contains([]string{"segmen", "segmented", "pilihan", "pilihan segmen", "opsi", "radio", "toggle", "field", "input"}, strings.ToLower(strings.TrimSpace(label)))
}

// matchInput is the unplaced page field a part of a line names: one whose
// label holds every word of the part or whose words the part holds, else
// the one sharing the most words with it. -1 when none or a tie.
func matchInput(part string, inputs []core.FieldRule, used []bool) int {
	if strings.TrimSpace(part) == "" {
		return -1
	}
	for i, in := range inputs {
		if used[i] {
			continue
		}
		label := fieldLabel(in)
		name := in.Field
		if _, n, ok := strings.Cut(name, "."); ok {
			name = n
		}
		if core.StemsCovered(label, part) || core.StemsCovered(part, label) || strings.EqualFold(strings.TrimSpace(part), strings.ReplaceAll(name, "_", " ")) {
			return i
		}
	}
	best, score, tied := -1, 0, false
	for i, in := range inputs {
		if used[i] {
			continue
		}
		switch n := core.StemOverlap(part, fieldLabel(in)); {
		case n > score:
			best, score, tied = i, n, false
		case n == score && n > 0:
			tied = true
		}
	}
	if tied {
		return -1
	}
	return best
}

// choiceList are the values a line lists ("Bahan bakar, Parkir, atau
// Lainnya").
func choiceList(s string) []string {
	var out []string
	for _, p := range partSplit.Split(strings.TrimSuffix(strings.TrimSpace(s), "."), -1) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, capital(p))
		}
	}
	return out
}

// choiceKind is the control for a choice of n values: a few short ones
// side by side, more in a list.
func choiceKind(n int) string {
	if n > 0 && n <= 3 {
		return "segmented"
	}
	return "select"
}

// ctlOf is the control word a line starts with or names, and the kind it
// asks for ("" when the line names none).
func (r *render) ctlOf(line string) (word, kind string) {
	l := strings.ToLower(line)
	first := ""
	if f := strings.Fields(l); len(f) > 0 {
		first = f[0]
	}
	switch {
	case has(l, checkWords...):
		return "centang", "check"
	case has(l, switchWords...):
		return "toggle", "bool"
	case has(l, segWords...):
		return "segmen", "segmented"
	case has(l, searchWords...):
		return "pemilih", "combobox"
	case slices.Contains(fileWords, first) || has(l, "lampiran"):
		return "unggah", "file"
	case has(l, "textarea"):
		return "textarea", "longtext"
	case slices.Contains(pickWords, first):
		return "pilih", "select"
	}
	return "", ""
}

// mainEntity is the entity most of the page's inputs belong to: the record
// the form makes. "" when no entity leads.
func (r *render) mainEntity() string {
	n := map[string]int{}
	order := []string{}
	for _, f := range r.fields("form-input") {
		if e, _, ok := strings.Cut(f.Field, "."); ok {
			if n[e] == 0 {
				order = append(order, e)
			}
			n[e]++
		}
	}
	best, top, tied := "", 0, false
	for _, e := range order {
		switch {
		case n[e] > top:
			best, top, tied = e, n[e], false
		case n[e] == top:
			tied = true
		}
	}
	if tied {
		return ""
	}
	return best
}

// refers reports whether entity from points at entity to, as far as the
// blueprint tells: true when it does not describe from.
func (r *render) refers(from, to string) bool {
	p := r.doc.Project
	if p == nil || p.Blueprint == nil {
		return true
	}
	for _, e := range p.Blueprint.Database.Entities {
		if !strings.EqualFold(e.Name, from) {
			continue
		}
		for _, spec := range e.Fields {
			if df := core.ParseDataField(e.Name, spec); df.Kind == "ref" && strings.EqualFold(df.Ref, to) {
				return true
			}
		}
		return slices.ContainsFunc(e.Relates, func(x string) bool { return strings.EqualFold(strings.TrimSpace(x), to) })
	}
	return true
}

// identityField names a record: picking one is a search over records.
func identityField(name string) bool {
	return slices.Contains([]string{"name", "nama", "title", "judul", "sku", "code", "kode", "number", "nomor"}, strings.ToLower(name))
}

// fieldOf is the input a line names, typed by the page field it maps to
// (nil when none) and the blueprint's data behind that field.
func (r *render) fieldOf(line string, rule *core.FieldRule, s core.Section) formField {
	label, hint, optional := fieldSpec(stripWord(stripWord(line, controlWords), pickHeads))
	l := strings.ToLower(line)
	f := formField{label: label, hint: hint, optional: optional, raw: line, kind: "text"}
	_, ctl := r.ctlOf(line)
	if rule != nil {
		if ctl != "check" {
			f.label = capital(fieldLabel(*rule))
		}
		if f.hint == "" && strings.TrimSpace(rule.Note) != "" {
			f.hint = strings.TrimSpace(rule.Note)
		}
		entity, name, _ := strings.Cut(rule.Field, ".")
		if df, ok := r.doc.Project.DataField(rule.Field); ok {
			r.typed(&f, df)
			if df.Kind == "text" {
				// The blueprint says no more than a name: the words decide.
				f.kind = legacyKind(l, f.label)
			}
		} else {
			f.kind = legacyKind(l, f.label)
		}
		if main := r.mainEntity(); r.page.Pattern != "settings" && main != "" && entity != main && identityField(name) && (f.kind == "text" || f.kind == "select") && r.refers(main, entity) {
			f.kind, f.unique = "combobox", false // the record another entity names
		}
	} else {
		f.kind = legacyKind(l, f.label)
		// A field the page shows elsewhere ("Peran" in its table) says
		// what the input holds.
		for _, pf := range r.page.Fields {
			if lb := fieldLabel(pf); strings.EqualFold(lb, f.label) || core.StemsCovered(lb, f.label) && core.StemsCovered(f.label, lb) {
				if df, ok := r.doc.Project.DataField(pf.Field); ok && df.Kind != "ref" {
					r.typed(&f, df)
				}
				break
			}
		}
	}
	switch ctl {
	case "check", "bool", "file", "longtext":
		f.kind = ctl
	case "segmented":
		f.kind = "segmented"
	case "combobox":
		if f.kind != "segmented" {
			f.kind = "combobox"
		}
	case "select":
		if !slices.Contains([]string{"segmented", "combobox", "select"}, f.kind) {
			f.kind = "select"
		}
	}
	// "Jenis masuk/keluar" offers Masuk and Keluar.
	if alts := slashChoices(f.label + " " + line); len(alts) >= 2 && (len(f.options) == 0 || len(f.options) == len(alts)) &&
		slices.Contains([]string{"select", "segmented", "text"}, f.kind) {
		f.options = alts
		if f.kind == "text" {
			f.kind = choiceKind(len(alts))
		}
	}
	if has(l, "bilangan bulat", "integer", "angka bulat") && (f.kind == "number" || f.kind == "money" || f.kind == "text") {
		if f.kind == "text" {
			f.kind = "number"
		}
		f.step = "1"
		if f.min == "" && has(l, "lebih dari 0", "> 0", ">0", "positif") {
			f.min = "1"
		}
	}
	if has(l, lockWords...) {
		f.readonly = true
	}
	if has(l, nowWords...) && f.def == "" {
		f.def = "now"
	}
	if m := defaultWord.FindStringSubmatch(line); m != nil && f.def == "" {
		f.def = capital(m[1])
	}
	if has(l, "chip", "pintasan", "shortcut") && (f.kind == "money" || f.kind == "number") {
		f.chips = true
	}
	if has(l, "wajib", "required") {
		f.optional = false
	}
	if has(l, "boleh kosong", " kosong ", "kosong menghapus") {
		f.optional = true // "Deadline kosong menghapus deadline"
	}
	if has(strings.ToLower(s.Name), "opsional", "optional") {
		f.optional = true
	}
	if f.kind == "check" || f.kind == "bool" {
		f.optional = !has(l, "setuju", "persetujuan", "consent")
	}
	if rule != nil && len(f.options) > 0 {
		// The page may name the values in its own words elsewhere
		// ("Kategori biaya: Bahan bakar, Parkir, Kuota, Lainnya").
		if words := r.namedChoices(f.label, len(f.options)); words != nil {
			f.options = words
		}
	}
	if (f.kind == "select" || f.kind == "segmented" || f.kind == "combobox") && len(f.options) == 0 {
		f.options = sampleOptions(f.label)
	}
	if f.def != "" && f.def != "now" && len(f.options) > 0 && !slices.Contains(f.options, f.def) {
		f.options = append([]string{f.def}, f.options...)
	}
	return f
}

// namedChoices are the n values a line of the page lists for label
// ("Label: a, b, c"); nil when none does.
func (r *render) namedChoices(label string, n int) []string {
	for _, s := range r.page.Sections {
		for _, c := range s.Content {
			head, list, ok := strings.Cut(c, ":")
			if !ok || !core.StemsCovered(label, head) {
				continue
			}
			if out := choiceList(list); len(out) == n {
				return out
			}
		}
	}
	return nil
}

// typed sets the control the blueprint's data field asks for.
func (r *render) typed(f *formField, df core.DataField) {
	f.unique, f.min, f.maxLen = df.Unique, df.Min, df.MaxLen
	switch df.Kind {
	case "enum":
		for _, o := range df.Options {
			f.options = append(f.options, enumLabel(o))
		}
		f.kind = choiceKind(len(f.options))
		if df.Default != "" {
			f.def = enumLabel(df.Default)
		}
	case "ref":
		f.kind, f.unique = "combobox", false
	case "json":
		f.kind = "longtext"
	case "phone", "number", "money", "date", "datetime", "bool", "email", "password", "url", "longtext", "text":
		f.kind = df.Kind
	}
	if df.Kind == "number" && f.step == "" {
		f.step = "1"
	}
	switch {
	case df.Required:
		f.optional = false
	case df.Optional:
		f.optional = true
	case df.Kind == "longtext":
		// A free note says more when there is more to say; the blueprint
		// asks for it only with "wajib".
		f.optional = true
	}
}

// enumLabel is how a stored value reads: in_progress is In progress,
// FUEL is Fuel.
func enumLabel(v string) string {
	return capital(strings.ToLower(strings.ReplaceAll(v, "_", " ")))
}

// legacyKind is the control a label asks for when no data field says.
func legacyKind(l, label string) string {
	switch {
	case has(l, checkWords...):
		return "check"
	case has(l, "textarea", "catatan", "deskripsi", "keterangan", "alamat", "pesan", "komentar", "note", "alasan", "uraian"):
		return "longtext"
	case has(l, "status", "jenis", "kategori", "peran", "tipe", "role", "metode", "pilih", "zona", "cabang", "lokasi", "select", "dropdown", "prioritas", "priority"):
		return "select"
	case has(l, "tanggal", "berlaku", "jadwal", "date", "deadline", "jatuh tempo", "tenggat"):
		return "date"
	case has(l, "sandi", "password"):
		return "password"
	case has(l, "email", "surel"):
		return "email"
	case has(l, "telepon", "phone", "whatsapp", "no hp", "nomor hp", "ponsel", "kontak"):
		return "phone"
	case money(strings.ToLower(label)):
		return "money"
	}
	return "text"
}

// slashChoices are the values a "a/b" word lists ("masuk/keluar").
func slashChoices(s string) []string {
	for _, w := range strings.Fields(s) {
		w = strings.Trim(w, ",.;:()")
		if !strings.Contains(w, "/") || strings.Contains(w, "//") {
			continue
		}
		var out []string
		for _, o := range strings.Split(w, "/") {
			if o = strings.TrimSpace(o); o != "" {
				out = append(out, capital(o))
			}
		}
		if len(out) >= 2 {
			return out
		}
	}
	return nil
}

// sampleOptions are placeholder choices for a label, without repeats.
func sampleOptions(label string) []string {
	var out []string
	for i := 0; i < 6 && len(out) < 4; i++ {
		if v := sample(label, i); !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// fieldValue is what a filled input shows.
func fieldValue(f formField) string {
	switch f.kind {
	case "select", "segmented", "combobox":
		if f.def != "" {
			return f.def
		}
		if len(f.options) > 0 {
			return f.options[0]
		}
	case "number":
		v := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, sample(f.label, 0))
		if v == "" {
			v = "12"
		}
		return v
	case "money":
		return "125000"
	case "date":
		return "2026-11-12"
	case "datetime":
		return "2026-11-17T08:30"
	case "bool", "check":
		return "on"
	case "file", "password":
		return ""
	case "longtext":
		return "Contoh " + strings.ToLower(f.label) + "."
	}
	return sample(f.label, 0)
}

// startValue is what an input holds before anyone types: its default.
func startValue(f formField) string {
	switch {
	case f.def == "now" && f.kind == "datetime":
		return "2026-11-17T08:30"
	case f.def == "now" && f.kind == "date":
		return "2026-11-17"
	case f.def != "now":
		return f.def
	}
	return ""
}

// wide reports whether an input takes the form's whole row.
func (f formField) wide() bool {
	switch f.kind {
	case "longtext", "file", "check":
		return true
	case "segmented":
		return len(f.options) > 3
	}
	return f.chips
}

// required reports whether the browser asks for the input before sending.
func (f formField) required() bool {
	return !f.optional && !f.readonly && f.kind != "file"
}

// control draws one input of a form. attrs carry its form, validity and
// state; value is what it holds.
func (r *render) control(f formField, id, attrs, value string) {
	val := func() string {
		if value == "" {
			return ""
		}
		return ` value="` + esc(value) + `"`
	}
	ph := esc(placeholder(f.label))
	extra := ""
	if f.readonly {
		extra += ` readonly aria-readonly="true"`
	}
	if f.maxLen > 0 {
		extra += fmt.Sprintf(` maxlength="%d"`, f.maxLen)
	}
	switch f.kind {
	case "longtext":
		r.f(`<textarea id="%s" name="%s" class="fk-input" rows="3" placeholder="Tulis %s"%s%s>%s</textarea>`, id, id, esc(strings.ToLower(f.label)), attrs, extra, esc(value))
	case "select":
		r.f(`<select id="%s" name="%s" class="fk-input fk-choice"%s><option value="">Pilih %s</option>`, id, id, attrs, esc(strings.ToLower(f.label)))
		for _, o := range f.options {
			sel := ""
			if o == value {
				sel = " selected"
			}
			r.f(`<option%s>%s</option>`, sel, esc(o))
		}
		r.w(`</select>`)
	case "combobox":
		r.f(`<div class="fk-combo">%s<input id="%s" name="%s" type="search" class="fk-input" list="%s-list" autocomplete="off" placeholder="Cari %s…"%s%s%s></div><datalist id="%s-list">`,
			icon("search"), id, id, id, esc(strings.ToLower(f.label)), val(), attrs, extra, id)
		for _, o := range f.options {
			r.f(`<option value="%s"></option>`, esc(o))
		}
		r.w(`</datalist>`)
	case "segmented":
		r.w(`<div class="fk-seg">`)
		for i, o := range f.options {
			chk, req := "", ""
			if o == value {
				chk = " checked"
			}
			if i == 0 && strings.Contains(attrs, " required") {
				req = " required"
			}
			form := ""
			if m := formAttrRe.FindString(attrs); m != "" {
				form = m
			}
			r.f(`<label><input type="radio" name="%s" value="%s"%s%s%s%s><span>%s</span></label>`, id, esc(o), chk, req, form, disabledAttr(attrs), esc(o))
		}
		r.w(`</div>`)
	case "file":
		r.f(`<label class="fk-drop" for="%s">%s<span><b>Pilih file</b> atau seret ke sini</span><input id="%s" name="%s" type="file"%s></label>`, id, icon("upload"), id, id, attrs)
	case "money":
		r.f(`<div class="fk-affix"><span aria-hidden="true">Rp</span><input id="%s" name="%s" type="number" inputmode="numeric" class="fk-input" placeholder="0"%s%s%s%s></div>`, id, id, numAttrs(f), val(), attrs, extra)
	case "number":
		r.f(`<input id="%s" name="%s" type="number" inputmode="numeric" class="fk-input" placeholder="0"%s%s%s%s>`, id, id, numAttrs(f), val(), attrs, extra)
	default:
		typ := map[string]string{"date": "date", "datetime": "datetime-local", "email": "email", "phone": "tel", "password": "password", "url": "url"}[f.kind]
		if typ == "" {
			typ = "text"
		}
		auto := ""
		switch f.kind {
		case "email":
			auto = ` autocomplete="email"`
		case "phone":
			auto = ` autocomplete="tel" inputmode="tel"`
		}
		r.f(`<input id="%s" name="%s" type="%s" class="fk-input" placeholder="%s"%s%s%s%s>`, id, id, typ, ph, auto, val(), attrs, extra)
	}
	if f.chips {
		r.f(`<div class="fk-chips" role="group" aria-label="Nominal cepat">`)
		for _, v := range []string{"10.000", "25.000", "50.000", "100.000"} {
			r.f(`<label class="fk-chip"><input type="radio" name="%s-quick"%s><span>Rp %s</span></label>`, id, disabledAttr(attrs), v)
		}
		r.w(`</div>`)
	}
}

var formAttrRe = regexp.MustCompile(` form="[^"]*"`)

func disabledAttr(attrs string) string {
	if strings.Contains(attrs, " disabled") {
		return " disabled"
	}
	return ""
}

// numAttrs are a number input's bounds.
func numAttrs(f formField) string {
	out := ""
	if f.min != "" {
		out += ` min="` + esc(f.min) + `"`
	}
	if f.step != "" {
		out += ` step="` + esc(f.step) + `"`
	}
	return out
}

// helpOf is what is said under an input: the design's hint, then what the
// data asks of it.
func (f formField) helpOf() string {
	var out []string
	if f.hint != "" {
		out = append(out, capital(f.hint))
	}
	switch {
	case f.readonly:
		out = append(out, "Tidak dapat diubah di sini.")
	case f.unique:
		out = append(out, "Harus unik, tidak boleh sama dengan data lain.")
	}
	if f.kind == "combobox" && f.hint == "" {
		out = append(out, "Ketik untuk mencari.")
	}
	return strings.Join(out, " ")
}

// EditTitle is the page's name in the mode the form is opened in: a form
// for "Tambah atau Ubah Barang" is "Ubah barang" holding a record, else
// "Tambah barang".
func (r *render) formTitle() string {
	name := r.page.Name
	if !formLike(r.page.Pattern) || r.page.Pattern == "auth" {
		return name
	}
	f := strings.Fields(name)
	for i := 1; i+1 < len(f); i++ {
		if !strings.EqualFold(f[i], "atau") && !strings.EqualFold(f[i], "or") {
			continue
		}
		object := strings.Join(f[i+2:], " ")
		if object == "" {
			return name
		}
		verb := f[0]
		if r.editing() {
			verb = f[i+1]
		}
		return capital(strings.ToLower(verb)) + " " + strings.ToLower(object)
	}
	return name
}

// editing reports whether the form holds a record to change: opened by an
// edit action (fill).
func (r *render) editing() bool {
	return r.doc.Sim.Fill && r.doc.Sim.View == "" && formLike(r.page.Pattern) && r.page.Pattern != "auth"
}

// actionHref leads an action to page pg: an edit action opens its form
// holding the record.
func (r *render) actionHref(pg, action string) string {
	if to, _ := r.doc.page(pg); to != nil && formLike(to.Pattern) && to.Pattern != "auth" && editAction(actionName(action)) {
		sim := r.doc.Sim.Nav()
		sim.Fill = true
		return r.doc.hrefSim(pg, sim)
	}
	return r.doc.href(pg)
}

// lineFields are the inputs a dialog's lines name, each typed by the page
// field it maps to.
func (r *render) lineFields(lines []string, s core.Section) []formField {
	inputs := r.fields("form-input")
	var out []formField
	for _, it := range lines {
		used := make([]bool, len(inputs))
		label, _, _ := fieldSpec(stripWord(it, controlWords))
		var rule *core.FieldRule
		if i := matchInput(label, inputs, used); i >= 0 && core.StemsCovered(fieldLabel(inputs[i]), label) {
			rule = &inputs[i]
		}
		f := r.fieldOf(it, rule, s)
		if rule != nil {
			f.label = label // the dialog's own words
		}
		out = append(out, f)
	}
	return out
}

// field draws input f of a form with its label, help and error: marked
// invalid when the form came back incomplete (fb invalid) or its action
// failed on it; holding a record when fill.
func (r *render) field(f formField, id, formAttr string, lock bool, fb string, fail failure, fill bool) {
	dis := r.disabled()
	if lock {
		dis = " disabled"
	}
	req, mark := "", ""
	switch {
	case f.required():
		req, mark = " required", `<span class="fk-req" aria-hidden="true">*</span>`
	case f.optional && f.kind != "check" && f.kind != "bool":
		mark = ` <span class="fk-opt">(opsional)</span>`
	}
	msg, bad := f.label+" wajib diisi.", false
	switch {
	case fb == "invalid" && f.required():
		bad = true
	case fb == "error" && f.label == fail.field:
		msg, bad = fail.fieldMsg, true
	}
	cls, aria := "fk-field", ` aria-describedby="`+id+`-m"`
	if bad {
		cls += " fk-invalid"
		aria += ` aria-invalid="true"`
	}
	if f.wide() {
		cls += " fk-field-wide"
	}
	value := startValue(f)
	if fill {
		value = fieldValue(f)
	}
	attrs := formAttr + aria + dis + req
	switch f.kind {
	case "check":
		chk := ""
		if fill || value != "" {
			chk = " checked"
		}
		r.f(`<label class="fk-check"><input type="checkbox" name="%s"%s%s%s%s><span></span>%s</label>`, id, formAttr, dis, req, chk, esc(f.label))
		return
	case "bool":
		chk := ""
		if fill || value != "" && value != "false" {
			chk = " checked"
		}
		r.f(`<div class="%s"><label class="fk-switch"><input type="checkbox" role="switch" id="%s" name="%s"%s%s%s><span class="fk-switch-track" aria-hidden="true"></span><span>%s</span></label>`, cls, id, id, formAttr, dis, chk, esc(f.label))
	case "segmented":
		r.f(`<fieldset class="%s fk-group" aria-describedby="%s-m"><legend>%s%s</legend>`, cls, id, esc(f.label), mark)
		r.control(f, id, attrs, value)
	default:
		r.f(`<div class="%s"><label for="%s">%s%s</label>`, cls, id, esc(f.label), mark)
		r.control(f, id, attrs, value)
	}
	help := f.helpOf()
	count := ""
	if f.maxLen > 0 {
		count = fmt.Sprintf(`<span class="fk-count">%d/%d</span>`, len([]rune(value)), f.maxLen)
	}
	if help != "" || count != "" {
		r.f(`<div class="fk-field-foot"><p class="fk-help">%s</p>%s</div>`, esc(help), count)
	}
	r.f(`<p class="fk-err" id="%s-m">%s%s</p>`, id, icon("alert-circle"), esc(msg))
	if f.kind == "segmented" {
		r.w(`</fieldset>`)
		return
	}
	r.w(`</div>`)
}

// splitWarnings takes the warnings out of a section's lines ("Peringatan
// 'melebihi stok tersedia' bila qty keluar lebih besar").
func splitWarnings(lines []string) (rest, warns []string) {
	for _, c := range lines {
		if kind, _ := formLine(c); kind == "alert" {
			warns = append(warns, c)
			continue
		}
		rest = append(rest, c)
	}
	return rest, warns
}

// leadWords join a part to the list before it ("dan stok minimum").
var leadWords = regexp.MustCompile(`(?i)^(dan|atau|serta|and|or)\s+`)

// contextLabels name the values beside a form by the page's own field
// labels: "dan stok minimum barang terpilih" is Stok minimum.
func (r *render) contextLabels(els []string) []string {
	shown := r.fields("detail", "summary", "badge", "card-value")
	used := make([]bool, len(shown))
	out := make([]string, 0, len(els))
	for _, it := range els {
		it = capital(strings.TrimSpace(leadWords.ReplaceAllString(strings.TrimSpace(it), "")))
		if i := matchInput(it, shown, used); i >= 0 && core.StemsCovered(fieldLabel(shown[i]), it) {
			used[i] = true
			it = capital(fieldLabel(shown[i]))
		}
		out = append(out, it)
	}
	return out
}

// callout is a warning the section shows: the words it quotes, and when.
func (r *render) callout(line string) {
	msg, when := line, ""
	if m := quotedText.FindStringSubmatchIndex(line); m != nil {
		msg = line[m[2]:m[3]]
		when = strings.TrimSpace(line[m[1]:])
	}
	r.f(`<div class="fk-callout" role="note">%s<div><b>%s</b>`, icon("alert"), esc(capital(strings.TrimSpace(msg))))
	if when != "" {
		r.f(`<span>Tampil %s.</span>`, esc(strings.TrimSuffix(strings.ToLower(when), ".")))
	}
	r.w(`</div></div>`)
}

// A wizard whose design names its inputs only as page fields spreads them
// over its steps: an input goes to the first step on whose name it shares
// a word, else stays with the input before it. The active step is the one
// the URL names (view step-N); its Lanjut checks the step's inputs and
// opens the next one, the last step sends the form.

func stepView(i int) string { return fmt.Sprintf("step-%d", i+1) }

// wizardStep is the active step of n, from the URL.
func (r *render) wizardStep(n int) int {
	var i int
	if _, err := fmt.Sscanf(r.doc.Sim.View, "step-%d", &i); err == nil && i >= 1 && i <= n {
		return i - 1
	}
	return 0
}

// stepFields are the inputs of each step.
func stepFields(steps []string, fs []formField) [][]formField {
	out := make([][]formField, len(steps))
	at := 0
	for _, f := range fs {
		for j := at; j < len(steps); j++ {
			if core.SharesStem(f.label, steps[j]) {
				at = j
				break
			}
		}
		out[at] = append(out[at], f)
	}
	return out
}

// wizardBody draws step cur of steps: its inputs, or on a last step
// without any what will be sent, and the way back and on.
func (r *render) wizardBody(steps []string, cur int) {
	per := stepFields(steps, r.plan().unplaced)
	last := cur == len(steps)-1
	action := r.doc.Self
	if action == "" {
		action = "#"
	}
	submit := r.page.PrimaryAction
	if submit == "" {
		submit = "Simpan"
	}
	r.f(`<div class="fk-step-card"><header><small>Langkah %d dari %d</small><h3>%s</h3></header>`, cur+1, len(steps), esc(steps[cur]))
	r.f(`<form class="fk-form" method="get" action="%s" data-foundry-sim="form">`, esc(action))
	var next Sim
	if last {
		next = r.outcome(submit, "")
	} else {
		next = r.doc.Sim.Nav()
		next.View = stepView(cur + 1)
	}
	if r.doc.live() {
		r.hidden(next)
	}
	if last && r.doc.Sim.View == stepView(cur) && r.doc.Sim.Feedback == "error" {
		r.alert("error", "Data belum tersimpan", "Periksa lagi isian lalu kirim ulang.", "", "")
	}
	fill := r.doc.Sim.Fill
	switch {
	case len(per[cur]) > 0:
		for i, f := range per[cur] {
			r.field(f, fmt.Sprintf("f-step%d-%d", cur+1, i), "", r.formLocked(), "", failure{}, fill)
		}
	case last:
		// The last step shows what the steps before it collected.
		r.w(`<p class="fk-sec-desc fk-field-wide">Periksa lagi data dari langkah sebelumnya sebelum menyimpan.</p><dl class="fk-dl fk-field-wide fk-review">`)
		for i, fs := range per {
			for _, f := range fs {
				v := fieldValue(f)
				if f.kind == "password" {
					v = "••••••••"
				}
				r.f(`<div><dt>%s · %s</dt><dd>%s</dd></div>`, esc(steps[i]), esc(f.label), esc(v))
			}
		}
		r.w(`</dl>`)
	default:
		r.w(`<p class="fk-help fk-field-wide">Isian langkah ini belum dirinci di desain.</p>`)
	}
	r.w(`<div class="fk-actions fk-form-bar">`)
	r.requiredNoteOf(per[cur])
	if cur > 0 {
		sim := r.doc.Sim.Nav()
		sim.View = stepView(cur - 1)
		r.f(`<a class="fk-btn" href="%s">%sKembali</a>`, esc(r.doc.hrefSim(r.page.ID, sim)), icon("left"))
	} else {
		r.back("Batal", "fk-btn")
	}
	label := "Lanjut"
	if last {
		label = submit
	}
	r.f(`<button class="fk-btn fk-btn-primary" type="submit"%s>%s%s</button>`, r.disabled(), esc(label), map[bool]string{true: "", false: icon("right")}[last])
	r.w(`</div></form></div>`)
}

// requiredNoteOf says what * means among fs, when it marks any.
func (r *render) requiredNoteOf(fs []formField) {
	for _, f := range fs {
		if f.required() && f.kind != "check" && f.kind != "bool" {
			r.w(`<span class="fk-form-note"><span class="fk-req" aria-hidden="true">*</span> Wajib diisi</span>`)
			return
		}
	}
}

// displayComponent shows values: a summary of what a form sends.
func displayComponent(c string) bool {
	return has(strings.ToLower(c), "statcard", "stat", "summary", "ringkasan", "total", "kpi")
}

// formSummary lists the values a form page's aside sums up ("Subtotal,
// pajak, total"), named by the page's own labels; the last total stands
// out, a warning is drawn as one.
func (r *render) formSummary(s core.Section) {
	rest, warns := splitWarnings(s.Content)
	var els, more []string
	for _, c := range rest {
		for _, p := range splitList(c) {
			p = strings.TrimSpace(leadWords.ReplaceAllString(p, ""))
			if kind, _ := formLine(p); kind == "alert" {
				more = append(more, p)
				continue
			}
			els = append(els, p)
		}
	}
	els = r.contextLabels(els)
	r.w(`<dl class="fk-sum">`)
	for i, it := range els {
		cls := ""
		if i == len(els)-1 && has(strings.ToLower(it), "total") && !has(strings.ToLower(it), "subtotal") {
			cls = ` class="fk-sum-total"`
		}
		r.f(`<div%s><dt>%s</dt><dd>%s</dd></div>`, cls, esc(capital(it)), esc(sample(it, i)))
	}
	r.w(`</dl>`)
	for _, w := range append(warns, more...) {
		r.callout(w)
	}
}
