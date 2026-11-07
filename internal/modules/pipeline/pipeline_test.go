package pipeline_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/agent"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
	"github.com/handrawanwdev/foundry/internal/modules/work"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

// The test binary doubles as a fake agent CLI: when FOUNDRY_FAKE_AGENT=1 it
// reads a prompt on stdin and answers like a real agent would.
func TestMain(m *testing.M) {
	if os.Getenv("FOUNDRY_FAKE_AGENT") == "1" {
		fakeAgent()
		return
	}
	os.Exit(m.Run())
}

const featureMapJSON = `{"summary":"Aplikasi klinik","modules":[{"name":"Janji Temu","features":[
  {"name":"Buat Janji","description":"Pasien membuat janji","purpose":"Mendapat jadwal periksa","benefit":"Tanpa antrian","outcome":"Janji tersimpan",
   "children":[{"name":"Cari dokter"},{"name":"Konfirmasi"}]}
]}]}`

const featureMapExpandJSON = `{"node":{"name":"Buat Janji","description":"Pasien memilih dokter dan jam","purpose":"Jadwal pasti","benefit":"Tanpa antrian","outcome":"Janji pending","children":[{"name":"Pilih jam"}]}}`

const featureMapGapsJSON = `{"modules":[{"name":"Tagihan","features":[{"name":"Invoice","description":"Tagihan setelah janji","purpose":"Pembayaran jelas","benefit":"Admin tidak mencatat manual","outcome":"Invoice terbit"}]}]}`

const discoveryJSON = `Here you go:
` + "```json" + `
{"name": "Klinik Sehat",
 "discovery": {"summary": "Aplikasi klinik", "business": "Klinik pratama", "users": ["Pasien", "Admin"],
   "roles": [{"name": "Pasien", "description": "membuat janji", "platform": "android"}, {"name": "Admin", "description": "kelola jadwal", "platform": "web"}],
   "data": ["Appointment: id, pasien, dokter, waktu"], "integrations": [], "security": ["Login wajib"], "workflows": ["Pasien membuat janji"]},
 "requirements": [{"id": "R1", "text": "Pasien dapat membuat janji temu", "priority": "MUST",
                   "acceptance": [{"kind": "behavior", "text": "Janji tersimpan dengan status pending", "evidence": "test integrasi"},
                                  {"kind": "negative", "text": "Jadwal bentrok ditolak dengan pesan jelas", "evidence": "test integrasi"}]},
                  {"id": "R2", "text": "Admin dapat mengelola jadwal dokter", "priority": "MUST",
                   "acceptance": [{"id": "ngawur", "kind": "Behavior", "text": "Jadwal baru tampil di tabel"}, {"kind": "edge", "text": "Jadwal di masa lalu ditolak"}]}],
 "features": [{"id": "F1", "name": "Janji Temu", "description": "Pasien membuat janji", "requirement_ids": ["R1"], "platforms": ["android", "backend"],
               "sub_features": [{"id": "x", "name": "Cari dokter"}, {"id": "y", "name": "Buat janji"}]},
              {"id": "F2", "name": "Jadwal Dokter", "description": "Admin kelola jadwal", "requirement_ids": ["R2"], "platforms": ["web", "backend"],
               "sub_features": [{"id": "z", "name": "Tambah jadwal"}]}],
 "flows": [{"id": "FL1", "name": "Buat janji", "feature_id": "F1", "role": "Pasien", "goal": "Mendapat jadwal periksa",
            "preconditions": ["Pasien sudah login"], "steps": ["Pilih dokter", "Pilih jadwal", "Konfirmasi"], "postconditions": ["Janji tersimpan (pending)"],
            "failures": ["Jadwal sudah terisi: pasien memilih jam lain"], "permissions": ["appointment:create"], "data_effects": ["Appointment: dibuat"]}],
 "artifacts": [{"id": "threat-model", "title": "Threat Model", "category": "quality", "body": "## Aset\n\n- Data pasien", "refs": ["requirements"]},
               {"id": "siklus-data", "title": "Siklus Data", "category": "data", "area": "data", "body": "## Retensi\n\n- Rekam janji 5 tahun"}],
 "definition_of_done": ["Semua acceptance criteria terbukti dengan test", "Tidak ada secret di kode"],
 "decisions": [{"kind": "assumption", "text": "Satu klinik, satu zona waktu", "area": "data"}, {"kind": "aneh", "text": "Notifikasi lewat SMS?"}, {"kind": "question", "text": " "}],
 "skip": [{"area": "operations", "reason": "Prototipe lokal tanpa server produksi"}, {"area": "acceptance", "reason": "tidak boleh"}],
 "platform": {"targets": [{"kind": "web", "audience": ["Admin"], "rationale": "dashboard"}, {"kind": "android", "audience": ["Pasien"], "rationale": "HP"},
                          {"kind": "backend", "audience": [], "rationale": "API"}], "reason": "Pasien memakai HP, admin memakai web."}}
` + "```"

// designJSON is the designer's first answer: rules, one shell, three pages
// and the pages of the flow. Ids, platform and access are not normalized
// yet; a blank principle, token and state are dropped.
const designJSON = `{"users": [{"role": "Pasien", "platform": "android", "context": "memesan dari HP", "goals": ["GOAL-1"], "tasks": ["Cari dokter", "Konfirmasi janji"]},
   {"role": "Admin", "platform": "web", "context": "mengelola jadwal di kantor", "goals": ["GOAL-2"], "tasks": ["Kelola jadwal"]}],
 "goals": [{"id": "GOAL-1", "role": "Pasien", "text": "Mendapat jadwal periksa", "success": "Janji tersimpan", "priority": "must"},
           {"id": "GOAL-2", "role": "Admin", "text": "Mengelola jadwal dokter", "success": "Jadwal tampil di tabel", "priority": "must"}],
 "navigation": [{"name": "Pasien", "items": [{"label": "Cari dokter", "page": "dokter", "primary": true}]},
                {"name": "Admin", "items": [{"label": "Jadwal", "page": "jadwal"}]}],
 "assumptions": ["Pasien memakai satu akun untuk semua janji"],
 "unknowns": ["Jumlah janji per hari belum diketahui"],
 "decisions": [{"subject": "dokter", "choice": "list", "why": ["pasien memilih dari sedikit dokter"], "basis": ["GOAL-1", "FLOW-001"], "source": "fact"},
               {"subject": "konfirmasi", "choice": "form", "why": ["satu langkah konfirmasi dengan data ringkas"], "basis": ["GOAL-1", "FLOW-001", "appointment:create"], "source": "fact"},
               {"subject": "jadwal", "choice": "table", "why": ["admin membandingkan banyak jadwal"], "basis": ["GOAL-2", "FEAT-002", "Admin", "web"], "source": "fact"}],
 "rules": {"source": "NEW", "direction": "Tenang dan jelas", "principles": ["Satu aksi utama per layar", " "],
  "ux_guidelines": ["Konfirmasi sebelum membatalkan janji"],
  "tokens": {"colors": [{"name": "action.primary", "value": "#1f6feb", "use": "aksi utama"}, {"name": " ", "value": " "}],
             "typography": [{"name": "type.body", "value": "Inter 14px/20px 400"}], "spacing": [{"name": "space.md", "value": "16px"}],
             "breakpoints": [{"name": "breakpoint.tablet", "value": "768px"}]},
  "components": [{"name": "Button", "use": "aksi", "variants": ["primary"], "rules": ["disabled saat mengirim"]}, {"name": "DataTable", "use": "daftar data"}],
  "accessibility": ["Kontras WCAG AA"], "responsive": ["Tabel jadi list di bawah breakpoint.tablet"],
  "platforms": [{"platform": "Android", "rules": ["Material 3"]}]},
 "shells": [{"id": "admin", "name": "Admin", "platform": "web", "pattern": "sidebar", "navigation": "Menu kiri: Jadwal",
   "regions": [{"id": "nav", "name": "Navigasi", "content": ["Logo", "Menu"]}]}],
 "pages": [
  {"id": "Dokter", "name": "Daftar Dokter", "platform": "android", "route": "Dokter", "role": "Pasien", "access": "AUTH", "feature_id": "FEAT-001",
   "purpose": "Cari dokter", "goal": "GOAL-1", "pattern": "list", "primary_action": "Pilih",
   "fields": [{"field": "Doctor.name", "label": "Nama", "presentation": "card-title"}, {"field": "Doctor.specialty", "label": "Spesialis", "presentation": "card-meta"}],
   "sections": [{"id": "daftar", "name": "Daftar dokter", "content": ["Kartu dokter: nama, spesialis, jadwal"], "components": ["Button"]}],
   "links": [{"to": "konfirmasi", "via": "Tombol Pilih"}],
   "states": {"loading": "Memuat dokter", "empty": "Belum ada dokter", "error": "Gagal memuat. Coba lagi.", "success": " "}},
  {"id": "konfirmasi", "name": "Konfirmasi Janji", "platform": "android", "route": "Konfirmasi", "role": "Pasien", "access": "auth",
   "feature_id": "FEAT-001", "purpose": "Konfirmasi janji", "goal": "GOAL-1", "pattern": "form", "primary_action": "Kirim",
   "fields": [{"field": "Appointment.scheduled_at", "label": "Jadwal", "presentation": "form-input", "format": "date-time"}],
   "sections": [{"id": "ringkasan", "name": "Ringkasan", "content": ["Dokter, tanggal, jam"]},
                {"id": "form", "name": "Form", "content": ["Tanggal", "Catatan"], "components": ["Button"]}],
   "hierarchy": ["form", "ringkasan"],
   "states": {"error": "Jadwal bentrok", "success": "Janji dibuat", "disabled": "Tombol Kirim nonaktif saat mengirim"}},
  {"id": "jadwal", "name": "Admin Jadwal", "platform": "web", "route": "/jadwal", "role": "Admin", "access": "auth", "feature_id": "FEAT-002",
   "shell": "admin", "purpose": "Kelola jadwal", "goal": "GOAL-2", "pattern": "table", "primary_action": "Tambah jadwal",
   "fields": [{"field": "Schedule.day", "label": "Hari", "presentation": "table-column"}, {"field": "Doctor.name", "label": "Dokter", "presentation": "table-column"}],
   "sections": [{"id": "tabel", "name": "Tabel jadwal", "content": ["Hari, Dokter, Jam"], "components": ["DataTable"], "spacing": "space.md"}],
   "states": {"loading": "Skeleton tabel", "empty": "Belum ada jadwal", "error": "Jadwal gagal dimuat"}}],
 "flows": {"FLOW-001": ["Dokter", "konfirmasi"]}}`

// webDesignJSON is the design of the web platform only.
const webDesignJSON = `{"users": [{"role": "Admin", "platform": "web", "context": "mengelola jadwal di kantor", "tasks": ["Kelola jadwal"]}],
 "goals": [{"id": "GOAL-2", "role": "Admin", "text": "Mengelola jadwal dokter", "success": "Jadwal tampil di tabel", "priority": "must"}],
 "navigation": [{"name": "Admin", "items": [{"label": "Jadwal", "page": "jadwal"}]}],
 "decisions": [{"subject": "jadwal", "choice": "table", "why": ["admin membandingkan banyak jadwal"], "basis": ["GOAL-2", "FEAT-002", "Admin", "web"], "source": "fact"}],
 "rules": {"source": "new", "principles": ["Tabel padat"], "ux_guidelines": ["Konfirmasi hapus"],
  "tokens": {"colors": [{"name": "action.primary", "value": "#1f6feb"}], "typography": [{"name": "type.body", "value": "Inter 14px"}],
             "spacing": [{"name": "space.md", "value": "16px"}]}, "accessibility": ["Kontras WCAG AA"],
  "responsive": ["Tabel jadi list di bawah breakpoint.tablet"]},
 "shells": [{"id": "admin", "name": "Admin", "platform": "web", "pattern": "sidebar", "navigation": "Menu kiri: Jadwal",
   "regions": [{"id": "nav", "name": "Navigasi", "content": ["Logo", "Menu"]}]}],
 "pages": [{"id": "jadwal", "name": "Admin Jadwal", "platform": "web", "route": "/jadwal", "role": "Admin", "access": "auth",
   "feature_id": "FEAT-002", "shell": "admin", "purpose": "Kelola jadwal", "goal": "GOAL-2", "pattern": "table", "primary_action": "Tambah jadwal",
   "sections": [{"id": "tabel", "name": "Tabel jadwal", "content": ["Hari, Dokter, Jam"], "components": ["DataTable"], "spacing": "space.md"}],
   "states": {"loading": "Skeleton tabel", "empty": "Belum ada jadwal", "error": "Jadwal gagal dimuat"}}],
 "flows": {}}`

// revisedDesignJSON adds a page, removes one and changes the flow; the
// link to the removed page is dropped by Foundry.
const revisedDesignJSON = `{"pages": [{"id": "profil", "name": "Profil", "platform": "android", "route": "Profil", "access": "auth",
   "feature_id": "FEAT-001", "purpose": "Data diri", "goal": "GOAL-1", "pattern": "form",
   "sections": [{"id": "form", "name": "Form profil", "content": ["Nama", "Telepon"]}],
   "states": {"error": "Gagal menyimpan", "success": "Tersimpan", "disabled": "Simpan nonaktif saat mengirim"},
   "links": [{"to": "dokter", "via": "Kembali"}]}],
 "decisions": [{"subject": "profil", "choice": "form", "why": ["pasien memperbarui data diri"], "basis": ["GOAL-1", "FEAT-001"], "source": "fact"}],
 "remove_pages": ["konfirmasi"],
 "flows": {"FLOW-001": ["dokter", "profil"]}}`

func fakeAgent() {
	in, _ := io.ReadAll(os.Stdin)
	prompt := string(in)
	mode := os.Getenv("FAKE_MODE")
	if strings.HasPrefix(mode, "overloaded") {
		runs, _ := os.ReadFile(os.Getenv("FAKE_RUNS"))
		countRun("attempt")
		if mode == "overloaded" || len(runs) == 0 {
			fmt.Fprintln(os.Stderr, "Error: Upstream request failed: [overloaded] the server is overloaded; retry shortly")
			os.Exit(1)
		}
	}
	switch {
	case strings.HasPrefix(prompt, "Balas hanya"):
		fmt.Println("OK")
	case strings.Contains(prompt, "unified blueprint architect of Foundry"):
		countRun("plan")
		refused := strings.Contains(prompt, "YOUR LAST ANSWER WAS REFUSED")
		fmt.Println(fakeUnifiedAnswer(prompt, (mode == "tech_invalid_once" && !refused) || mode == "tech_invalid"))
	case strings.Contains(prompt, "technology architect of Foundry"):
		countRun("tech")
		refused := strings.Contains(prompt, "YOUR LAST ANSWER WAS REFUSED")
		fmt.Println(fakeTechAnswer(prompt, (mode == "tech_invalid_once" && !refused) || mode == "tech_invalid"))
	case strings.Contains(prompt, "repository architect of Foundry"):
		countRun("repo")
		refused := strings.Contains(prompt, "YOUR LAST ANSWER WAS REFUSED")
		fmt.Println(fakeRepoAnswer(prompt, mode == "repo_invalid_once" && !refused))
	case strings.Contains(prompt, "feature mapper"):
		countRun("feature_map")
		if mode == "bad_json" && strings.Contains(prompt, "ACTION: generate") && strings.Contains(prompt, "YOUR LAST ANSWER WAS REFUSED") {
			fmt.Println(featureMapJSON)
			return
		}
		if strings.Contains(prompt, "ACTION: expand") {
			fmt.Println(featureMapExpandJSON)
			return
		}
		if strings.Contains(prompt, "ACTION: gaps") {
			fmt.Println(featureMapGapsJSON)
			return
		}
		fmt.Println(featureMapJSON)
	case strings.Contains(prompt, "discovery analyst"):
		countRun("discovery")
		if mode == "bad_json" {
			fmt.Println("maaf, saya tidak bisa")
			return
		}
		if strings.HasPrefix(mode, "area") {
			fmt.Println(openAreasDiscovery())
			return
		}
		if mode == "embedded" {
			fmt.Println(embeddedDiscoveryJSON)
			return
		}
		if mode == "critical" {
			fmt.Println(strings.Replace(discoveryJSON, `{"kind": "aneh", "text": "Notifikasi lewat SMS?"}`,
				`{"kind": "question", "text": "Notifikasi lewat SMS atau WhatsApp?", "area": "integration", "critical": true}`, 1))
			return
		}
		fmt.Println(discoveryJSON)
	case strings.Contains(prompt, "UI/UX designer of Foundry"):
		countRun("design")
		refused := strings.Contains(prompt, "YOUR LAST ANSWER WAS REFUSED")
		switch {
		case strings.Contains(prompt, "Tambahkan halaman profil"):
			fmt.Println(revisedDesignJSON)
		case strings.Contains(prompt, "rubah ini"):
			fmt.Println("```json\n{}\n```")
		case mode == "design_invalid_once" && !refused:
			fmt.Println(strings.Replace(designJSON, `"pattern": "list"`, `"pattern": "hero"`, 1))
		default:
			fmt.Println(designJSON)
		}
	case strings.Contains(prompt, "workflow planner"):
		countRun("workflow")
		switch {
		case strings.Contains(prompt, "Keputusan berikut berubah setelah command ditulis"):
			if strings.Contains(prompt, "→ Keputusan: WhatsApp lewat penyedia resmi") {
				countRun("workflow-sync")
			}
			fmt.Println(`{"commands": []}`)
		case strings.Contains(prompt, "dihapus dari blueprint"):
			fmt.Println(cleanupWorkflowJSON(prompt))
		case strings.Contains(prompt, "DETAIL PHASE"):
			src := workflowJSON
			if mode == "embedded" {
				src = embeddedWorkflowJSON
			}
			fmt.Println(detailAnswer(prompt, src))
		case strings.Contains(prompt, "CURRENT COMMANDS"):
			fmt.Println(revisionRound(prompt, workflowRevisionChanges...))
		case mode == "embedded":
			fmt.Println(embeddedWorkflowJSON)
		case mode == "cycle_once" && !strings.Contains(prompt, "YOUR LAST ANSWER WAS REFUSED"):
			fmt.Println(cyclicWorkflowJSON)
		default:
			fmt.Println(workflowJSON)
		}
	case strings.Contains(prompt, "software architect of Foundry"):
		countRun("structure")
		refused := strings.Contains(prompt, "YOUR LAST ANSWER WAS REFUSED")
		fmt.Println(fakeStructurePlan(prompt, mode == "structure_invalid_once" && !refused))
	case strings.Contains(prompt, "blueprint editor"):
		countRun("blueprint")
		if ms, _ := strconv.Atoi(os.Getenv("FAKE_AREA_MS")); ms > 0 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
		if mode == "area_data_fail" && strings.Contains(prompt, "tulis area data") {
			fmt.Fprintln(os.Stderr, "agent mati")
			os.Exit(1)
		}
		if mode == "area_empty_once" && strings.Contains(prompt, "Lengkapi blueprint: tulis area") && !strings.Contains(prompt, "YOUR LAST ANSWER WAS REFUSED") {
			fmt.Println(`{"artifacts": [{"id": "catatan-lain", "title": "Catatan", "category": "other", "body": "## Catatan\n\nTidak terkait."}]}`)
			return
		}
		if strings.Contains(prompt, "Sesuaikan blueprint dengan keputusan") {
			if strings.Contains(prompt, "DECISIONS CHANGED") && strings.Contains(prompt, "→ Keputusan: WhatsApp lewat penyedia resmi") {
				countRun("blueprint-sync")
			}
			fmt.Println("```json\n{}\n```") // nothing contradicts the decision
			return
		}
		if strings.Contains(prompt, "dihapus dari blueprint") {
			if mode == "cleanup_fail" {
				fmt.Println("maaf, saya tidak bisa")
				return
			}
			fmt.Println(cleanupArtifactsJSON)
			return
		}
		if strings.Contains(prompt, "Lengkapi blueprint: tulis area data") {
			fmt.Println(`{"artifacts": [{"id": "data-lifecycle", "title": "Siklus Data", "category": "data", "body": "## Retensi\n\nAsumsi: 5 tahun."}]}`)
			return
		}
		if m := requestedArea.FindStringSubmatch(prompt); m != nil {
			fmt.Printf(`{"artifacts": [{"id": "isi-%s", "title": "Isi %s", "category": "quality", "area": %q, "body": "## Isi\n\nAsumsi: 5 tahun."}]}`+"\n", m[1], m[1], m[1])
			return
		}
		fmt.Println(artifactsJSON)
	default:
		fmt.Fprintln(os.Stderr, "unknown prompt")
		os.Exit(3)
	}
}

func unwrapJSONFence(s string) json.RawMessage {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSpace(strings.TrimSuffix(s, "```"))
	return json.RawMessage(s)
}

func fakeUnifiedSections(prompt string) (tech, arch, repo bool) {
	_, after, ok := strings.Cut(prompt, "SECTIONS TO PRODUCE:")
	if !ok {
		return true, true, true
	}
	line, _, _ := strings.Cut(after, "\n")
	return strings.Contains(line, "technology"), strings.Contains(line, "architecture"), strings.Contains(line, "repository")
}

func fakeUnifiedAnswer(prompt string, invalid bool) string {
	needTech, needArch, needRepo := fakeUnifiedSections(prompt)
	body := map[string]any{}
	var techHint string
	if needTech {
		raw := unwrapJSONFence(fakeTechPlanJSON(prompt, invalid))
		body["technology"] = json.RawMessage(raw)
		techHint = fakeTechArchHint(raw)
	}
	if needArch {
		body["architecture"] = json.RawMessage(fakeStructurePlan(prompt+techHint, false))
	}
	if needRepo {
		body["repository"] = unwrapJSONFence(fakeRepoAnswer(fakeUnifiedRepoPrompt(prompt, techHint), false))
	}
	if strings.Contains(prompt, "EXISTING PROJECT") {
		body["adaptation"] = []map[string]any{{
			"action": "reuse", "target": "existing layout", "reason": "Konvensi repository masih relevan",
		}}
	}
	raw, _ := json.Marshal(body)
	return "```json\n" + string(raw) + "\n```"
}

func fakeTechArchHint(raw json.RawMessage) string {
	var plan struct {
		Items []struct {
			Area, Technology string
		} `json:"items"`
	}
	if json.Unmarshal(raw, &plan) != nil {
		return ""
	}
	var b strings.Builder
	for _, it := range plan.Items {
		if it.Area == core.TechArchitecture || it.Area == core.TechRepository {
			fmt.Fprintf(&b, "\n- %s: %s", it.Area, it.Technology)
		}
	}
	return b.String()
}

func fakeUnifiedRepoPrompt(prompt, techHint string) string {
	var b strings.Builder
	b.WriteString(fakeStructurePartsBlock(prompt + techHint))
	b.WriteByte('\n')
	for _, header := range []string{"PROJECT MODULES", "TARGET PLATFORM:", "DOMAINS THAT MUST APPEAR"} {
		lines := promptLines(prompt, header)
		if len(lines) == 0 {
			continue
		}
		b.WriteString(header + "\n")
		for _, l := range lines {
			b.WriteString("- " + l + "\n")
		}
		b.WriteByte('\n')
	}
	b.WriteString(techHint)
	return b.String()
}

func fakeStructurePartsBlock(prompt string) string {
	domains := promptDomainList(prompt)
	if len(domains) == 0 {
		domains = promptSectionIDs(prompt, "DOMAIN MODULES")
	}
	var b strings.Builder
	b.WriteString("STRUCTURE PARTS (exact refs; each must be linked to exactly one folder through part):\n")
	for _, a := range promptPlatforms(prompt) {
		fmt.Fprintf(&b, "- app:%s (default folder apps/%s/)\n", a, a)
	}
	root := "apps/api/"
	if fakeStructureType(prompt) == core.StructMicroservices {
		root = "services/api/"
	}
	fmt.Fprintf(&b, "- service:api (default folder %s)\n", root)
	for _, d := range domains {
		fmt.Fprintf(&b, "- module:%s (default folder %smodules/%s/)\n", d, root, d)
	}
	return b.String()
}

// fakeStructurePlan answers the structure analysis with a plan built from the
// prompt's own domains and platforms. dropLast omits one domain, like an AI
// that overlooks a blueprint module.
func fakeStructurePlan(prompt string, dropLast bool) string {
	domains := promptDomainList(prompt)
	if len(domains) == 0 {
		domains = promptSectionIDs(prompt, "DOMAIN MODULES")
	}
	if dropLast && len(domains) > 1 {
		domains = domains[:len(domains)-1]
	}
	var apps []core.StructurePart
	for _, a := range promptPlatforms(prompt) {
		apps = append(apps, core.StructurePart{Name: a, Reason: "Aplikasi " + a + " pada platform yang dipilih"})
	}
	reasons := map[string]string{}
	var planDomains []core.StructureDomain
	var modules []string
	var services []core.StructurePart
	for _, d := range domains {
		reasons[d] = "Tanggung jawab domain " + d
		planDomains = append(planDomains, core.StructureDomain{Name: d, Responsibility: reasons[d]})
		modules = append(modules, d)
		services = append(services, core.StructurePart{Name: d, Reason: "Domain " + d + " dipisah"})
	}
	kind := fakeStructureType(prompt)
	oneAPI := &core.StructureIR{Type: kind, Docs: "docs/", Apps: apps,
		Services: []core.StructurePart{{Name: "api", Reason: "Satu API melayani semua domain", Modules: modules, ModuleReasons: reasons}}}
	perDomain := &core.StructureIR{Type: kind, Docs: "docs/", Apps: apps, Services: services}
	plan := core.StructurePlan{
		Summary:         "Sistem dengan domain yang dipisah jelas.",
		Drivers:         []string{"Domain bisnis jelas"},
		Domains:         planDomains,
		DependencyRules: []string{"Modul hanya memanggil modul lain lewat public API."},
		Recommendation:  "recommended",
		Reason:          "Satu API dengan modul per domain paling sederhana.",
		Alternatives: []core.StructureAlternative{
			{ID: "recommended", Name: "Satu API per domain", Summary: "Satu deploy, modul per domain.", Type: kind, IR: oneAPI,
				Evaluation: core.StructureEvaluation{Maintainability: 5, Simplicity: 5, Extensibility: 3, Testability: 4, Separation: 4, Complexity: 2}},
			{ID: "alt-2", Name: "Service per domain", Summary: "Tiap domain punya service sendiri.", Type: kind, IR: perDomain,
				Evaluation: core.StructureEvaluation{Maintainability: 3, Simplicity: 2, Extensibility: 5, Testability: 3, Separation: 5, Complexity: 4}},
		},
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// promptDomainList reads the exact domain ids the analysis must place.
func promptDomainList(prompt string) []string {
	const header = "DOMAINS THAT MUST APPEAR IN EVERY ALTERNATIVE (exact ids): "
	for _, line := range strings.Split(prompt, "\n") {
		if !strings.HasPrefix(line, header) {
			continue
		}
		var out []string
		for _, d := range strings.Split(strings.TrimPrefix(line, header), ",") {
			if d = strings.TrimSpace(d); d != "" {
				out = append(out, d)
			}
		}
		return out
	}
	return nil
}

// fakeStructureType follows the approved architecture/repository decision, so
// the fake answer is not refused for a technology mismatch.
func fakeStructureType(prompt string) string {
	monorepo, micro := false, false
	for _, line := range strings.Split(prompt, "\n") {
		if strings.Contains(line, "|") {
			continue // catalog rows, not the decision
		}
		if strings.HasPrefix(line, "- repository:") && strings.Contains(line, "Monorepo") {
			monorepo = true
		}
		if strings.HasPrefix(line, "- architecture:") && strings.Contains(line, "Microservices") {
			micro = true
		}
	}
	switch {
	case micro:
		return core.StructMicroservices
	case monorepo:
		return core.StructMonorepo
	}
	return core.StructModular
}

// promptSectionIDs reads the first field of "- id | ..." lines under a
// prompt section header.
func promptSectionIDs(prompt, header string) []string {
	var out []string
	in := false
	for _, line := range strings.Split(prompt, "\n") {
		switch {
		case strings.HasPrefix(line, header):
			in = true
		case in && strings.HasPrefix(line, "- "):
			id := strings.TrimSpace(strings.SplitN(strings.TrimPrefix(line, "- "), " | ", 2)[0])
			if id != "" {
				out = append(out, id)
			}
		case in && strings.TrimSpace(line) == "":
			return out
		}
	}
	return out
}

// promptPlatforms maps the target platform lines to the app names the IR uses.
func promptPlatforms(prompt string) []string {
	names := map[string]string{"web": "web", "android": "mobile", "ios": "mobile", "desktop": "desktop", "cli": "cli"}
	var out []string
	in := false
	for _, line := range strings.Split(prompt, "\n") {
		switch {
		case strings.HasPrefix(line, "TARGET PLATFORM:"):
			in = true
		case in && strings.HasPrefix(line, "- "):
			kind := strings.TrimSpace(strings.SplitN(strings.TrimPrefix(line, "- "), ":", 2)[0])
			name := names[kind]
			if name != "" && !slices.Contains(out, name) {
				out = append(out, name)
			}
		case in && strings.TrimSpace(line) == "":
			return out
		}
	}
	return out
}

// detailFor matches the command a detail round asks for ("- CMD-002: ...").
var detailFor = regexp.MustCompile(`DETAIL PHASE - Fill in details for these \d+ commands:\n- CMD-(\d+):`)

// detailAnswer answers a detail round from a full workflow answer: the
// outline numbered its commands CMD-001, CMD-002, ... in answer order, so
// the requested command is the n-th one, sent back under its CMD id.
func detailAnswer(prompt, raw string) string {
	m := detailFor.FindStringSubmatch(prompt)
	if m == nil {
		return `{"commands": []}`
	}
	n, _ := strconv.Atoi(m[1])
	var out struct {
		Commands []map[string]any `json:"commands"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil || n < 1 || n > len(out.Commands) {
		return `{"commands": []}`
	}
	c := out.Commands[n-1]
	c["id"] = fmt.Sprintf("CMD-%03d", n)
	b, _ := json.Marshal(map[string]any{"commands": []any{c}})
	return string(b)
}

// submitWorkflow answers a workflow job as an MCP host: the outline first,
// then every detail round Foundry queues, one command at a time.
func submitWorkflow(s *pipeline.Service, raw string) error {
	if err := s.SubmitWork(work.KindWorkflow, raw); err != nil {
		return err
	}
	for {
		item, err := s.NextWork()
		if err != nil {
			return err
		}
		if item.Kind != work.KindWorkflow || !strings.Contains(item.Prompt, "DETAIL PHASE") {
			return nil
		}
		if err := s.SubmitWork(work.KindWorkflow, detailAnswer(item.Prompt, raw)); err != nil {
			return err
		}
	}
}

const workflowJSON = `{"commands": [
 {"id": "C1", "name": "Kontrak API janji temu", "objective": "Tetapkan kontrak API", "instructions": "Periksa repository. Tulis kontrak API janji temu.",
  "artifacts": ["requirements", "architecture"], "covers": ["REQ-001", "req-002"], "expected_outputs": ["openapi.yaml"], "acceptance_criteria": ["Kontrak mencakup REQ-001"]},
 {"id": "C2", "name": "Backend janji temu", "objective": "Endpoint janji", "instructions": "Implementasikan endpoint sesuai kontrak C1.",
  "depends_on": ["C1"], "artifacts": ["architecture"]},
 {"id": "C3", "name": "Layar janji", "objective": "Layar janji temu", "instructions": "Bangun layar dokter, konfirmasi, dan jadwal sesuai desain.",
  "depends_on": ["C1"], "artifacts": ["design-rules", "layout-spec", "requirements"], "covers": ["REQ-001"]}]}`

// cleanupArtifactsJSON is the AI's rewrite of the threat model after a
// document it referred to was deleted.
const cleanupArtifactsJSON = `{"artifacts": [{"id": "threat-model", "title": "Threat Model", "category": "quality", "area": "security",
  "summary": "Ancaman utama tanpa siklus data", "body": "## Aset\n\n- Data pasien", "refs": ["requirements"]}]}`

// cleanupWorkflowJSON rewrites exactly the commands the cleanup request is
// about, like an AI that follows "do not change other commands".
func cleanupWorkflowJSON(prompt string) string {
	rewrites := map[string]string{
		"CMD-002": `{"id": "CMD-002", "name": "Backend janji temu", "objective": "Endpoint janji", "instructions": "Implementasikan endpoint sesuai kontrak CMD-001.",
  "depends_on": ["CMD-001"], "artifacts": ["architecture"]}`,
		"CMD-003": `{"id": "CMD-003", "name": "Layar janji", "objective": "Layar janji temu", "instructions": "Bangun layar dari requirement; desain menyusul.",
  "depends_on": ["CMD-001"], "artifacts": ["requirements"], "covers": ["REQ-001"]}`,
	}
	_, about, _ := strings.Cut(prompt, "Commands the request is about: ")
	about, _, _ = strings.Cut(about, ".\n")
	var out []string
	for _, id := range strings.Split(about, ", ") {
		if r, ok := rewrites[id]; ok {
			out = append(out, r)
		}
	}
	return revisionRound(prompt, out...)
}

const cyclicWorkflowJSON = `{"commands": [
 {"id": "C1", "name": "A", "objective": "a", "instructions": "a", "depends_on": ["C2"], "artifacts": ["requirements"]},
 {"id": "C2", "name": "B", "objective": "b", "instructions": "b", "depends_on": ["C1"], "artifacts": ["requirements"]}]}`

// workflowRevisionChanges is the revision "tambahkan test integrasi", one
// command change per round.
var workflowRevisionChanges = []string{
	`{"id": "CMD-002", "name": "Backend janji temu + notifikasi", "objective": "Endpoint janji", "instructions": "Implementasikan endpoint dan notifikasi.",
  "depends_on": ["CMD-001"], "artifacts": ["architecture"]}`,
	`{"id": "NEW1", "name": "Test integrasi", "objective": "Uji alur janji", "instructions": "Tulis test integrasi alur janji.",
  "depends_on": ["CMD-002", "CMD-003"], "artifacts": ["requirements"], "acceptance_criteria": ["Alur buat janji lulus"]}`,
}

// earlierRounds reads how many changes the earlier rounds of a revision made.
var earlierRounds = regexp.MustCompile(`EARLIER ROUNDS of this request \(already saved in CURRENT COMMANDS\): ([^\n]*)\.\n`)

// revisionRound answers one round of a sequential revision: the next of the
// given command changes, with done on the last one.
func revisionRound(prompt string, changes ...string) string {
	k := 0
	if m := earlierRounds.FindStringSubmatch(prompt); m != nil {
		k = len(strings.Split(m[1], ", "))
	}
	if k >= len(changes) {
		return `{"commands": [], "done": true}`
	}
	return fmt.Sprintf(`{"commands": [%s], "done": %t}`, changes[k], k == len(changes)-1)
}

const artifactsJSON = `{"artifacts": [{"id": "Threat Model", "title": "Threat Model", "category": "quality", "summary": "Ancaman utama",
  "body": "## Aset\n\n- Data pasien", "refs": ["requirements", "ghost"]}]}`

const embeddedWorkflowJSON = `{"commands": [
 {"id": "C1", "name": "Driver sensor", "objective": "Baca sensor suhu", "instructions": "Periksa repository. Implementasikan driver I2C.",
  "artifacts": ["firmware-architecture"]},
 {"id": "C2", "name": "Protokol MQTT", "objective": "Kirim data", "instructions": "Kirim pembacaan lewat MQTT.", "artifacts": ["requirements"], "covers": ["REQ-001"]},
 {"id": "C3", "name": "Integrasi", "objective": "Gabungkan", "instructions": "Hubungkan driver dan protokol.", "depends_on": ["C1", "C2"],
  "artifacts": ["firmware-architecture"]}]}`

const embeddedDiscoveryJSON = `{"name": "Sensor Suhu",
 "discovery": {"summary": "Firmware sensor suhu", "business": "Pantau suhu gudang", "users": ["Operator"], "roles": [], "data": [], "integrations": ["MQTT broker"],
   "security": [], "workflows": []},
 "requirements": [{"id": "R1", "text": "Kirim suhu tiap menit", "priority": "MUST", "acceptance": [{"kind": "behavior", "text": "Pesan MQTT tiap 60 detik"}, {"kind": "edge", "text": "Broker putus: data disimpan lalu dikirim ulang"}]}],
 "definition_of_done": ["Diuji di perangkat"],
 "skip": [{"area": "use-cases", "reason": "Perangkat tanpa interaksi pengguna"}, {"area": "security", "reason": "Jaringan tertutup; dicatat sebagai asumsi"}, {"area": "operations", "reason": "Belum ada server pemantau"}],
 "features": [{"id": "F1", "name": "Telemetri", "description": "Kirim suhu", "requirement_ids": ["R1"], "platforms": ["embedded"], "sub_features": [{"name": "Baca sensor"}, {"name": "Kirim MQTT"}]}],
 "flows": [],
 "platform": {"targets": [{"kind": "embedded", "audience": ["Operator"], "rationale": "perangkat"}], "reason": "Perangkat di gudang."},
 "artifacts": [{"id": "firmware-architecture", "title": "Arsitektur Firmware", "category": "architecture", "body": "## Task\n\n- sensor\n- mqtt"},
               {"id": "requirements", "title": "Bentrok", "body": "id bawaan"},
               {"id": "integrations", "title": "Integrasi MQTT", "category": "platform", "body": "## Broker\n\nPerlu verifikasi: alamat broker."}]}`

// countRun appends label to $FAKE_RUNS, so a test can count agent runs.
// requestedArea is the area a "complete the blueprint" request asks for.
var requestedArea = regexp.MustCompile(`USER REQUEST:\nLengkapi blueprint: tulis area (\S+) \(`)

// openAreasDiscovery leaves data, security and operations for their own jobs.
func openAreasDiscovery() string {
	s := discoveryJSON
	for _, cut := range []string{
		`{"id": "threat-model", "title": "Threat Model", "category": "quality", "body": "## Aset\n\n- Data pasien", "refs": ["requirements"]},`,
		`{"id": "siklus-data", "title": "Siklus Data", "category": "data", "area": "data", "body": "## Retensi\n\n- Rekam janji 5 tahun"}`,
		`{"area": "operations", "reason": "Prototipe lokal tanpa server produksi"}, `,
	} {
		if !strings.Contains(s, cut) {
			panic("fixture changed: " + cut)
		}
		s = strings.Replace(s, cut, "", 1)
	}
	return s
}

func countRun(label string) {
	path := os.Getenv("FAKE_RUNS")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, label)
}

type fakeAI struct {
	mcp  bool
	none bool
}

func (f fakeAI) Runner() (*agent.Runner, bool, error) {
	if f.none {
		return nil, false, pipeline.ErrNoAI
	}
	if f.mcp {
		return nil, true, nil
	}
	r, err := agent.NewRunner("fake", []string{os.Args[0]}, agent.Options{})
	return r, false, err
}

func newService(t *testing.T, ai pipeline.AI, mode string) *pipeline.Service {
	t.Helper()
	t.Setenv("FOUNDRY_FAKE_AGENT", "1")
	t.Setenv("FAKE_MODE", mode)
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	st, err := core.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Init("test"); err != nil {
		t.Fatal(err)
	}
	return pipeline.New(st, ai)
}

func load(t *testing.T, s *pipeline.Service) *core.Project {
	t.Helper()
	p, err := s.Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustJob(t *testing.T, p *core.Project, kind string, st core.JobStatus) {
	t.Helper()
	if p.Job == nil || p.Job.Kind != kind || p.Job.Status != st {
		t.Fatalf("job = %+v, want %s/%s", p.Job, kind, st)
	}
}

// designReady drives Tell → Feature Map → Discovery → Design.
func designReady(t *testing.T, s *pipeline.Service) *core.Project {
	t.Helper()
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	clearLandedPlan(t, s)
	p := load(t, s)
	if p.Stage != core.StageDesign || p.UI == nil || !p.UI.Written() {
		t.Fatalf("design not ready: stage=%s ui=%v job=%+v", p.Stage, p.UI, p.Job)
	}
	return p
}

// designReadyKeepPlan drives the whole first blueprint run and keeps it.
func designReadyKeepPlan(t *testing.T, s *pipeline.Service) *core.Project {
	t.Helper()
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	return load(t, s)
}

// clearLandedPlan drops the auto-chained unified plan so tests that
// exercise standalone tech/structure/repo start from a blank slate.
func clearLandedPlan(t *testing.T, s *pipeline.Service) {
	t.Helper()
	if _, err := s.Store.Update(func(p *core.Project) error {
		if st := p.Structure; st != nil {
			st.Plan, st.Repo = nil, nil
			st.PlanRequest, st.RepoRequest = nil, nil
			st.Adaptation = nil
			st.Approved, st.Recommended, st.Status = false, "", ""
			st.Proposal = nil
		}
		if p.Stack != nil {
			p.Stack.Plan = nil
			p.Stack.PlanRequest = nil
			p.Stack.Proposal = nil
		}
		if p.UI != nil && p.Stack != nil {
			p.UI.TechVersion = p.Stack.Version()
		}
		p.BlueprintQueue = nil
		p.BlueprintBatch = 0
		p.Job = &core.Job{Kind: work.KindDesign, Status: core.JobDone, Message: "Menyusun aturan desain & layout selesai"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// holdAutoPlan cancels a waiting unified-plan job started after design
// so MCP tests can drive GenerateBlueprint or standalone plans themselves.
func holdAutoPlan(t *testing.T, s *pipeline.Service) {
	t.Helper()
	p := load(t, s)
	if p.Job == nil || p.Job.Kind != work.KindPlan {
		return
	}
	if p.Job.Status == core.JobFailed {
		if err := s.DismissJob(); err != nil {
			t.Fatal(err)
		}
		return
	}
	if _, err := s.Store.Update(func(p *core.Project) error {
		if p.Stack != nil {
			p.Stack.PlanRequest = nil
		}
		if p.Structure != nil {
			p.Structure.PlanRequest = nil
			p.Structure.RepoRequest = nil
		}
		p.BlueprintQueue = nil
		p.BlueprintBatch = 0
		p.Job = &core.Job{Kind: work.KindDesign, Status: core.JobDone, Message: "Menyusun aturan desain & layout selesai", Started: p.Job.Started}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func mcpAfterStory(t *testing.T, s *pipeline.Service) {
	t.Helper()
	if err := s.SubmitWork(work.KindFeatureMap, featureMapJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
}

func TestEndToEndAgent(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	p := designReady(t, s)

	if p.Stage != core.StageDesign || len(p.Requirements) != 2 || len(p.Features) != 2 {
		t.Fatalf("discovery: stage=%s reqs=%d feats=%d", p.Stage, len(p.Requirements), len(p.Features))
	}
	if p.Features[0].SubFeatures[1].ID != "FEAT-001.2" {
		t.Fatalf("sub-feature id renumber: %+v", p.Features[0].SubFeatures)
	}
	// Platform detection → monorepo suggested because mobile is needed.
	if p.Structure.Mode != core.StructureNew || p.Structure.Template != "monorepo" {
		t.Fatalf("structure = %+v", p.Structure)
	}
	// The design is written and normalized: ids, platform, access and
	// source are lowercased, blanks are dropped.
	r := p.UI.Rules
	if r.Source != core.RulesNew || len(r.Principles) != 1 || len(r.Tokens.Colors) != 1 || r.Platforms[0].Platform != "android" {
		t.Fatalf("rules = %+v", r)
	}
	pg := p.UI.Layout.Pages
	if len(pg) != 3 || pg[0].ID != "dokter" || pg[0].Access != "auth" || pg[0].States["success"] != "" || len(pg[0].States) != 3 {
		t.Fatalf("pages = %+v", pg)
	}
	if got := p.Flows[0].PageIDs; !slices.Equal(got, []string{"dokter", "konfirmasi"}) {
		t.Fatalf("flow pages = %v", got)
	}
	if errs := core.Errors(p.DesignIssues(nil)); len(errs) != 0 || p.UI.Version != 1 || p.UI.TechVersion != p.Stack.Version() {
		t.Fatalf("issues = %+v ui = %+v", errs, p.UI)
	}
	// Both parts are blueprint artifacts.
	ids := map[string]bool{}
	for _, a := range spec.Artifacts(p) {
		ids[a.ID] = true
	}
	if !ids[core.ArtifactDesignRules] || !ids[core.ArtifactLayoutSpec] || ids["ui-design"] {
		t.Fatalf("artifacts = %v", ids)
	}

	// User override: structure type, applied after its impact.
	if _, err := s.ProposeStructure([]core.StructureOp{{Op: "type", Name: "modular"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyStructureProposal(); err != nil {
		t.Fatal(err)
	}
	if err := approve(s); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if p.Stage != core.StageReady || p.Job != nil && p.Job.Status == core.JobRunning {
		t.Fatalf("finalize: stage=%s job=%+v", p.Stage, p.Job)
	}
	// The export package lands in docs/foundry for an external coding agent.
	root := filepath.Join(p.Dir, "docs", "foundry")
	raw, err := os.ReadFile(filepath.Join(root, "spec.json"))
	if err != nil {
		t.Fatalf("spec.json missing: %v", err)
	}
	var out spec.Spec
	if err := json.Unmarshal(raw, &out); err != nil || out.Layout == nil || len(out.Layout.Pages) != 3 || out.DesignRules == nil {
		t.Fatalf("spec.json = %s, %v", raw, err)
	}
	for _, f := range []string{"README.md", "manifest.json", "blueprint/design-rules.md", "blueprint/layout-spec.md", "blueprint/requirements.md",
		"design/design-rules.json", "design/layout-spec.json", "workflow/workflow.json", "workflow/CMD-001.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s missing: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "design-brief.md")); !os.IsNotExist(err) {
		t.Errorf("design-brief.md written: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "blueprint", "layout-spec.md")); err != nil || !strings.Contains(string(b), "/jadwal") {
		t.Fatalf("layout-spec.md = %s, %v", b, err)
	}
	// The screen command points at the design it implements.
	if b, err := os.ReadFile(filepath.Join(root, "workflow", "CMD-003.md")); err != nil ||
		!strings.Contains(string(b), "blueprint/design-rules.md") || !strings.Contains(string(b), "blueprint/layout-spec.md") {
		t.Fatalf("CMD-003.md = %s, %v", b, err)
	}
	// No code is written: no scaffold folders.
	if _, err := os.Stat(filepath.Join(p.Dir, "apps", "api", "modules")); err == nil {
		t.Fatal("finalize must not scaffold code")
	}
}

// Finalize locks the design; reopening allows another revision.
func TestFinalizeThenReopen(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := approve(s); err != nil {
		t.Fatal(err)
	}
	if err := s.ReviseDesign("ubah warna", nil); !errors.Is(err, pipeline.ErrFinalized) {
		t.Fatalf("revise after finalize err = %v", err)
	}
	if _, err := s.EditDesignRules(core.DesignRules{}); !errors.Is(err, pipeline.ErrFinalized) {
		t.Fatalf("edit after finalize err = %v", err)
	}
	if err := s.Reopen(); err != nil {
		t.Fatal(err)
	}
	if p := load(t, s); p.Stage != core.StageDesign {
		t.Fatalf("reopen: stage=%s", p.Stage)
	}
	if err := s.Reopen(); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("reopen twice err = %v", err)
	}
}

func TestNoAIConnected(t *testing.T) {
	s := newService(t, fakeAI{none: true}, "")
	if _, err := s.SubmitStory("Aplikasi laundry untuk pelanggan"); !errors.Is(err, pipeline.ErrNoAI) {
		t.Fatalf("err = %v", err)
	}
	mustJob(t, load(t, s), work.KindFeatureMap, core.JobFailed)
}

func TestInvalidDiscoveryOutput(t *testing.T) {
	s := newService(t, fakeAI{}, "bad_json")
	s.SubmitStory("Aplikasi keuangan pribadi")
	s.Wait()
	mustJob(t, load(t, s), work.KindFeatureMap, core.JobDone)
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindDiscovery, core.JobFailed)
	if p.Job.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestDiscoveryCompletesBlueprint(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	b := p.Blueprint
	if b == nil || b.Module("identity") == nil || b.Database.Engine == "" || b.Auth.Hashing == "" {
		t.Fatalf("blueprint = %+v", b)
	}
	if !strings.Contains(strings.Join(p.Structure.Layout, "\n"), "identity") && p.Stack.Choice(core.TechArchitecture) == core.ArchModular {
		t.Fatalf("layout missing identity module: %v", p.Structure.Layout)
	}
}

func finalize(t *testing.T, s *pipeline.Service) {
	t.Helper()
	designReady(t, s)
	if err := approve(s); err != nil {
		t.Fatal(err)
	}
	s.Wait()
}

func TestExistingProjectKeepsStructure(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	dir := s.Store.Dir
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n\ngo 1.22\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "web"), 0o755)
	os.WriteFile(filepath.Join(dir, "web", "package.json"), []byte(`{"dependencies": {"react": "18"}, "scripts": {"build": "vite build", "test": "vitest"}}`), 0o644)
	os.MkdirAll(filepath.Join(dir, "web", "src", "components"), 0o755)
	os.WriteFile(filepath.Join(dir, "web", "src", "router.tsx"), []byte(`[{ path: "/orders" }, { path: "/settings" }]`), 0o644)
	os.WriteFile(filepath.Join(dir, "web", "src", "components", "Sidebar.tsx"), []byte(`[{ label: "Pesanan" }]`), 0o644)
	os.WriteFile(filepath.Join(dir, "web", "src", "index.css"), []byte(`:root { --primary: #7c3aed; --background: #ffffff; --foreground: #111827; --border: #e5e7eb; }`), 0o644)
	p := designReady(t, s)
	// The existing frontend is read: the design rules follow its tokens and
	// components, and the prompt knows its routes and navigation.
	ui := p.Structure.UI
	if ui == nil || ui.Navigation != "side" || len(ui.Routes) != 2 || ui.Tokens == nil || !slices.Equal(ui.TokenSources, []string{"web/src/index.css"}) {
		t.Fatalf("ui = %+v", p.Structure.UI)
	}
	if prompt := work.DesignPrompt(p); !strings.Contains(prompt, "EXISTING UI") || !strings.Contains(prompt, "/orders, /settings") ||
		!strings.Contains(prompt, "Pesanan") || !strings.Contains(prompt, "#7c3aed") {
		t.Fatalf("design prompt misses the existing ui")
	}
	r := p.UI.Rules
	primary := slices.IndexFunc(r.Tokens.Colors, func(t core.Token) bool { return t.Name == "action.primary" })
	if r.Source != core.RulesExisting || primary < 0 || r.Tokens.Colors[primary].Value != "#7c3aed" ||
		!slices.Contains(r.Sources, "web/src/index.css") {
		t.Fatalf("rules = %+v", r)
	}
	if !slices.ContainsFunc(r.Components, func(c core.ComponentRule) bool {
		return c.Name == "Sidebar" && c.Source == "web/src/components/Sidebar.tsx"
	}) {
		t.Fatalf("components = %+v", r.Components)
	}
	if p.Structure.Mode != core.StructureExisting {
		t.Fatalf("mode = %s", p.Structure.Mode)
	}
	kinds := map[string]bool{}
	for _, c := range p.Structure.Components {
		kinds[c.Kind] = true
	}
	if !kinds["backend"] || !kinds["web"] {
		t.Fatalf("components = %+v", p.Structure.Components)
	}
	// The stack keeps what the code uses; replacing it is a migration the
	// user must explain, and it is reviewed like any other change.
	if d := p.Stack.Decision(core.TechBackend); d == nil || d.Choice() != "Go" || p.Stack.DecisionState(*d) != core.StateExisting {
		t.Fatalf("backend = %+v", d)
	}
	if _, err := s.ProposeStack([]stack.Change{{Area: core.TechBackend, Value: "Python + FastAPI"}}); !errors.Is(err, stack.ErrExistingReason) {
		t.Fatalf("replace existing without reason err = %v", err)
	}
	if p, err := s.ProposeStack([]stack.Change{{Area: core.TechBackend, Value: "Python + FastAPI", Reason: "tim data"}}); err != nil ||
		p.Stack.Proposal == nil || p.Stack.Choice(core.TechBackend) != "Go" {
		t.Fatalf("propose: %v", err)
	}
	if _, err := s.DiscardStackProposal(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ProposeStructure([]core.StructureOp{{Op: "type", Name: "simple"}}); err == nil {
		t.Fatal("template must not replace existing structure")
	}
}

func TestDetectStructureKeepsNewIR(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Structure = &core.Structure{Mode: core.StructureNew, Template: "modular", IR: &core.StructureIR{Type: core.StructModular}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	p, err := s.DetectStructure()
	if err != nil {
		t.Fatal(err)
	}
	if p.Structure == nil || p.Structure.Template != "modular" || p.Structure.IR == nil || p.Structure.IR.Type != core.StructModular {
		t.Fatalf("new IR wiped: %+v", p.Structure)
	}
}

func TestSubmitStoryDetectsExistingBeforeDiscovery(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if err := os.WriteFile(filepath.Join(s.Store.Dir, "package.json"), []byte(`{"name":"dompetku","dependencies":{"react-native":"0.76.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := s.SubmitStory("Saya ingin menambahkan anggaran bulanan ke aplikasi keuangan yang sudah ada")
	if err != nil {
		t.Fatal(err)
	}
	if p.Structure == nil || p.Structure.Mode != core.StructureExisting {
		t.Fatalf("structure after story = %+v", p.Structure)
	}
	prompt := work.DiscoveryPrompt(p)
	if !strings.Contains(prompt, "EXISTING PROJECT COMPONENTS") || !strings.Contains(prompt, "package.json") {
		t.Fatalf("discovery prompt misses existing project: %s", prompt)
	}
	if p.Intent != core.IntentExisting {
		t.Fatalf("intent = %s", p.Intent)
	}
}

func TestSubmitStoryIntentChangeOnExisting(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	dir := s.Store.Dir
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "internal", "billing"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "billing", "service.go"), []byte("package billing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := s.SubmitStoryIntent("Saya ingin menambahkan notifikasi invoice", nil, core.IntentNewFeature, "billing sudah ada")
	if err != nil {
		t.Fatal(err)
	}
	if p.Intent != core.IntentNewFeature {
		t.Fatalf("intent = %s", p.Intent)
	}
	if p.Structure == nil || p.Structure.Context == nil || p.Structure.Context.Corrections != "billing sudah ada" {
		t.Fatalf("context %+v", p.Structure)
	}
	if !strings.Contains(work.DiscoveryPrompt(p), "FEATURE CHANGE BLUEPRINT") {
		t.Fatal("discovery prompt misses change blueprint")
	}
}

func TestMCPHostFlow(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mustJob(t, load(t, s), work.KindFeatureMap, core.JobWaiting)

	w, _ := s.NextWork()
	if w.Kind != work.KindFeatureMap || !strings.Contains(w.Prompt, "feature mapper") || !strings.Contains(w.Prompt, "klinik") {
		t.Fatalf("work = %+v", w)
	}
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("wrong kind err = %v", err)
	}
	if err := s.SubmitWork(work.KindFeatureMap, featureMapJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	mustJob(t, load(t, s), work.KindDiscovery, core.JobWaiting)
	w, _ = s.NextWork()
	if w.Kind != work.KindDiscovery || !strings.Contains(w.Prompt, "klinik") || !strings.Contains(w.Prompt, "FEATURE MAP") {
		t.Fatalf("work = %+v", w)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("wrong kind err = %v", err)
	}
	if err := s.SubmitWork(work.KindDiscovery, "bukan json"); err == nil {
		t.Fatal("invalid json accepted")
	}
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	mustJob(t, load(t, s), work.KindDesign, core.JobWaiting) // chained
	if w, _ := s.NextWork(); w.Kind != work.KindDesign || !strings.Contains(w.Prompt, "UI/UX designer") || !strings.Contains(w.Prompt, "FLOW-001") {
		t.Fatalf("work = %+v", w)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	holdAutoPlan(t, s)
	if d := load(t, s).UI; d == nil || len(d.Layout.Pages) != 3 || d.Layout.Shells[0].ID != "admin" {
		t.Fatalf("design = %+v", d)
	}
	if err := approve(s); err != nil {
		t.Fatal(err)
	}
	if p := load(t, s); p.Stage != core.StageReady {
		t.Fatalf("stage = %s", p.Stage)
	}
}

// A truncated inventory (rules + pages without sections) lands and the next
// MCP work is a detail round, not a full rewrite.
func TestMCPDesignSketchChainsToDetails(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mcpAfterStory(t, s)
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	sketch := `{"rules": {"source": "new", "principles": ["Satu aksi"], "ux_guidelines": ["Konfirmasi"],
	  "accessibility": ["AA"], "responsive": ["List di HP"],
	  "tokens": {"colors": [{"name": "action.primary", "value": "#1f6feb"}], "typography": [{"name": "type.body", "value": "Inter 14px"}],
	              "spacing": [{"name": "space.md", "value": "16px"}]},
	  "components": [{"name": "Button", "use": "aksi"}]},
	 "pages": [{"id": "dokter", "name": "Dokter", "platform": "android", "route": "Dokter", "purpose": "Cari dokter", "pattern": "list", "feature_id": "FEAT-001"},
	           {"id": "jadwal", "name": "Jadwal", "platform": "web", "route": "/jadwal", "purpose": "Kelola jadwal", "pattern": "table", "feature_id": "FEAT-002"}]}`
	if err := s.SubmitWork(work.KindDesign, sketch); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if !work.DesignNeedsDetails(p) || p.Job == nil || p.Job.Kind != work.KindDesign || p.Job.Status != core.JobWaiting {
		t.Fatalf("expected waiting detail job, job=%+v needs=%v", p.Job, work.DesignNeedsDetails(p))
	}
	w, _ := s.NextWork()
	if w.Kind != work.KindDesign || !strings.Contains(w.Prompt, "DETAIL PHASE") || !strings.Contains(w.Prompt, "dokter") {
		t.Fatalf("detail work = %+v", w)
	}
	detail := `{"pages": [
	  {"id": "dokter", "name": "Dokter", "platform": "android", "route": "Dokter", "purpose": "Cari dokter", "pattern": "list", "feature_id": "FEAT-001",
	   "sections": [{"id": "daftar", "name": "Daftar", "content": ["Kartu dokter"]}], "states": {"loading": "x", "empty": "x", "error": "x"}},
	  {"id": "jadwal", "name": "Jadwal", "platform": "web", "route": "/jadwal", "purpose": "Kelola jadwal", "pattern": "table", "feature_id": "FEAT-002",
	   "sections": [{"id": "tabel", "name": "Tabel", "content": ["Hari, Dokter"]}], "states": {"loading": "x", "empty": "x", "error": "x"}}]}`
	if err := s.SubmitWork(work.KindDesign, detail); err != nil {
		t.Fatal(err)
	}
	holdAutoPlan(t, s)
	p = load(t, s)
	if work.DesignNeedsDetails(p) || p.Job.Status != core.JobDone || len(p.UI.Layout.Pages) != 2 {
		t.Fatalf("after detail job=%+v pages=%d needs=%v", p.Job, len(p.UI.Layout.Pages), work.DesignNeedsDetails(p))
	}
}

// mcpDesigned drives an MCP project to a written design.
func mcpDesigned(t *testing.T, s *pipeline.Service) {
	t.Helper()
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mcpAfterStory(t, s)
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	holdAutoPlan(t, s)
}

func TestDesignRevise(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	if err := s.ReviseDesign("Tambahkan halaman profil", []string{"ghost"}); err == nil {
		t.Fatal("unknown focus accepted")
	}
	if err := s.ReviseDesign("Tambahkan halaman profil", []string{core.ArtifactLayoutSpec, "dokter"}); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindDesign, core.JobDone)
	var ids []string
	for _, pg := range p.UI.Layout.Pages {
		ids = append(ids, pg.ID)
	}
	if !slices.Equal(ids, []string{"dokter", "jadwal", "profil"}) || p.UI.Request != nil || p.UI.Version != 2 {
		t.Fatalf("pages = %v ui = %+v", ids, p.UI)
	}
	// The link and flow step to the removed page are dropped.
	if len(p.UI.Layout.Pages[0].Links) != 0 || !slices.Equal(p.Flows[0].PageIDs, []string{"dokter", "profil"}) {
		t.Fatalf("links = %+v flow = %v", p.UI.Layout.Pages[0].Links, p.Flows[0].PageIDs)
	}
	// Only the layout changed: the screen command needs review, the others
	// do not.
	r := pipeline.CheckWorkflow(p)
	if !slices.Equal(r.Stale["CMD-003"], []string{core.ArtifactLayoutSpec}) || len(r.Stale["CMD-001"]) != 0 || r.Ready {
		t.Fatalf("report = %+v", r)
	}
	if b := pipeline.Blockers(p); !slices.Contains(b, "workflow: 1 masalah dependency/isi") {
		t.Fatalf("blockers = %v", b)
	}
	// An outline still waiting for its details is not judged yet.
	keep := p.Workflow.Commands[0].Instructions
	p.Workflow.Commands[0].Instructions = ""
	if b := pipeline.Blockers(p); !slices.Contains(b, fmt.Sprintf("command workflow belum lengkap: 1 dari %d command belum berisi detail", len(p.Workflow.Commands))) {
		t.Fatalf("blockers while filling = %v", b)
	}
	p.Workflow.Commands[0].Instructions = keep
	// The new page `profil` has no command yet: the design change also
	// requires the workflow to be revised, not only reviewed.
	if r := pipeline.CheckWorkflow(p); !slices.ContainsFunc(r.Issues, func(is workflow.Issue) bool {
		return strings.Contains(is.Message, "profil")
	}) {
		t.Fatalf("issues = %+v", r.Issues)
	}
	if _, err := s.ApproveStack(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveStructure(); err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(); !errors.Is(err, pipeline.ErrIncomplete) {
		t.Fatalf("finalize with stale command err = %v", err)
	}
	if _, err := s.ReviewCommand("CMD-003", false, ""); err != nil {
		t.Fatal(err)
	}
	if r := pipeline.CheckWorkflow(load(t, s)); len(r.Review) != 0 {
		t.Fatalf("review = %+v", r.Review)
	}
}

// An invalid answer is refused as a whole and retried once with the reason.
func TestDesignInvalidRetriedOnce(t *testing.T) {
	runs := filepath.Join(t.TempDir(), "runs")
	t.Setenv("FAKE_RUNS", runs)
	s := newService(t, fakeAI{}, "design_invalid_once")
	p := designReady(t, s)
	if b, _ := os.ReadFile(runs); strings.Count(string(b), "design") != 2 {
		t.Fatalf("design runs = %q", b)
	}
	if p.UI.Request != nil || len(p.UI.Layout.Pages) != 3 || !slices.ContainsFunc(p.Events, func(e core.Event) bool { return strings.Contains(e.Message, "hero") }) {
		t.Fatalf("ui = %+v", p.UI)
	}
}

// A request the AI finds nothing for fails after one retry; the design is
// unchanged and the request can be dismissed.
func TestDesignNoChange(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	before := designReady(t, s).UI
	if err := s.ReviseDesign("rubah ini", nil); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindDesign, core.JobFailed)
	if p.UI.Request == nil || !strings.Contains(p.UI.Request.Rejected, "tidak menemukan perubahan") || !reflect.DeepEqual(p.UI.Layout, before.Layout) {
		t.Fatalf("ui = %+v", p.UI)
	}
	if err := s.DismissJob(); err != nil {
		t.Fatal(err)
	}
	if p := load(t, s); p.Job != nil || p.UI.Request != nil || !p.UI.Written() {
		t.Fatalf("after dismiss: job %+v ui %+v", p.Job, p.UI)
	}
	// Generate only starts while nothing is written.
	if err := s.GenerateDesign(); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("generate twice err = %v", err)
	}
}

// The user edits rules and layout by hand; an invalid edit is refused
// whole and the change marks the commands that follow it for review.
func TestDesignEdit(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	mcpDesigned(t, s)
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	l := *p.UI.Layout
	l.Pages = slices.Clone(l.Pages)
	l.Pages[2].States = map[string]string{"loading": "Skeleton"} // a table needs empty and error too
	_, err := s.EditLayout(l)
	var invalid *core.Invalid
	if !errors.As(err, &invalid) || !strings.Contains(err.Error(), `state "empty" wajib`) {
		t.Fatalf("invalid edit err = %v", err)
	}
	if load(t, s).UI.Version != p.UI.Version {
		t.Fatal("invalid edit saved")
	}
	// Removing a page drops the links and flow steps to it.
	l.Pages = []core.LayoutPage{p.UI.Layout.Pages[0], p.UI.Layout.Pages[2]}
	if _, err := s.EditLayout(l); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if len(p.UI.Layout.Pages[0].Links) != 0 || !slices.Equal(p.Flows[0].PageIDs, []string{"dokter"}) {
		t.Fatalf("after remove: %+v %v", p.UI.Layout.Pages[0].Links, p.Flows[0].PageIDs)
	}
	rules := *p.UI.Rules
	rules.Principles = nil
	if _, err := s.EditDesignRules(rules); err == nil || !strings.Contains(err.Error(), "principles belum diisi") {
		t.Fatalf("rules err = %v", err)
	}
	rules.Principles = []string{"Data dulu, dekorasi belakangan"}
	if _, err := s.EditDesignRules(rules); err != nil {
		t.Fatal(err)
	}
	r := pipeline.CheckWorkflow(load(t, s))
	if !slices.Equal(r.Stale["CMD-003"], []string{core.ArtifactDesignRules, core.ArtifactLayoutSpec}) || len(r.Stale["CMD-002"]) != 0 {
		t.Fatalf("stale = %+v", r.Stale)
	}
}

// A product with screens is not finalized while its design has errors,
// and a workflow must implement the design.
func TestFinalizeNeedsValidDesign(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	mcpDesigned(t, s)
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.UI.Rules.Accessibility = nil // e.g. a design migrated from an older version
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if b := pipeline.Blockers(load(t, s)); !slices.Contains(b, "rancangan UI: accessibility belum diisi") {
		t.Fatalf("blockers = %v", b)
	}
	if err := approve(s); !errors.Is(err, pipeline.ErrIncomplete) {
		t.Fatalf("finalize err = %v", err)
	}
	// A workflow that never refers to the layout spec is refused.
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.UI.Rules.Accessibility = []string{"Kontras AA"}
		p.Workflow = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.GenerateWorkflow(); err != nil {
		t.Fatal(err)
	}
	noUI := strings.Replace(workflowJSON, `"design-rules", "layout-spec", `, ``, 1)
	if err := submitWorkflow(s, noUI); err == nil || !strings.Contains(err.Error(), "tidak ada command yang merujuk artefak design-rules") {
		t.Fatalf("workflow without design err = %v", err)
	}
}

func TestTechStack(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mcpAfterStory(t, s)
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	// Web + android + backend with two features: a simple monorepo.
	p := load(t, s)
	st := p.Stack
	if st == nil || st.Profile != "saas" || st.Choice(core.TechFrontend) != "React + TypeScript + Vite" ||
		st.Choice(core.TechMobile) != "React Native + TypeScript + Expo" || st.Choice(core.TechBackend) != "NestJS + TypeScript" ||
		st.Choice(core.TechArchitecture) != core.ArchSimple || st.Choice(core.TechRepository) != core.RepoMonorepo {
		t.Fatalf("stack = %+v", st)
	}
	if st.State() != core.TechRecommended || st.Version() != 1 {
		t.Fatalf("status = %s v%d", st.State(), st.Version())
	}
	if p.Structure.Template != "monorepo" || !slices.Contains(p.Structure.Layout, "apps/mobile/") {
		t.Fatalf("structure = %+v", p.Structure)
	}
	propose := func(area, value, reason string) (*core.Project, error) {
		return s.ProposeStack([]stack.Change{{Area: area, Value: value, Reason: reason}})
	}
	// A change is a proposal: nothing changes until the user applies it.
	p, err := propose(core.TechFrontend, "vue + typescript + vite", "Tim sudah pakai Vue")
	if err != nil {
		t.Fatal(err)
	}
	if pr := p.Stack.Proposal; pr == nil || p.Stack.State() != core.TechReview || p.Stack.Choice(core.TechFrontend) != "React + TypeScript + Vite" ||
		len(pr.Changes) != 2 || pr.Changes[0].To != "Vue + TypeScript + Vite" || len(pr.Impact) != 2 || !pr.Impact[0].Design {
		t.Fatalf("proposal = %+v", p.Stack.Proposal)
	}
	// The UI library follows the frontend; the change reaches the technology artifact.
	pr := p.Stack.Proposal
	if c := pr.Changes[1]; c.Area != core.TechUI || !c.Follow || c.To != "PrimeVue" {
		t.Fatalf("follow = %+v", c)
	}
	if pr.Affected == nil || !slices.Contains(pr.Affected.Artifacts, "technology") {
		t.Fatalf("affected = %+v", pr.Affected)
	}
	for _, bad := range [][2]string{{core.TechArchitecture, "Serverless"}, {core.TechDesktop, "Tauri + React + TypeScript"}, {core.TechCache, "Memcached"}} {
		if _, err := propose(bad[0], bad[1], ""); err == nil {
			t.Fatalf("%v must fail", bad)
		}
	}
	if _, err := s.ApproveStack(); !errors.Is(err, stack.ErrPending) {
		t.Fatalf("approve with proposal err = %v", err)
	}
	p, err = s.DiscardStackProposal()
	if err != nil || p.Stack.Proposal != nil || p.Stack.State() != core.TechRecommended {
		t.Fatalf("discard: %v %+v", err, p.Stack)
	}
	if _, err := propose(core.TechFrontend, "vue + typescript + vite", "Tim sudah pakai Vue"); err != nil {
		t.Fatal(err)
	}
	if _, err := propose(core.TechRepository, core.RepoSimple, ""); err != nil { // merged into the same proposal
		t.Fatal(err)
	}
	p, err = s.ApplyStackProposal()
	if err != nil {
		t.Fatal(err)
	}
	if p.Stack.State() != core.TechImpactChecked || p.Stack.Version() != 2 || len(p.Stack.History[1].Changes) != 3 {
		t.Fatalf("applied = %+v", p.Stack)
	}
	if p.Structure.Template != "simple" || !slices.Equal(p.Structure.Layout, []string{"apps/web/", "apps/mobile/", "apps/api/", "docs/"}) {
		t.Fatalf("structure = %+v", p.Structure)
	}
	// A platform change decides again but keeps the user's choices.
	p, err = s.SetPlatform([]core.PlatformTarget{{Kind: "web"}, {Kind: "backend"}})
	if err != nil {
		t.Fatal(err)
	}
	d := p.Stack.Decision(core.TechFrontend)
	if p.Stack.Decision(core.TechMobile) != nil || d.Choice() != "Vue + TypeScript + Vite" || d.Lang() != "TypeScript" ||
		d.Override.Reason != "Tim sudah pakai Vue" || p.Stack.Choice(core.TechRepository) != core.RepoSimple || p.Stack.Version() != 3 {
		t.Fatalf("stack = %+v", p.Stack)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err == nil || !strings.Contains(err.Error(), `platform "android" tidak dipilih`) {
		t.Fatalf("pages of a dropped platform err = %v", err)
	}
	if err := s.RetryJob(); err != nil && !errors.Is(err, pipeline.ErrNothingToDo) {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDesign, webDesignJSON); err != nil {
		t.Fatal(err)
	}
	holdAutoPlan(t, s)
	if p = load(t, s); p.UI.TechVersion != 3 {
		t.Fatalf("design tech version = %d", p.UI.TechVersion)
	}
	// The platform change reached the requirements; the threat model that
	// refers to them is reviewed before finalize.
	if b := pipeline.Blockers(load(t, s)); !slices.Contains(b, "1 artefak perlu ditinjau karena artefak acuannya berubah") {
		t.Fatalf("blockers = %v", b)
	}
	if _, err := s.ReviewArtifact("threat-model"); err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(); !errors.Is(err, pipeline.ErrStackNotApproved) {
		t.Fatalf("approve design before stack err = %v", err)
	}
	if _, err := s.ApproveStack(); err != nil {
		t.Fatal(err)
	}
	// After approval the user still decides: Rust + Axum, a free choice.
	if _, err := propose(core.TechBackend, "Rust + Axum", "Tim backend memakai Rust"); err != nil {
		t.Fatal(err)
	}
	p, err = s.ApplyStackProposal()
	if err != nil {
		t.Fatal(err)
	}
	if p.Stack.State() != core.TechChanged || p.Stack.Decision(core.TechBackend).Lang() != "Rust" ||
		!slices.ContainsFunc(p.Stack.Issues, func(i core.TechIssue) bool { return i.Area == core.TechBackend && i.Level == "warning" }) {
		t.Fatalf("changed = %+v", p.Stack)
	}
	if rv := p.TechReview(); len(rv) != 1 || rv[0].From != "NestJS + TypeScript" || rv[0].To != "Rust + Axum" {
		t.Fatalf("review = %+v", rv)
	}
	if p.Blueprint.Backend.Background == "BullMQ" {
		t.Fatalf("blueprint kept the old backend: %+v", p.Blueprint.Backend)
	}
	if err := s.Finalize(); !errors.Is(err, pipeline.ErrStackNotApproved) {
		t.Fatalf("changed stack err = %v", err)
	}
	if _, err := s.ApproveStack(); err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(); !errors.Is(err, pipeline.ErrTechReview) {
		t.Fatalf("unreviewed design err = %v", err)
	}
	if _, err := s.AckTechReview(); err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(); !errors.Is(err, pipeline.ErrStructureNotApproved) {
		t.Fatalf("unapproved structure err = %v", err)
	}
	if _, err := s.ApproveStructure(); err != nil {
		t.Fatal(err)
	}
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(); err != nil {
		t.Fatal(err)
	}
	// The final spec carries the approved decision.
	raw, err := os.ReadFile(filepath.Join(s.Store.Dir, "docs", "foundry", "blueprint", "technology.md"))
	if err != nil || !strings.Contains(string(raw), "Vue + TypeScript + Vite") || !strings.Contains(string(raw), "Rust + Axum") ||
		!strings.Contains(string(raw), "Tim sudah pakai Vue") {
		t.Fatalf("spec = %s, %v", raw, err)
	}
	if _, err := propose(core.TechFrontend, "", ""); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("approved err = %v", err)
	}
}

// A change in words goes to the AI and comes back as a proposal; an answer
// with nothing to change leaves the decision as it was.
func TestStackRequest(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mcpAfterStory(t, s)
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.RequestStackChange("Ganti backend ke Rust"); !errors.Is(err, pipeline.ErrBusy) {
		t.Fatalf("request while design waits err = %v", err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	holdAutoPlan(t, s)
	if err := s.RequestStackChange("Ganti backend ke Rust. Tetap PostgreSQL."); err != nil {
		t.Fatal(err)
	}
	w, _ := s.NextWork()
	if w.Kind != work.KindStack || !strings.Contains(w.Prompt, "Ganti backend ke Rust. Tetap PostgreSQL.") {
		t.Fatalf("work = %+v", w)
	}
	if err := s.SubmitWork(work.KindStack, `{"changes":[{"area":"backend","value":"Rust + Axum","reason":"permintaan user"}],"note":"Backend diganti ke Rust."}`); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if pr := p.Stack.Proposal; pr == nil || pr.Source != "ai" || pr.Request == "" || p.Stack.Choice(core.TechBackend) != "NestJS + TypeScript" ||
		p.Stack.Choice(core.TechDatabase) != "PostgreSQL" || p.Stack.Request != "" {
		t.Fatalf("stack = %+v", p.Stack)
	}
	if _, err := s.DiscardStackProposal(); err != nil {
		t.Fatal(err)
	}
	if err := s.RequestStackChange("buat lebih bagus"); err != nil {
		t.Fatal(err)
	}
	s.SubmitWork(work.KindStack, `{"changes":[],"note":"tidak jelas"}`)
	p = load(t, s)
	mustJob(t, p, work.KindStack, core.JobFailed)
	if p.Stack.Proposal != nil || p.Stack.Request != "" || p.Stack.Version() != 1 {
		t.Fatalf("unclear request changed the stack: %+v", p.Stack)
	}
}

func TestStructureDecision(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mcpAfterStory(t, s)
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	holdAutoPlan(t, s)
	// Recommendation: web + mobile + backend sharing code is a monorepo;
	// domains are modules of one API, shared packages have a consumer.
	p := load(t, s)
	st := p.Structure
	ir := st.IR
	if ir == nil || ir.Type != core.StructMonorepo || st.State() != core.TechRecommended || st.Version() != 1 ||
		len(ir.Services) != 1 || ir.Services[0].Path != "apps/api/" || !slices.Contains(ir.Services[0].Modules, "identity") ||
		!slices.ContainsFunc(ir.Packages, func(x core.StructurePart) bool { return x.Name == "domain" }) {
		t.Fatalf("structure = %+v", ir)
	}
	domains := ir.Services[0].Modules
	other := domains[len(domains)-1]

	// Microservices: one service per domain, never per operation; the
	// architecture change it needs is shown before it is applied.
	p, err := s.ProposeStructure([]core.StructureOp{{Op: "type", Name: "microservices"}})
	if err != nil {
		t.Fatal(err)
	}
	pr := p.Structure.Proposal
	if pr == nil || p.Structure.IR.Type != core.StructMonorepo || len(pr.Tech) != 1 || pr.Tech[0].To != core.ArchMicroservices ||
		len(pr.IR.Services) != len(domains) || !slices.ContainsFunc(pr.Impact, func(i core.StructureImpact) bool { return i.Area == "Deployment" }) ||
		!slices.ContainsFunc(pr.Issues, func(i core.TechIssue) bool { return strings.Contains(i.Message, "Modular biasanya cukup") }) {
		t.Fatalf("proposal = %+v", pr)
	}
	if p.Stack.Choice(core.TechArchitecture) == core.ArchMicroservices {
		t.Fatal("technology changed before apply")
	}
	if _, err := s.ApproveStructure(); !errors.Is(err, structure.ErrPending) {
		t.Fatalf("approve with proposal err = %v", err)
	}
	p, err = s.ApplyStructureProposal()
	if err != nil {
		t.Fatal(err)
	}
	if p.Structure.Version() != 2 || p.Structure.State() != core.TechImpactChecked || p.Stack.Choice(core.TechArchitecture) != core.ArchMicroservices ||
		!slices.Contains(p.Structure.Layout, "services/"+other+"/") || len(p.Structure.Errors()) != 0 {
		t.Fatalf("applied = %+v %+v", p.Structure, p.Structure.Issues)
	}
	// "Reporting jangan jadi service sendiri, jadikan modul di identity."
	if _, err := s.ProposeStructure([]core.StructureOp{{Op: "move", Kind: "service", Name: other, To: "identity"}}); err != nil {
		t.Fatal(err)
	}
	p, err = s.ApplyStructureProposal()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(p.Structure.Layout, "services/identity/"+other+"/") || slices.Contains(p.Structure.Layout, "services/"+other+"/") {
		t.Fatalf("layout = %v", p.Structure.Layout)
	}
	// A CRUD boundary is flagged.
	p, err = s.ProposeStructure([]core.StructureOp{{Op: "add", Kind: "service", Name: "create-booking"}})
	if err != nil || !slices.ContainsFunc(p.Structure.Proposal.Issues, func(i core.TechIssue) bool { return strings.Contains(i.Message, "CRUD") }) {
		t.Fatalf("crud: %v %+v", err, p.Structure.Proposal)
	}
	if _, err := s.DiscardStructureProposal(); err != nil {
		t.Fatal(err)
	}
	// The technology moves away from the user's structure: the structure
	// is not rewritten, it is flagged and cannot be approved.
	if _, err := s.ProposeStack([]stack.Change{{Area: core.TechArchitecture, Value: core.ArchModular}}); err != nil {
		t.Fatal(err)
	}
	p, err = s.ApplyStackProposal()
	if err != nil {
		t.Fatal(err)
	}
	if p.Structure.IR.Type != core.StructMicroservices || len(p.Structure.Errors()) == 0 {
		t.Fatalf("structure after tech change = %+v %+v", p.Structure.IR, p.Structure.Issues)
	}
	if _, err := s.ApproveStructure(); err == nil {
		t.Fatal("inconsistent structure approved")
	}
	// Natural language through the AI host becomes a proposal.
	if err := s.RequestStructureChange("Jadikan modular saja, backend satu."); err != nil {
		t.Fatal(err)
	}
	w, _ := s.NextWork()
	if w.Kind != work.KindStructure || !strings.Contains(w.Prompt, "Jadikan modular saja") || !strings.Contains(w.Prompt, "service identity (modules: identity, "+other) {
		t.Fatalf("work = %+v", w)
	}
	if err := s.SubmitWork(work.KindStructure, `{"ops":[{"op":"type","name":"modular"}],"note":"Diubah ke modular."}`); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if pr := p.Structure.Proposal; pr == nil || pr.Source != "ai" || pr.IR.Type != core.StructModular || p.Structure.Request != "" {
		t.Fatalf("ai proposal = %+v", p.Structure)
	}
	if p, err = s.ApplyStructureProposal(); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(p.Structure.Layout, "apps/api/modules/"+other+"/") || len(p.Structure.Errors()) != 0 {
		t.Fatalf("modular = %v %+v", p.Structure.Layout, p.Structure.Issues)
	}
	if p, err = s.ApproveStructure(); err != nil || p.Structure.State() != core.TechApproved || p.Structure.Version() != 4 {
		t.Fatalf("approve: %v %+v", err, p.Structure)
	}
	// The final spec follows the approved structure.
	if err := approve(s); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(s.Store.Dir, "docs", "foundry", "blueprint", "technology.md"))
	if err != nil || !strings.Contains(string(raw), "Tipe: modular") || !strings.Contains(string(raw), "apps/api") {
		t.Fatalf("spec = %s, %v", raw, err)
	}
}

// approve approves the technology decision and structure, then the design.
func approve(s *pipeline.Service) error {
	if _, err := s.ApproveStack(); err != nil && !errors.Is(err, pipeline.ErrWrongStage) {
		return err
	}
	if _, err := s.ApproveStructure(); err != nil && !errors.Is(err, pipeline.ErrWrongStage) && !errors.Is(err, structure.ErrExisting) {
		return err
	}
	if err := workflowReady(s); err != nil {
		return err
	}
	return s.Finalize()
}

// workflowReady writes the command workflow when there is none, with the
// agent or (MCP mode) by submitting the fake answer.
func workflowReady(s *pipeline.Service) error {
	p, err := s.Store.Load()
	if err != nil {
		return err
	}
	if p.Workflow != nil && len(p.Workflow.Commands) > 0 {
		return nil
	}
	if err := s.GenerateWorkflow(); err != nil {
		return err
	}
	s.Wait()
	if p, _ = s.Store.Load(); p.Job != nil && p.Job.Status == core.JobWaiting {
		return submitWorkflow(s, workflowJSON)
	}
	if p.Workflow == nil || len(p.Workflow.Commands) == 0 {
		return fmt.Errorf("workflow not written: job %+v", p.Job)
	}
	return nil
}
