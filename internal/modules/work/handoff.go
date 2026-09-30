package work

import (
	"fmt"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// PageHandoff is the build spec of one page, assembled from the design
// without AI so it always matches what the user approved.
type PageHandoff struct {
	PageID         string         `json:"page_id"`
	Name           string         `json:"name"`
	Route          string         `json:"route"`
	Platform       string         `json:"platform"`
	Access         string         `json:"access"`
	Role           string         `json:"role"`
	Kind           string         `json:"kind,omitempty"`
	Purpose        string         `json:"purpose"`
	Hierarchy      []string       `json:"hierarchy,omitempty"`
	PrimaryAction  string         `json:"primary_action"`
	Secondary      []string       `json:"secondary_actions,omitempty"`
	Responsive     string         `json:"responsive,omitempty"`
	Components     []string       `json:"components"`
	Data           *core.PageData `json:"data,omitempty"`
	Interactions   []string       `json:"interactions"`
	EntryPoints    []string       `json:"entry_points"`
	States         []string       `json:"states"`
	Validation     []string       `json:"validation"`
	Tests          []string       `json:"tests"`
	FeatureID      string         `json:"feature_id"`
	SubFeatureIDs  []string       `json:"sub_feature_ids,omitempty"`
	RequirementIDs []string       `json:"requirement_ids"`
	FlowIDs        []string       `json:"flow_ids"`
}

// Blocker is a missing or broken part of the page contract; the design
// cannot be approved while any exists.
type Blocker struct {
	PageID string `json:"page_id,omitempty"`
	Page   string `json:"page,omitempty"`
	Text   string `json:"text"`
}

var stateNames = []struct{ key, label string }{
	{"loading", "memuat"}, {"empty", "kosong"}, {"no_results", "tanpa hasil"}, {"error", "error"}, {"success", "berhasil"},
}

func stateCopy(s *core.PageStates, key string) string {
	if s == nil {
		return ""
	}
	switch key {
	case "loading":
		return s.Loading
	case "empty":
		return s.Empty
	case "no_results":
		return s.NoResults
	case "error":
		return s.Error
	case "success":
		return s.Success
	}
	return ""
}

// navTarget resolves a nav item to the page of the same platform with the
// same name (the UI uses the same rule).
func navTarget(item string, from core.Page, pages []core.Page) *core.Page {
	name := strings.ToLower(strings.TrimSpace(item))
	for i := range pages {
		if pages[i].Platform == from.Platform && strings.ToLower(strings.TrimSpace(pages[i].Name)) == name {
			return &pages[i]
		}
	}
	return nil
}

func pageRef(pg *core.Page) string {
	if pg.Route != "" {
		return fmt.Sprintf("%s (%s)", pg.Name, pg.Route)
	}
	return pg.Name
}

// Handoffs returns the build spec of every designed page.
func Handoffs(p *core.Project) []PageHandoff {
	if p.Design == nil {
		return nil
	}
	pages := p.Design.Pages
	out := make([]PageHandoff, 0, len(pages))
	for _, pg := range pages {
		out = append(out, handoff(p, pg, pages))
	}
	return out
}

func handoff(p *core.Project, pg core.Page, pages []core.Page) PageHandoff {
	h := PageHandoff{PageID: pg.ID, Name: pg.Name, Route: pg.Route, Platform: pg.Platform, Access: pg.Access, Role: pg.Role,
		Kind: pg.Kind, Purpose: pg.Purpose, Hierarchy: pg.Hierarchy, Secondary: pg.SecondaryActions, Responsive: pg.Responsive,
		Data: pg.Data, FeatureID: pg.FeatureID, SubFeatureIDs: pg.SubFeatureIDs,
		RequirementIDs: featureReqs(p, pg.FeatureID)}
	byID := map[string]*core.Page{}
	for i := range pages {
		byID[pages[i].ID] = &pages[i]
	}
	exits := map[string]bool{}
	var exitOrder []string
	exit := func(to *core.Page) string {
		if !exits[to.ID] {
			exits[to.ID] = true
			exitOrder = append(exitOrder, to.Name)
		}
		return pageRef(to)
	}

	if a := pg.PrimaryAction; a != nil && a.Label != "" {
		h.PrimaryAction = a.Label
		if to := byID[a.Target]; to != nil {
			h.PrimaryAction += " → " + exit(to)
		}
	} else if a != nil {
		h.PrimaryAction = "tidak ada: " + a.Reason
	}

	for _, c := range pg.Components {
		h.Components = append(h.Components, componentSpec(c))
		to := byID[c.Target]
		switch {
		case c.Type == "nav":
			for _, it := range c.Items {
				if t := navTarget(it, pg, pages); t != nil && t.ID != pg.ID {
					h.Interactions = append(h.Interactions, fmt.Sprintf("Menu “%s” → buka %s", it, exit(t)))
				}
			}
		case c.Type == "form":
			res := "tampilkan kondisi berhasil"
			if to != nil {
				res = "buka " + exit(to)
			}
			h.Interactions = append(h.Interactions, fmt.Sprintf("Kirim %s → validasi → %s", componentName(c), res))
		case to != nil && (c.Type == "list" || c.Type == "table"):
			h.Interactions = append(h.Interactions, fmt.Sprintf("Klik baris “%s” → buka %s", c.Label, exit(to)))
		case to != nil:
			h.Interactions = append(h.Interactions, fmt.Sprintf("Klik “%s” → buka %s", c.Label, exit(to)))
		}
		for _, f := range c.Fields {
			var rule []string
			if f.Required {
				rule = append(rule, "wajib")
			}
			rule = append(rule, f.Type)
			if f.Validation != "" {
				rule = append(rule, f.Validation)
			}
			v := fmt.Sprintf("%s: %s", f.Label, strings.Join(rule, ", "))
			if f.Error != "" {
				v += fmt.Sprintf(" — pesan “%s”", f.Error)
			}
			h.Validation = append(h.Validation, v)
			if f.Required || f.Validation != "" {
				h.Tests = append(h.Tests, "Validasi isian "+f.Label)
			}
		}
	}

	for _, from := range pages {
		if from.ID != pg.ID && linksTo(from, pg.ID, pages) {
			h.EntryPoints = append(h.EntryPoints, from.Name)
		}
	}
	for _, fl := range p.Flows {
		for _, id := range fl.PageIDs {
			if id == pg.ID {
				h.FlowIDs = append(h.FlowIDs, fl.ID)
				break
			}
		}
	}
	for _, s := range stateNames {
		if c := stateCopy(pg.States, s.key); c != "" {
			h.States = append(h.States, fmt.Sprintf("%s: %s", s.label, c))
			h.Tests = append(h.Tests, "Kondisi "+s.label+" tampil")
		}
	}

	tests := []string{"Halaman tampil di " + orDash(pg.Route)}
	if pg.Access == "auth" {
		tests = append(tests, "Tanpa login diarahkan ke halaman masuk")
	}
	if a := pg.PrimaryAction; a != nil && a.Label != "" {
		tests = append(tests, "Aksi utama “"+a.Label+"” berfungsi")
	}
	for _, n := range exitOrder {
		tests = append(tests, "Navigasi ke "+n+" berfungsi")
	}
	if pg.Platform == core.PlatformWeb {
		tests = append(tests, "Tampil di mobile tanpa scroll horizontal")
	}
	if len(h.Validation) > 0 {
		tests = append(tests, "Tiap isian punya label, error tampil di isiannya, form bisa dipakai dengan keyboard")
	}
	h.Tests = append(tests, h.Tests...)
	return h
}

func linksTo(from core.Page, id string, pages []core.Page) bool {
	if a := from.PrimaryAction; a != nil && a.Target == id {
		return true
	}
	for _, c := range from.Components {
		if c.Target == id {
			return true
		}
		if c.Type == "nav" {
			for _, it := range c.Items {
				if t := navTarget(it, from, pages); t != nil && t.ID == id {
					return true
				}
			}
		}
	}
	return false
}

func componentSpec(c core.PageComponent) string {
	s := componentName(c)
	switch {
	case len(c.Columns) > 0:
		s += " kolom: " + strings.Join(c.Columns, ", ")
	case len(c.Fields) > 0:
		var ls []string
		for _, f := range c.Fields {
			ls = append(ls, f.Label)
		}
		s += " isian: " + strings.Join(ls, ", ")
	case c.Type == "nav" && len(c.Items) > 0:
		s += ": " + strings.Join(c.Items, ", ")
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// Blockers lists contract gaps that stop approval: no design system,
// missing route, access, primary action or hierarchy, duplicate routes,
// duplicate actions, missing data states, tables without columns and nav
// items that open no page.
func Blockers(p *core.Project) []Blocker {
	if p.Design == nil {
		return nil
	}
	pages := p.Design.Pages
	var out []Blocker
	if p.Design.System == nil {
		out = append(out, Blocker{Text: "design system (warna, tipografi, spacing) belum ditentukan"})
	}
	routes := map[string]string{}
	for _, pg := range pages {
		add := func(text string) { out = append(out, Blocker{PageID: pg.ID, Page: pg.Name, Text: text}) }
		if pg.Route == "" {
			add("route belum ditentukan")
		} else if other, ok := routes[pg.Platform+" "+pg.Route]; ok {
			add(fmt.Sprintf("route %s sama dengan halaman “%s”", pg.Route, other))
		} else {
			routes[pg.Platform+" "+pg.Route] = pg.Name
		}
		if pg.Access == "" {
			add("akses (publik atau harus login) belum ditentukan")
		}
		if pg.PrimaryAction == nil {
			add("aksi utama belum ditentukan (atau alasan halaman tanpa aksi)")
		}
		if len(pg.Hierarchy) == 0 {
			add("urutan informasi (apa yang dilihat user lebih dulu) belum ditentukan")
		}
		for _, a := range duplicateActions(pg) {
			add(fmt.Sprintf("aksi “%s” tampil lebih dari sekali", a))
		}
		if missing := missingStates(pg); len(missing) > 0 {
			add("kondisi " + strings.Join(missing, ", ") + " belum ditentukan")
		}
		for _, c := range pg.Components {
			if c.Type == "table" && len(c.Columns) == 0 {
				add(fmt.Sprintf("tabel “%s” belum punya kolom", c.Label))
			}
			if c.Type == "nav" {
				var dead []string
				for _, it := range c.Items {
					if navTarget(it, pg, pages) == nil {
						dead = append(dead, it)
					}
				}
				if len(dead) > 0 {
					add("menu tidak menuju halaman: " + strings.Join(dead, ", "))
				}
			}
		}
	}
	return out
}

// duplicateActions lists action labels the page renders more than once:
// buttons and the submit button of each form.
func duplicateActions(pg core.Page) []string {
	count := map[string]int{}
	label := map[string]string{}
	var order []string
	see := func(l string) {
		k := strings.ToLower(strings.TrimSpace(l))
		if k == "" {
			return
		}
		if count[k] == 0 {
			label[k] = strings.TrimSpace(l)
			order = append(order, k)
		}
		count[k]++
	}
	for _, c := range pg.Components {
		switch c.Type {
		case "button":
			see(c.Label)
		case "form":
			see(FormSubmit(pg))
		}
	}
	var out []string
	for _, k := range order {
		if count[k] > 1 {
			out = append(out, label[k])
		}
	}
	return out
}

// dataKinds show collections; a list elsewhere (e.g. on a detail page) is
// attributes of one record and cannot be empty.
var dataKinds = map[string]bool{"list": true, "admin": true, "dashboard": true, "search": true}

// missingStates lists the states a data page must describe: loading and
// error when it loads data, empty too when it shows a collection.
func missingStates(pg core.Page) []string {
	list := false
	for _, c := range pg.Components {
		list = list || c.Type == "table" || c.Type == "list" && dataKinds[pg.Kind]
	}
	loads := list
	if pg.Data != nil {
		for _, a := range pg.Data.API {
			loads = loads || strings.HasPrefix(a, "GET ")
		}
	}
	if !loads {
		return nil
	}
	var out []string
	for _, s := range stateNames {
		need := s.key == "loading" || s.key == "error" || s.key == "empty" && list
		if need && stateCopy(pg.States, s.key) == "" {
			out = append(out, s.label)
		}
	}
	return out
}
