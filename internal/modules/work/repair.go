package work

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// RepairDesign closes the gaps verification reports on the design: every UI
// feature gets a page and every flow of a UI feature walks through the pages
// of its feature. Missing pages are minimal overview pages built from the
// feature itself (nothing invented); the design status is kept. Returns the
// pages added and the flows linked.
func RepairDesign(p *core.Project) (pages, flows int) {
	d := p.Design
	if d == nil {
		return 0, 0
	}
	has := map[string]bool{}
	routes := map[string]bool{}
	next := 0
	for _, pg := range d.Pages {
		has[pg.FeatureID] = true
		routes[pg.Platform+" "+pg.Route] = true
		var n int
		if _, err := fmt.Sscanf(pg.ID, "PAGE-%d", &n); err == nil && n > next {
			next = n
		}
	}
	var added []core.Page
	for _, f := range UIFeatures(p) {
		if has[f.ID] {
			continue
		}
		next++
		pg := featurePage(p, f, fmt.Sprintf("PAGE-%03d", next), routes)
		routes[pg.Platform+" "+pg.Route] = true
		added = append(added, pg)
	}
	if len(added) > 0 {
		core.EnsureIR(added, p.Name)
		d.Pages = append(d.Pages, added...)
		pages = len(added)
		// The change must show in the build: tasks and the verification
		// are rebuilt from the new page list by the caller.
	}

	byFeature := map[string][]string{}
	for _, pg := range d.Pages {
		byFeature[pg.FeatureID] = append(byFeature[pg.FeatureID], pg.ID)
	}
	ui := map[string]bool{}
	for _, f := range UIFeatures(p) {
		ui[f.ID] = true
	}
	for i := range p.Flows {
		fl := &p.Flows[i]
		if !ui[fl.FeatureID] || len(fl.PageIDs) > 0 || len(byFeature[fl.FeatureID]) == 0 {
			continue
		}
		fl.PageIDs = append([]string(nil), byFeature[fl.FeatureID]...)
		flows++
	}
	return pages, flows
}

// featurePage is the minimal page for a feature: a title and the list of its
// sub-features, read-only, behind login.
func featurePage(p *core.Project, f core.Feature, id string, routes map[string]bool) core.Page {
	platform := core.PlatformWeb
	for _, pl := range f.Platforms {
		if pl != core.PlatformBackend {
			platform = pl
			break
		}
	}
	role := "Pengguna"
	for _, fl := range p.Flows {
		if fl.FeatureID == f.ID && fl.Role != "" {
			role = fl.Role
			break
		}
	}
	var subIDs, subNames []string
	for _, s := range f.SubFeatures {
		subIDs = append(subIDs, s.ID)
		subNames = append(subNames, s.Name)
	}
	if len(subNames) == 0 {
		subNames = []string{f.Name}
	}
	purpose := strings.TrimSpace(f.Description)
	if purpose == "" {
		purpose = "Mengelola " + f.Name
	}
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(f.Name), "-"), "-")
	if slug == "" {
		slug = strings.ToLower(f.ID)
	}
	route := "/" + slug
	for n := 2; routes[platform+" "+route]; n++ {
		route = fmt.Sprintf("/%s-%d", slug, n)
	}
	return core.Page{
		ID: id, Name: f.Name, Platform: platform, Role: role, FeatureID: f.ID, SubFeatureIDs: subIDs,
		Purpose: purpose, Kind: "list", Density: "comfortable",
		Hierarchy: []string{f.Name, strings.Join(subNames, ", ")},
		Route:     route, Access: "auth",
		PrimaryAction: &core.PageAction{Reason: "Halaman ringkasan; aksi dilakukan di sub-fitur"},
		States: &core.PageStates{Loading: "Memuat " + f.Name + "…", Empty: "Belum ada data " + f.Name,
			Error: "Gagal memuat " + f.Name + ". Coba lagi."},
		Components: []core.PageComponent{
			{ID: "title", Type: "header", Label: f.Name},
			{ID: "items", Type: "list", Label: f.Name, Items: subNames},
		},
	}
}
