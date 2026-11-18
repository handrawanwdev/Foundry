package uikit

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A mockup is read by people who do not know the data model: its rows
// must look like the product's real data ("EURUSD", "H1", "Aktif"), not
// placeholders ("Instrumen 1"). A value comes from the design's examples
// of the field, then from what the blueprint says of the field (its
// choices, a yes/no, a figure, a moment, a lifecycle status), and only
// then from a guess on the field's name and label.

// ruleFor is the page field a label shows: the one it names exactly, else
// one whose words it holds or that holds its words ("Interval" shows
// "Interval (detik)"). nil when none.
func (r *render) ruleFor(label string) *core.FieldRule {
	label = strings.TrimSpace(label)
	if label == "" {
		return nil
	}
	for i, f := range r.page.Fields {
		if strings.EqualFold(fieldLabel(f), label) || strings.EqualFold(f.Field, label) {
			return &r.page.Fields[i]
		}
	}
	for i, f := range r.page.Fields {
		if f.Presentation == "row-action" || f.Presentation == "hidden" {
			continue
		}
		if lb := fieldLabel(f); core.StemsCovered(lb, label) || core.StemsCovered(label, lb) {
			return &r.page.Fields[i]
		}
	}
	// A column another page maps under the same label shows the same data
	// ("Instrumen" is Instrument.symbol on every page).
	if r.doc.Layout != nil {
		for pi := range r.doc.Layout.Pages {
			for i, f := range r.doc.Layout.Pages[pi].Fields {
				if f.Presentation != "row-action" && f.Presentation != "hidden" && strings.EqualFold(fieldLabel(f), label) {
					return &r.doc.Layout.Pages[pi].Fields[i]
				}
			}
		}
	}
	return nil
}

// recordName is the field that names a record of an entity to its
// users: its symbol, code, name, title or email. "" when none.
func recordName(p *core.Project, entity string) string {
	if p == nil || p.Blueprint == nil {
		return ""
	}
	for _, e := range p.Blueprint.Database.Entities {
		if !strings.EqualFold(e.Name, entity) {
			continue
		}
		for _, want := range []string{"symbol", "code", "kode", "name", "nama", "title", "judul", "email", "number", "nomor"} {
			for _, spec := range e.Fields {
				if df := core.ParseDataField(e.Name, spec); strings.EqualFold(df.Name, want) {
					return e.Name + "." + df.Name
				}
			}
		}
	}
	return ""
}

// valueOf is a realistic value of a page field in a row and the tone of
// its badge ("" for plain text); ok is false when neither the design nor
// the blueprint says anything about the field.
func (r *render) valueOf(f *core.FieldRule, label string, row int) (text, tone string, ok bool) {
	if f == nil {
		return "", "", false
	}
	badge := f.Presentation == "badge" || status(strings.ToLower(label))
	if ex := nonBlank(f.Examples); len(ex) > 0 {
		v := ex[row%len(ex)]
		if badge {
			return v, toneOf(v), true
		}
		return v, "", true
	}
	var p *core.Project
	if r.doc.Project != nil {
		p = r.doc.Project
	}
	entity, name, _ := strings.Cut(f.Field, ".")
	if p != nil && strings.EqualFold(name, "status") {
		if lc := p.LifecycleOf(entity); lc != nil && len(lc.States) > 0 {
			st := lc.States[row%len(lc.States)]
			v := lc.StateLabel(st.ID)
			return v, toneOf(v), true
		}
	}
	df, found := p.DataField(f.Field)
	if !found {
		return "", "", false
	}
	switch df.Kind {
	case "bool":
		on := row%4 != 3
		return map[bool]string{true: "Aktif", false: "Nonaktif"}[on], map[bool]string{true: "success", false: "warning"}[on], true
	case "enum":
		o := enumLabel(df.Options[row%len(df.Options)])
		if words := r.choiceWords(f.Field, len(df.Options)); words != nil {
			o = words[row%len(words)]
		}
		if raw := df.Options[row%len(df.Options)]; codeLike(raw) {
			o = raw // a code (H1, M5, MN) reads as written
		}
		if badge {
			return o, toneOf(o), true
		}
		return o, "info", true
	case "money":
		return sample("nominal", row), "", true
	case "number":
		return digits(sample(label+" "+humanName(df.Name), row), metricValue(label, row)), "", true
	case "date":
		return sample("tanggal", row), "", true
	case "datetime":
		return sample("tanggal", row) + ", " + sample("masuk", row), "", true
	case "email":
		return sample("email", row), "", true
	case "phone":
		return sample("telepon", row), "", true
	case "ref":
		// A reference reads as the record it points at: instrument_id is
		// the instrument's symbol.
		if id := recordName(p, df.Ref); id != "" && !strings.EqualFold(id, f.Field) {
			ref := core.FieldRule{Field: id, Presentation: f.Presentation}
			if v, t, ok := r.valueOf(&ref, label, row); ok {
				return v, t, true
			}
		}
		fallthrough
	case "text":
		// The field's own name says more than a label like "Instrumen"
		// (Instrument.symbol); a name like "name" is no person's.
		if n := humanName(df.Name); placeholderOf(sample(label, row), label, row) && !person(n) {
			if v := sample(n, row); !placeholderOf(v, n, row) {
				return v, "", true
			}
		}
	}
	return "", "", false
}

// cellValue is a value of a column or attribute by its label.
func (r *render) cellValue(label string, row int) (text, tone string) {
	if v, t, ok := r.valueOf(r.ruleFor(label), label, row); ok {
		return v, t
	}
	if status(strings.ToLower(label)) || r.presentationOf(label) == "badge" {
		return statusFor(label, row)
	}
	return sample(label, row), ""
}

// hintOf is what a column means in plain words, for a reader who does not
// know the data: the design's note, then a few example values.
func (r *render) hintOf(label string) string {
	f := r.ruleFor(label)
	if f == nil || f.Presentation == "row-action" {
		return ""
	}
	var parts []string
	if n := strings.TrimSpace(f.Note); n != "" {
		parts = append(parts, strings.TrimSuffix(capital(n), ".")+".")
	}
	var ex []string
	seen := map[string]bool{}
	for row := 0; row < 6 && len(ex) < 3; row++ {
		if v, _, ok := r.valueOf(f, label, row); ok && !seen[v] {
			seen[v] = true
			ex = append(ex, v)
		}
	}
	if len(ex) > 1 {
		parts = append(parts, "Contoh: "+strings.Join(ex, "; ")+".")
	}
	return strings.Join(parts, " ")
}

// toneOf is the badge tone a value reads as.
func toneOf(v string) string {
	l := strings.ToLower(v)
	switch {
	case has(l, "nonaktif", "inactive", "gagal", "failed", "ditolak", "rejected", "invalid", "batal", "cancel", "error", "dicabut", "revoked", "kedaluwarsa", "expired", "habis", "offline", "blocked", "terhalang", "kritis"):
		return "error"
	case has(l, "menunggu", "pending", "waiting", "dijeda", "paused", "belum", "tertunda", "draft", "antre", "queued", "peringatan", "warning"):
		return "warning"
	case has(l, "aktif", "active", "valid", "lolos", "selesai", "done", "lunas", "berjalan", "running", "sukses", "success", "online", "lengkap", "disetujui", "approved", "tersedia"):
		return "success"
	}
	return "info"
}

// humanName is a field name as words: next_run_at is "next run at".
func humanName(name string) string {
	var b strings.Builder
	for i, r := range name {
		switch {
		case r == '_' || r == '-':
			b.WriteRune(' ')
		case r >= 'A' && r <= 'Z' && i > 0:
			b.WriteRune(' ')
			b.WriteRune(r + 'a' - 'A')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// placeholderOf reports whether sample fell back to "Label n".
func placeholderOf(v, label string, row int) bool {
	return v == fmt.Sprintf("%s %d", capital(label), row+1)
}

// codeLike reports a value written as a code: upper case or with digits.
func codeLike(v string) bool {
	return v != "" && (strings.ToUpper(v) == v || strings.ContainsAny(v, "0123456789"))
}

func nonBlank(xs []string) []string {
	var out []string
	for _, x := range xs {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// textOf is the text of a value, without its tone.
func textOf(a, _ string) string { return a }

// digitsOnly reports a figure: digits with separators.
func digitsOnly(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range v {
		if (r < '0' || r > '9') && r != '.' && r != ',' {
			return false
		}
	}
	return true
}
