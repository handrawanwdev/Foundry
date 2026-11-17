package core

import (
	"slices"
	"strings"
)

// A design names its menu and some links between pages; the pages it
// leaves out of both still belong somewhere. A form or detail page of a
// feature is entered from the list page of that feature in the menu: the
// list's create action opens the form that makes a record, and its rows
// open the record's detail page. These entries follow from the design
// (the pages' features and patterns), not from guessing at words, so the
// mockups draw them and the quality gate counts them.

// ListPatterns list the records of a feature.
var ListPatterns = []string{"table", "list", "cards", "master-detail", "kanban", "calendar"}

func formPattern(p string) bool { return p == "form" || p == "wizard" }

// inMenu reports whether a menu item leads to page id.
func inMenu(l *LayoutSpec, id string) bool {
	for _, g := range l.Navigation {
		for _, it := range g.Items {
			if strings.TrimSpace(it.Page) == id {
				return true
			}
		}
	}
	return false
}

// FeatureHome is the page of feature f people open from the menu: its first
// list page there, else its first page there. "" when the menu has none.
func FeatureHome(l *LayoutSpec, f string) string {
	if l == nil || f == "" {
		return ""
	}
	first := ""
	for _, g := range l.Navigation {
		for _, it := range g.Items {
			pg := layoutPage(l, strings.TrimSpace(it.Page))
			if pg == nil || pg.FeatureID != f || pg.Pattern == "auth" {
				continue
			}
			if slices.Contains(ListPatterns, pg.Pattern) {
				return pg.ID
			}
			if first == "" {
				first = pg.ID
			}
		}
	}
	return first
}

// ParentPage is the page a page outside the menu is entered from: the page
// that links to it, else the home of its feature. "" for a menu page.
func ParentPage(l *LayoutSpec, id string) string {
	if l == nil || inMenu(l, id) {
		return ""
	}
	for _, pg := range l.Pages {
		if pg.ID != id && slices.ContainsFunc(pg.Links, func(ln PageLink) bool { return ln.To == id }) {
			return pg.ID
		}
	}
	pg := layoutPage(l, id)
	if pg == nil || pg.Pattern == "auth" {
		return ""
	}
	if home := FeatureHome(l, pg.FeatureID); home != id {
		return home
	}
	return ""
}

// createWords start an action that makes a new record.
var createWords = []string{"tambah", "tambahkan", "menambah", "menambahkan", "buat", "membuat", "daftarkan", "mendaftarkan",
	"catat", "mencatat", "input", "undang", "mengundang", "unggah", "mengunggah", "impor", "mengimpor", "rekam", "merekam",
	"ajukan", "mengajukan", "add", "create", "new", "register", "record", "invite", "upload", "import"}

// StartsCreate reports whether label starts with an action that makes a
// new record ("Tambah pengguna").
func StartsCreate(label string) bool {
	f := strings.Fields(strings.ToLower(label))
	return len(f) > 0 && slices.Contains(createWords, f[0])
}

func words(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return r == ' ' || r == '/' || r == ',' || r == ':' || r == '(' || r == ')' || r == '.' || r == '\t'
	})
}

// CreateLabel is the create action a form page's name starts with
// ("Tambah atau Ubah Barang" is "Tambah barang"); "" when it starts with
// none.
func CreateLabel(name string) string {
	f := strings.Fields(strings.TrimSpace(name))
	if len(f) < 2 || !slices.Contains(createWords, strings.ToLower(f[0])) {
		return ""
	}
	rest := f[1:]
	// "atau Ubah" offers another way to use the same form.
	for len(rest) > 2 && (strings.EqualFold(rest[0], "atau") || strings.EqualFold(rest[0], "or")) {
		rest = rest[2:]
	}
	return capitalize(strings.ToLower(f[0]) + " " + strings.ToLower(strings.Join(rest, " ")))
}

func capitalize(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return strings.ToUpper(string(r[:1])) + string(r[1:])
}

// CreateForm is the form a list page creates records with: the one form
// page of its feature outside the menu whose name starts with a create
// action, with that action as label.
func CreateForm(l *LayoutSpec, listID string) (formID, label string) {
	list := layoutPage(l, listID)
	if list == nil || list.FeatureID == "" || !slices.Contains(ListPatterns, list.Pattern) || FeatureHome(l, list.FeatureID) != listID {
		return "", ""
	}
	for _, pg := range l.Pages {
		if pg.FeatureID != list.FeatureID || !formPattern(pg.Pattern) || inMenu(l, pg.ID) {
			continue
		}
		if lb := CreateLabel(pg.Name); lb != "" {
			if formID != "" {
				return "", "" // more than one: the design must say which
			}
			formID, label = pg.ID, lb
		}
	}
	return formID, label
}

// DetailPage is the page a list page's rows open: the one detail page of
// its feature outside the menu.
func DetailPage(l *LayoutSpec, listID string) string {
	list := layoutPage(l, listID)
	if list == nil || list.FeatureID == "" || !slices.Contains(ListPatterns, list.Pattern) || FeatureHome(l, list.FeatureID) != listID {
		return ""
	}
	found := ""
	for _, pg := range l.Pages {
		if pg.FeatureID != list.FeatureID || pg.Pattern != "detail" || inMenu(l, pg.ID) {
			continue
		}
		if found != "" {
			return ""
		}
		found = pg.ID
	}
	return found
}

// CreateIntent reports whether a use case of the page's feature makes the
// records the page lists: its name or a user step starts one ("Admin
// membuat akun pengguna", "Buat/ubah kategori") and names the page's
// subject.
func (p *Project) CreateIntent(pg *LayoutPage) bool {
	if p == nil || pg == nil || pg.FeatureID == "" {
		return false
	}
	makes := func(text string) bool {
		return slices.ContainsFunc(words(text), func(w string) bool { return slices.Contains(createWords, w) }) && SharesStem(text, pg.Name)
	}
	for _, fl := range p.Flows {
		if fl.FeatureID != pg.FeatureID {
			continue
		}
		if makes(fl.Name) || slices.ContainsFunc(fl.Steps, func(s string) bool { return !SystemStep(s) && makes(s) }) {
			return true
		}
	}
	return false
}

// entryEdges are the ways into pages the design leaves out of its links
// that the mockups draw: a list's create action to its form, and its rows
// to the detail page.
func entryEdges(l *LayoutSpec) map[string][]string {
	out := map[string][]string{}
	for _, pg := range l.Pages {
		if id, _ := CreateForm(l, pg.ID); id != "" {
			out[pg.ID] = append(out[pg.ID], id)
		}
		if id := DetailPage(l, pg.ID); id != "" {
			out[pg.ID] = append(out[pg.ID], id)
		}
	}
	return out
}

// LinkTarget is the page a link a section names leads to ("Tautan ke
// riwayat mutasi" leads to Riwayat Mutasi): the page whose name shares
// the most words with it, a menu page winning a tie. "" when none shares
// one, or the best are tied.
func LinkTarget(l *LayoutSpec, text, from string) string {
	if l == nil {
		return ""
	}
	want := stems(text)
	best, score, tied := "", 0, false
	for _, pg := range l.Pages {
		if pg.ID == from || pg.Pattern == "auth" {
			continue
		}
		s := overlap(stems(pg.Name), want) * 2
		if s == 0 {
			continue
		}
		if inMenu(l, pg.ID) {
			s++
		}
		switch {
		case s > score:
			best, score, tied = pg.ID, s, false
		case s == score:
			tied = true
		}
	}
	if tied {
		return ""
	}
	return best
}
