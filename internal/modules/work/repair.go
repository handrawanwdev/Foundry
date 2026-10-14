package work

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/scan"
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
		if has[f.ID] || len(PageIDsOf(p, f.ID)) > 0 {
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

// SmokeBrief is the task for the agent when the project does not prove that
// it runs: exit codes of build and test say nothing about a blank page, a
// dead API or a worker that never connected.
const SmokeBrief = `Project belum terbukti BERJALAN. Build dan test hijau belum cukup.
Buat script "smoke" di package.json root (mis. node scripts/smoke.mjs) yang:
1. Menyiapkan dependensi runtime (database/redis bila perlu; baca port yang benar dari konfigurasi, jangan menebak), menjalankan migrasi dan seed.
2. Menjalankan SEMUA app yang ADA di project (web, API, worker; jangan mengada-ada app yang tidak ada, mis. project hanya-klien cukup web) sebagai proses sungguhan, bukan mock.
3. Memeriksa tiap app: API menjawab endpoint health/ready dan satu endpoint nyata (login memakai akun seed), web menyajikan halaman "/" yang benar-benar merender (root tidak kosong, tidak ada route yang tidak cocok, tidak ada error console, request ke API tidak diblok CORS), worker tetap hidup dan terhubung ke queue.
4. Menghentikan semua proses di akhir dan keluar dengan kode non-nol jika satu saja gagal, dengan pesan yang menyebut app dan penyebabnya.
Jika smoke menemukan kegagalan, perbaiki penyebabnya di kode (route, CORS, konfigurasi, migrasi), bukan melemahkan pemeriksaan.`

// SmokeCriteriaFor lists what the smoke script must prove for the apps the
// project actually has: demanding an API or worker of a client-only app can
// never be met and would block the task forever.
func SmokeCriteriaFor(dir string) []string {
	var web, api, worker bool
	for _, c := range scan.Scan(dir).Components {
		switch c.Kind {
		case "web", "desktop", "mobile":
			web = true
		case "backend":
			api = true
			worker = worker || strings.Contains(strings.ToLower(c.Path), "worker")
		}
	}
	crit := []string{"npm/pnpm run smoke menjalankan semua app project sebagai proses sungguhan lalu keluar 0, dan keluar non-nol bila satu pemeriksaan gagal"}
	if web || !api {
		crit = append(crit, "Halaman utama web merender konten dan tidak ada error console")
	}
	if api {
		crit = append(crit, "API menjawab health/ready dan satu endpoint nyata")
	}
	if worker {
		crit = append(crit, "Worker hidup dan terhubung ke queue")
	}
	return crit
}
