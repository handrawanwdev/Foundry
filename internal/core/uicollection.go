package core

import (
	"fmt"
	"strings"
	"unicode"
)

// A collection is a section that shows records as a table or a list. Every
// collection must let the user search/filter it and page through it: the
// page's own toolbar and pagination sections serve its first collection;
// any other collection names its controls in its content ("Filter: …",
// "Paginasi: …").

// Data controls a section or content entry provides.
const (
	ControlFilter     = "filter"
	ControlPagination = "pagination"
)

// ContentKind is the block a section's own words name: "Tabel antrean: …"
// is a table, "Grafik …" a chart, "Daftar …" a list. The first entry that
// is not a control or tabs decides, else the section name; a section of
// tabs only is "tabs".
func ContentKind(s Section) string {
	first, tabs, tabTable := "", false, false
	for _, c := range s.Content {
		switch {
		case ControlEntry(c) != "":
		case wordKind(c) == "tabs":
			tabs = true
			if _, _, ok := TabColumns(c); ok {
				tabTable = true
			}
		default:
			if first == "" {
				first = c
			}
		}
	}
	if first == "" && tabTable {
		return "table" // its tabs are views of one table
	}
	if first == "" && tabs {
		return "tabs"
	}
	if k := wordKind(first); k != "" && k != "tabs" {
		return k
	}
	if k := wordKind(s.Name); k != "tabs" {
		return k
	}
	return ""
}

// TabColumns splits a tab entry that lists the tab's columns ("Tab Mutasi:
// Waktu, Barang, Qty") into the tab and its columns; ok is false for an
// entry that lists the tabs themselves ("Tab: Aktif, Selesai").
func TabColumns(c string) (tab, cols string, ok bool) {
	c = strings.TrimSpace(c)
	if wordKind(c) != "tabs" {
		return "", "", false
	}
	if i := strings.IndexAny(c, ": "); i >= 0 {
		c = c[i+1:]
	}
	tab, cols, ok = strings.Cut(c, ":")
	tab, cols = strings.TrimSpace(tab), strings.TrimSpace(cols)
	return tab, cols, ok && tab != "" && strings.Contains(cols, ",")
}

// wordKind is the block the first word of a text names.
func wordKind(text string) string {
	first := strings.ToLower(strings.TrimSpace(text))
	for _, c := range []struct{ kind, words string }{
		{"table", "tabel table"}, {"chart", "grafik chart diagram"}, {"list", "daftar list"},
		{"form", "form formulir input isian"}, {"metrics", "kpi metrik"}, {"cards", "kartu card"}, {"tabs", "tab tabs"},
	} {
		for _, w := range strings.Fields(c.words) {
			if strings.HasPrefix(first, w+" ") || strings.HasPrefix(first, w+":") || first == w {
				return c.kind
			}
		}
	}
	return ""
}

var (
	filterWords = []string{"filter", "cari", "pencarian", "search", "saring", "penyaring"}
	pagerWords  = []string{"paginasi", "pagination", "per halaman", "muat lebih", "load more", "infinite scroll", "gulir tanpa batas"}
)

// ControlEntry is the data control a content entry names: "Cari plat" and
// "Filter: status, tanggal" are filters, "Paginasi: 20 per halaman" is
// pagination.
func ControlEntry(c string) string {
	l := strings.ToLower(strings.TrimSpace(c))
	for _, w := range filterWords {
		if strings.HasPrefix(l, w) {
			return ControlFilter
		}
	}
	for _, w := range pagerWords {
		if strings.HasPrefix(l, w) || strings.Contains(l, w) && len(l) < 60 {
			return ControlPagination
		}
	}
	return ""
}

// slotKind is the kind of a slot of a pattern template ("" when unknown).
func slotKind(pattern, slot string) string {
	if KitSlots == nil || slot == "" {
		return ""
	}
	slots, _ := KitSlots(pattern)
	for _, s := range slots {
		if s.ID == slot {
			return s.Kind
		}
	}
	return ""
}

// IsCollection reports whether the section shows records as a table or a
// list: its pattern, its own words or its slot in the page template say so.
func IsCollection(pagePattern string, s Section) bool {
	if DialogSection(pagePattern, s) {
		return false
	}
	switch s.Pattern {
	case "table", "list", "feed":
		return true
	}
	switch ContentKind(s) {
	case "table", "list":
		return true
	case "":
		k := slotKind(pagePattern, s.Slot)
		return k == "table" || k == "list"
	}
	return false
}

// DialogSection reports whether a section is a dialog its page opens (a
// create or edit form, a confirmation): it is drawn over the page, not in
// a slot of the page's template. The sections of a modal page are the
// dialog's own content.
func DialogSection(pagePattern string, s Section) bool {
	if pagePattern == "modal" {
		return false
	}
	if s.Pattern == "modal" || s.Pattern == "dialog" {
		return true
	}
	for _, w := range strings.FieldsFunc(strings.ToLower(s.ID+" "+s.Name), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if w == "dialog" || w == "modal" || w == "popup" {
			return true
		}
	}
	return false
}

// ControlSection is the data control a whole section is: a toolbar or
// facets section filters, a pagination section pages.
func ControlSection(pagePattern string, s Section) string {
	if DialogSection(pagePattern, s) {
		return ""
	}
	switch slotKind(pagePattern, s.Slot) {
	case "toolbar", "facets":
		return ControlFilter
	case "pagination":
		return ControlPagination
	}
	if s.Pattern == "search" {
		return ControlFilter
	}
	text := strings.ToLower(s.ID + " " + s.Name)
	for _, w := range pagerWords {
		if strings.Contains(text, w) {
			return ControlPagination
		}
	}
	for _, w := range filterWords {
		if strings.Contains(text, w) {
			return ControlFilter
		}
	}
	return ""
}

// Collection is a collection section and where its controls come from:
// "section" (its own content), a page section id, or "" when missing.
type Collection struct {
	Section    string
	Filter     string
	Pagination string
}

// Collections lists the collection sections of a page with their controls.
func Collections(pg *LayoutPage) []Collection {
	var out []Collection
	page := map[string]string{}
	for _, s := range pg.Sections {
		if IsCollection(pg.Pattern, s) {
			continue
		}
		if c := ControlSection(pg.Pattern, s); c != "" && page[c] == "" {
			page[c] = s.ID
		}
	}
	for _, s := range pg.Sections {
		if !IsCollection(pg.Pattern, s) {
			continue
		}
		c := Collection{Section: s.ID}
		for _, e := range s.Content {
			switch ControlEntry(e) {
			case ControlFilter:
				c.Filter = "section"
			case ControlPagination:
				c.Pagination = "section"
			}
		}
		if len(out) == 0 {
			if c.Filter == "" {
				c.Filter = page[ControlFilter]
			}
			if c.Pagination == "" {
				c.Pagination = page[ControlPagination]
			}
		}
		out = append(out, c)
	}
	return out
}

// collectionIssues checks that every table or list of a page can be
// searched/filtered and paged.
func collectionIssues(where string, pg *LayoutPage, format int) []Issue {
	level := IssueWarning
	if format >= UIDesignFormat {
		level = IssueError
	}
	var out []Issue
	for _, c := range Collections(pg) {
		var miss []string
		if c.Filter == "" {
			miss = append(miss, "pencarian/filter")
		}
		if c.Pagination == "" {
			miss = append(miss, "paginasi")
		}
		if len(miss) > 0 {
			out = append(out, Issue{Level: level, Where: where + "/" + c.Section,
				Message: fmt.Sprintf("tabel/daftar wajib punya %s: tambahkan section toolbar/pagination atau item content \"Filter: …\" dan \"Paginasi: …\"", strings.Join(miss, " dan "))})
		}
	}
	return out
}
