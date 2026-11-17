package core

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// The blueprint writes an entity's fields in short free text: "sku
// unique", "type (in|out)", "amountIdr (integer, >0)", "deadline (date,
// nullable)", "category_id FK". A form takes its input controls from
// them: the kind of value, the choices, the bounds, whether it may be
// left empty. Only what the text says is read; a field it says nothing
// about is plain text.

// DataField is one entity field as the blueprint describes it.
type DataField struct {
	Entity, Name string
	// Kind is the value: text, longtext, number, money, date, datetime,
	// bool, email, phone, password, url, enum, ref or json.
	Kind    string
	Options []string // the values of an enum
	Ref     string   // the entity a reference points at
	// Min is the least number, "" when unbounded; MinLen and MaxLen bound
	// a text (0 when unbounded).
	Min            string
	MinLen, MaxLen int
	Unique         bool
	Optional       bool // nullable, opsional
	Required       bool // wajib, not null
	Default        string
}

var (
	fieldNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
	enumRe      = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_]*(?:\s*\|\s*[A-Za-z][A-Za-z0-9_]*)+`)
	minRe       = regexp.MustCompile(`(>=|>|≥)\s*(-?\d+(?:[.,]\d+)?)`)
	lenRe       = regexp.MustCompile(`(\d+)\s*-\s*(\d+)\s*(karakter|char|chars|characters|huruf)`)
	maxLenRe    = regexp.MustCompile(`(?:max|maks|maksimal|maximum)\s*(\d+)\s*(karakter|char|chars|characters|huruf)?`)
	defaultRe   = regexp.MustCompile(`default\s*[:=]?\s*([A-Za-z0-9_.-]+)`)
	fkRe        = regexp.MustCompile(`\bfk\b\s*([A-Za-z][A-Za-z0-9_]*)?`)
)

// ParseDataField reads one field of entity as the blueprint writes it.
func ParseDataField(entity, spec string) DataField {
	spec = strings.TrimSpace(spec)
	f := DataField{Entity: entity, Name: fieldNameRe.FindString(spec)}
	rest := strings.TrimSpace(spec[len(f.Name):])
	low := strings.ToLower(rest)
	words := strings.FieldsFunc(low, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' })
	word := func(ws ...string) bool {
		for _, w := range words {
			for _, x := range ws {
				if w == x {
					return true
				}
			}
		}
		return false
	}
	f.Unique = word("unique", "unik")
	f.Optional = word("nullable", "optional", "opsional")
	f.Required = word("wajib", "required") || strings.Contains(low, "not null")
	if m := defaultRe.FindStringSubmatch(rest); m != nil {
		f.Default = m[1]
	}
	if m := lenRe.FindStringSubmatch(low); m != nil {
		f.MinLen, _ = strconv.Atoi(m[1])
		f.MaxLen, _ = strconv.Atoi(m[2])
	} else if m := maxLenRe.FindStringSubmatch(low); m != nil && m[2] != "" {
		f.MaxLen, _ = strconv.Atoi(m[1])
	}
	if m := enumRe.FindString(rest); m != "" {
		for _, o := range strings.Split(m, "|") {
			f.Options = append(f.Options, strings.TrimSpace(o))
		}
	}
	name := strings.ToLower(f.Name)
	parts := nameParts(f.Name)
	hasName := func(ps ...string) bool {
		for _, p := range ps {
			for _, w := range parts {
				if w == p {
					return true
				}
			}
		}
		return false
	}
	numeric := word("int", "integer", "bigint", "smallint", "number", "numeric", "decimal", "float", "double", "angka", "bilangan")
	if m := minRe.FindStringSubmatch(rest); m != nil {
		numeric = true
		f.Min = strings.ReplaceAll(m[2], ",", ".")
		if m[1] == ">" {
			if n, err := strconv.Atoi(f.Min); err == nil {
				f.Min = strconv.Itoa(n + 1)
			}
		}
	}
	camel := func(suffix string) bool { // completedAt, trackingAllowed
		return strings.HasSuffix(f.Name, suffix) && len(f.Name) > len(suffix) && unicode.IsLower(rune(f.Name[len(f.Name)-len(suffix)-1]))
	}
	switch {
	case len(f.Options) >= 2:
		f.Kind = "enum"
	case fkRe.MatchString(low) || strings.HasSuffix(name, "_id") || camel("Id"):
		f.Kind = "ref"
		if m := fkRe.FindStringSubmatch(low); m != nil && m[1] != "" && m[1] != "self" && m[1] != "nullable" {
			f.Ref = fkTarget(rest, m[1])
		}
		if f.Ref == "" && f.Name != "" {
			f.Ref = refName(f.Name)
		}
		if strings.Contains(low, "fk self") {
			f.Ref = entity
		}
	case word("json", "jsonb"):
		f.Kind = "json"
	case word("bool", "boolean") || strings.HasPrefix(name, "is_") || strings.HasPrefix(name, "has_") || strings.HasPrefix(name, "can_") ||
		camel("Allowed") || camel("Enabled") || camel("Active"):
		f.Kind = "bool"
	case word("timestamp", "datetime", "timestamptz") || strings.HasSuffix(name, "_at") || camel("At"):
		f.Kind = "datetime"
	case word("date", "tanggal") || strings.HasSuffix(name, "_date") || name == "date" || name == "tanggal":
		f.Kind = "date"
	case word("rupiah", "currency", "money", "idr") || numeric && hasName("amount", "price", "harga", "cost", "biaya", "nominal", "idr", "fee", "tarif", "total", "salary", "gaji"):
		f.Kind = "money"
	case numeric || hasName("qty", "quantity", "count", "jumlah"):
		f.Kind = "number"
	case hasName("email", "surel"):
		f.Kind = "email"
	case hasName("password", "sandi"):
		f.Kind = "password"
	case hasName("phone", "telepon", "whatsapp", "mobile", "ponsel"):
		f.Kind = "phone"
	case hasName("url", "website", "link"):
		f.Kind = "url"
	case word("text") && f.MaxLen == 0 || hasName("note", "description", "deskripsi", "catatan", "uraian", "keterangan", "alasan", "reason", "comment", "komentar", "address", "alamat", "bio", "message", "pesan", "body", "content"):
		f.Kind = "longtext"
	default:
		f.Kind = "text"
	}
	return f
}

// nameParts are the lower-case words of a field name: counted_amount and
// countedAmount are counted, amount.
func nameParts(name string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, strings.ToLower(b.String()))
			b.Reset()
		}
	}
	rs := []rune(name)
	for i, r := range rs {
		switch {
		case r == '_' || r == '-':
			flush()
			continue
		case unicode.IsUpper(r) && i > 0 && unicode.IsLower(rs[i-1]):
			flush()
		}
		b.WriteRune(r)
	}
	flush()
	return out
}

// fkTarget is the entity "FK User" names, as the blueprint writes it.
func fkTarget(rest, lower string) string {
	i := strings.Index(strings.ToLower(rest), lower)
	if i < 0 {
		return lower
	}
	return rest[i : i+len(lower)]
}

// refName is the entity a reference field names: category_id is
// Category, sessionId is Session.
func refName(name string) string {
	base := strings.TrimSuffix(strings.TrimSuffix(name, "_id"), "Id")
	var b strings.Builder
	up := true
	for _, r := range base {
		if r == '_' {
			up = true
			continue
		}
		if up {
			r = unicode.ToUpper(r)
			up = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// DataField is the blueprint's field a page field names ("Item.sku");
// false when the blueprint has no such entity field.
func (p *Project) DataField(path string) (DataField, bool) {
	entity, name, ok := strings.Cut(strings.TrimSpace(path), ".")
	if p == nil || p.Blueprint == nil || !ok {
		return DataField{}, false
	}
	for _, e := range p.Blueprint.Database.Entities {
		if !strings.EqualFold(strings.TrimSpace(e.Name), entity) {
			continue
		}
		for _, spec := range e.Fields {
			if f := ParseDataField(e.Name, spec); strings.EqualFold(f.Name, name) {
				return f, true
			}
		}
	}
	return DataField{}, false
}

// StemOverlap counts the words of a that b holds by their root.
func StemOverlap(a, b string) int { return overlap(stems(a), stems(b)) }
