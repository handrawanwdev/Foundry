package pipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
	"github.com/handrawanw/foundry/internal/modules/pipeline"
	"github.com/handrawanw/foundry/internal/modules/spec"
	"github.com/handrawanw/foundry/internal/modules/stack"
	"github.com/handrawanw/foundry/internal/modules/structure"
	"github.com/handrawanw/foundry/internal/modules/work"
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

const discoveryJSON = `Here you go:
` + "```json" + `
{"name": "Klinik Sehat",
 "discovery": {"summary": "Aplikasi klinik", "business": "Klinik pratama", "users": ["Pasien", "Admin"],
   "roles": [{"name": "Pasien", "description": "membuat janji", "platform": "android"}, {"name": "Admin", "description": "kelola jadwal", "platform": "web"}],
   "data": ["Appointment: id, pasien, dokter, waktu"], "integrations": [], "security": ["Login wajib"], "workflows": ["Pasien membuat janji"]},
 "requirements": [{"id": "R1", "text": "Pasien dapat membuat janji temu", "priority": "MUST"},
                  {"id": "R2", "text": "Admin dapat mengelola jadwal dokter", "priority": "MUST"}],
 "features": [{"id": "F1", "name": "Janji Temu", "description": "Pasien membuat janji", "requirement_ids": ["R1"], "platforms": ["android", "backend"],
               "sub_features": [{"id": "x", "name": "Cari dokter"}, {"id": "y", "name": "Buat janji"}]},
              {"id": "F2", "name": "Jadwal Dokter", "description": "Admin kelola jadwal", "requirement_ids": ["R2"], "platforms": ["web", "backend"],
               "sub_features": [{"id": "z", "name": "Tambah jadwal"}]}],
 "flows": [{"id": "FL1", "name": "Buat janji", "feature_id": "F1", "role": "Pasien", "steps": ["Pilih dokter", "Pilih jadwal", "Konfirmasi"]}],
 "platform": {"targets": [{"kind": "web", "audience": ["Admin"], "rationale": "dashboard"}, {"kind": "android", "audience": ["Pasien"], "rationale": "HP"},
                          {"kind": "backend", "audience": [], "rationale": "API"}], "reason": "Pasien memakai HP, admin memakai web."}}
` + "```"

const designJSON = `{"system": {"colors": [{"name": "primary", "value": "#1f6feb"}, {"name": "", "value": "#fff"}],
   "typography": [{"name": "body", "value": "Inter 14px"}], "spacing": [{"name": "space-2", "value": "8px"}]},
 "pages": [
  {"id": "P1", "name": "Daftar Dokter", "platform": "android", "role": "Pasien", "feature_id": "FEAT-001", "sub_feature_ids": ["FEAT-001.1"], "purpose": "Cari dokter",
   "hierarchy": ["Daftar dokter"], "secondary_actions": ["pilih", "Filter", " "], "states": {"loading": "Memuat dokter", "error": "Gagal memuat. Coba lagi."},
   "route": "Dokter", "access": "AUTH", "primary_action": {"label": "Pilih", "target": "P2"}, "data": {"required": ["Nama dokter"], "api": ["get /doctors", "ambil dokter"]},
   "components": [{"type": "header", "label": "Dokter"}, {"type": "list", "label": "Dokter", "columns": ["x"], "items": ["dr. A", "dr. B"]}, {"type": "button", "label": "Pilih", "target": "P2"}]},
  {"id": "P2", "name": "Konfirmasi Janji", "platform": "android", "role": "Pasien", "feature_id": "FEAT-001", "sub_feature_ids": ["FEAT-001.2"], "purpose": "Konfirmasi", "kind": "form",
   "hierarchy": ["Ringkasan janji", "Form"], "route": "/janji/konfirmasi", "access": "auth", "primary_action": {"label": "Kirim", "target": "P9"},
   "components": [{"type": "form", "label": "Janji", "items": ["x"], "fields": [{"label": "Tanggal", "type": "DATE", "required": true, "group": "Jadwal"}, {"label": " "}]}, {"type": "button", "label": "Ke halaman hilang", "target": "P9"}]},
  {"id": "P3", "name": "Admin Jadwal", "platform": "web", "role": "Admin", "feature_id": "FEAT-002", "sub_feature_ids": ["FEAT-002.1"], "purpose": "Kelola jadwal",
   "kind": "ADMIN", "density": "padat", "hierarchy": ["Tabel jadwal"], "states": {"loading": "Memuat jadwal", "empty": "Belum ada jadwal. Tambahkan jadwal dokter.", "error": "Jadwal gagal dimuat. Muat ulang."}, "route": "/jadwal", "access": "PUBLIC", "primary_action": {"reason": "Halaman baca saja"},
   "components": [{"type": "table", "label": "Jadwal", "columns": ["Hari"], "items": ["Senin"]}]}],
 "flows": [{"id": "FLOW-001", "page_ids": ["P1", "P2"]}]}`

// revisedDesignJSON is a patch: one changed component, one new page, a
// longer flow. Everything else stays as it is.
const revisedDesignJSON = `{"patches": [{"page": "PAGE-003", "operations": [
   {"op": "update", "target": 0, "component": {"type": "table", "label": "Jadwal", "columns": ["Hari"], "items": ["Senin", "Selasa"]}}]}],
 "pages": [
  {"id": "NEW-1", "name": "Profil", "platform": "android", "role": "Pasien", "feature_id": "FEAT-001", "purpose": "Data diri",
   "components": [{"type": "form", "label": "Profil", "items": ["Nama"]}, {"type": "button", "label": "Dokter", "target": "PAGE-001"}]}],
 "flows": [{"id": "FLOW-001", "page_ids": ["PAGE-001", "PAGE-002", "NEW-1"]}]}`

// fullPageJSON sends an existing page whole: a revision must reject it.
const fullPageJSON = `{"pages": [{"id": "PAGE-003", "name": "Admin Jadwal", "platform": "web", "role": "Admin", "feature_id": "FEAT-002",
   "components": [{"type": "table", "label": "Jadwal"}]}]}`

const movePatchJSON = `{"patches": [{"page": "PAGE-001", "operations": [{"op": "move", "target": 2, "after": -1}]}]}`

func fakeAgent() {
	in, _ := io.ReadAll(os.Stdin)
	prompt := string(in)
	mode := os.Getenv("FAKE_MODE")
	switch {
	case strings.HasPrefix(prompt, "Balas hanya"):
		fmt.Println("OK")
	case strings.Contains(prompt, "discovery analyst"):
		countRun("discovery")
		if mode == "bad_json" {
			fmt.Println("maaf, saya tidak bisa")
			return
		}
		fmt.Println(discoveryJSON)
	case strings.Contains(prompt, "design analyst"):
		if mode == "ref_error" {
			fmt.Println(`{"error": "hanya halaman login"}`)
			return
		}
		if !strings.Contains(prompt, "voiceflow.com") || mode == "ref_shots" && !strings.Contains(prompt, "Voiceflow-desktop.png") {
			fmt.Fprintln(os.Stderr, "reference missing in prompt")
			os.Exit(4)
		}
		fmt.Println(referenceJSON)
	case strings.Contains(prompt, "product designer"):
		if strings.Contains(prompt, "Tambahkan halaman profil") {
			fmt.Println(revisedDesignJSON)
			return
		}
		if strings.Contains(prompt, "Pindahkan tombol") {
			if mode == "reject_once" && !strings.Contains(prompt, "YOUR PREVIOUS PATCH WAS REJECTED") || mode == "reject_always" {
				fmt.Println(fullPageJSON)
				return
			}
			fmt.Println(movePatchJSON)
			return
		}
		if mode == "ir_invalid_once" && !strings.Contains(prompt, "YOUR PREVIOUS DESIGN WAS REJECTED") {
			fmt.Println(strings.Replace(designJSON, `"kind": "ADMIN",`, `"kind": "ADMIN", "layout": {"frame": "app", "root": {"type": "stack", "children": ["ghost"]}},`, 1))
			return
		}
		fmt.Println(designJSON)
	case strings.Contains(prompt, "visual QA checker"):
		if mode == "initial_fix" && strings.Contains(prompt, "A new design was just generated") && strings.Contains(prompt, "-i1.png") {
			fmt.Println(`{"status": "fix", "notes": "aksi utama tenggelam", "issues": [
  {"page": "PAGE-001", "device": "mobile", "severity": "high", "category": "hierarchy", "component": "button-3", "problem": "Tombol Pilih ada di bawah daftar panjang", "fix": "Pindahkan tombol Pilih ke atas"},
  {"page": "PAGE-002", "severity": "low", "category": "spacing", "problem": "Make it cleaner", "fix": "improve the UI"}],
 "patches": [{"page": "PAGE-001", "operations": [{"op": "move", "target": 2, "after": -1}]}]}`)
			return
		}
		if mode == "degrade_fix" && strings.Contains(prompt, "-i1.png") {
			// Removing the primary button lowers the quality score.
			fmt.Println(`{"status": "fix", "notes": "tombol mengganggu", "patches": [{"page": "PAGE-001", "operations": [{"op": "remove", "target": 2}]}]}`)
			return
		}
		if mode == "qa_fix" && strings.Contains(prompt, "-r1.png") {
			fmt.Println(`{"status": "fix", "notes": "tombol terlalu atas", "patches": [{"page": "PAGE-001", "operations": [{"op": "move", "target": 0, "after": 1}]}]}`)
			return
		}
		fmt.Println(`{"status": "pass"}`)
	default:
		fmt.Fprintln(os.Stderr, "unknown prompt")
		os.Exit(3)
	}
}

// countRun appends label to $FAKE_RUNS, so a test can count agent runs.
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

// designReady drives Tell → Discovery → Design.
func designReady(t *testing.T, s *pipeline.Service) *core.Project {
	t.Helper()
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindDesign, core.JobDone)
	return p
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
	// Design traces to features; dead link P9 removed; flow linked to pages.
	if len(p.Design.Pages) != 3 || p.Design.Pages[1].Components[1].Target != "" {
		t.Fatalf("design pages = %+v", p.Design.Pages)
	}
	if p.Design.Pages[0].Components[2].Target != "PAGE-002" {
		t.Fatalf("prototype link = %q", p.Design.Pages[0].Components[2].Target)
	}
	if got := p.Flows[0].PageIDs; len(got) != 2 {
		t.Fatalf("flow pages = %v", got)
	}
	// Contract fields are normalized: route gets a slash, access and field
	// types are lowercased, invalid api calls and empty fields are dropped,
	// form items follow the field spec, dead action targets are cleared.
	pg := p.Design.Pages
	if pg[0].Route != "/dokter" || pg[0].Access != "auth" || pg[2].Access != "public" ||
		!slices.Equal(pg[0].Data.API, []string{"GET /doctors"}) || pg[0].PrimaryAction.Target != "PAGE-002" || pg[1].PrimaryAction.Target != "" {
		t.Fatalf("contract = %+v", pg)
	}
	form := pg[1].Components[0]
	if len(form.Fields) != 1 || form.Fields[0].Type != "date" || !slices.Equal(form.Items, []string{"# Jadwal", "Tanggal"}) {
		t.Fatalf("form = %+v", form)
	}
	if b := work.Blockers(p); len(b) != 0 {
		t.Fatalf("blockers = %+v", b)
	}
	h := work.PageSpecs(p)[0]
	if h.Route != "/dokter" || h.PrimaryAction != "Pilih → Konfirmasi Janji (/janji/konfirmasi)" ||
		!slices.Contains(h.Tests, "Tanpa login diarahkan ke halaman masuk") || !slices.Contains(work.PageSpecs(p)[1].Validation, "Tanggal: wajib, date") {
		t.Fatalf("handoff = %+v", h)
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
	if p.Stage != core.StageReady || p.Design.Status != core.DesignApproved || p.Job != nil && p.Job.Status == core.JobRunning {
		t.Fatalf("finalize: stage=%s design=%s job=%+v", p.Stage, p.Design.Status, p.Job)
	}
	// The final spec lands in docs/ for an external implementation tool.
	raw, err := os.ReadFile(filepath.Join(p.Dir, "docs", "spec.json"))
	if err != nil {
		t.Fatalf("spec.json missing: %v", err)
	}
	var out spec.Spec
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Pages) != 3 || out.Pages[0].Route != "/dokter" {
		t.Fatalf("spec.json = %s, %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "docs", "pages.md")); err != nil {
		t.Fatalf("pages.md missing: %v", err)
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
	if err := s.ReviseDesign("ubah warna", nil, nil); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("revise after finalize err = %v", err)
	}
	if err := s.ReopenDesign(); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if p.Stage != core.StageDesign || p.Design.Status != core.DesignDraft {
		t.Fatalf("reopen: stage=%s design=%s", p.Stage, p.Design.Status)
	}
	if err := s.ReopenDesign(); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("reopen twice err = %v", err)
	}
}

func TestNoAIConnected(t *testing.T) {
	s := newService(t, fakeAI{none: true}, "")
	if _, err := s.SubmitStory("Aplikasi laundry untuk pelanggan"); !errors.Is(err, pipeline.ErrNoAI) {
		t.Fatalf("err = %v", err)
	}
	mustJob(t, load(t, s), work.KindDiscovery, core.JobFailed)
}

func TestInvalidDiscoveryOutput(t *testing.T) {
	s := newService(t, fakeAI{}, "bad_json")
	s.SubmitStory("Aplikasi keuangan pribadi")
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
	p := load(t, s)
	b := p.Blueprint
	if b == nil || b.Module("identity") == nil || b.Database.Engine == "" || b.Auth.Hashing == "" {
		t.Fatalf("blueprint = %+v", b)
	}
	if !strings.Contains(strings.Join(p.Structure.Layout, "\n"), "identity") && p.Stack.Choice(core.TechArchitecture) == core.ArchModular {
		t.Fatalf("layout missing identity module: %v", p.Structure.Layout)
	}
}

func TestDesignRevise(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := s.ReviseDesign("Tambahkan halaman profil", nil, nil); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	if p.Design.Status != core.DesignDraft || len(p.Design.Feedback) != 1 {
		t.Fatalf("design = %+v", p.Design)
	}
	rev := p.Design.Revision
	if rev == nil || rev.Request != "Tambahkan halaman profil" || len(rev.Changes) != 2 {
		t.Fatalf("revision = %+v", rev)
	}
	if c := rev.Changes[0]; c.Kind != "changed" || c.Page != "Admin Jadwal" || len(c.Details) != 1 {
		t.Fatalf("change 0 = %+v", c)
	}
	if c := rev.Changes[1]; c.Kind != "added" || c.Page != "Profil" || c.PageID != "PAGE-004" {
		t.Fatalf("change 1 = %+v", c)
	}
	// Untouched pages keep id and content; the new page links to them.
	if pg := findPage(p, "PAGE-002"); pg == nil || pg.Route != "/janji/konfirmasi" || len(pg.Components[0].Fields) != 1 {
		t.Fatalf("untouched page = %+v", pg)
	}
	if got := findPage(p, "PAGE-004").Components[1].Target; got != "PAGE-001" {
		t.Fatalf("new page target = %q", got)
	}
	if got := p.Flows[0].PageIDs; !slices.Equal(got, []string{"PAGE-001", "PAGE-002", "PAGE-004"}) {
		t.Fatalf("flow pages = %v", got)
	}

	if err := s.UndoDesignChange(); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if len(p.Design.Pages) != 3 || len(p.Design.Feedback) != 0 || p.Design.Revision != nil {
		t.Fatalf("after undo: %+v", p.Design)
	}
	if got := p.Flows[0].PageIDs; len(got) != 2 {
		t.Fatalf("flow pages after undo = %v", got)
	}
	if err := s.KeepDesignChange(); err == nil {
		t.Fatal("keep without a revision must fail")
	}

	if err := s.ReviseDesign("Tambahkan halaman profil", nil, nil); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.KeepDesignChange(); err != nil {
		t.Fatal(err)
	}
	p = load(t, s)
	if len(p.Design.Pages) != 4 || len(p.Design.Feedback) != 1 || p.Design.Revision != nil {
		t.Fatalf("after keep: %+v", p.Design)
	}
	// The new profile page has no route, access or primary action.
	if err := approve(s); !errors.Is(err, pipeline.ErrIncomplete) {
		t.Fatalf("approve incomplete err = %v", err)
	}
}

func findPage(p *core.Project, id string) *core.Page {
	for i := range p.Design.Pages {
		if p.Design.Pages[i].ID == id {
			return &p.Design.Pages[i]
		}
	}
	return nil
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
	// The existing frontend is read: the design follows its tokens and the
	// prompts know its routes, navigation and components.
	if ui := p.Structure.UI; ui == nil || ui.Navigation != "side" || len(ui.Routes) != 2 || ui.ThemeID != "existing-ui" {
		t.Fatalf("ui = %+v", p.Structure.UI)
	}
	if p.Theme != "existing-ui" || p.Design.Theme == nil || p.Design.Theme.ID != "existing-ui" {
		t.Fatalf("theme = %q %+v", p.Theme, p.Design.Theme)
	}
	if prompt := work.DesignPrompt(p); !strings.Contains(prompt, "EXISTING UI") || !strings.Contains(prompt, "/orders, /settings") || !strings.Contains(prompt, "Pesanan") {
		t.Fatalf("design prompt misses the existing ui")
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
}

func TestMCPHostFlow(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mustJob(t, load(t, s), work.KindDiscovery, core.JobWaiting)

	w, _ := s.NextWork()
	if w.Kind != work.KindDiscovery || !strings.Contains(w.Prompt, "klinik") {
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
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	// Kind is normalized, unknown density is dropped, columns only stay on
	// tables, secondary actions drop blanks and the primary action.
	d := load(t, s).Design
	pg := d.Pages
	if pg[2].Kind != "admin" || pg[2].Density != "" || pg[0].Components[1].Columns != nil || !slices.Equal(pg[0].SecondaryActions, []string{"Filter"}) {
		t.Fatalf("pages = %+v", pg)
	}
	// The theme gives the tokens; the designer's component rules stay.
	if d.Theme == nil || d.Theme.Source != "auto" || d.System == nil || len(d.System.Colors) != len(core.ThemeColorTokens) ||
		!strings.HasPrefix(d.System.Direction, d.Theme.Name) {
		t.Fatalf("theme = %+v system = %+v", d.Theme, d.System)
	}
	if err := approve(s); err != nil {
		t.Fatal(err)
	}
	if p := load(t, s); p.Stage != core.StageReady {
		t.Fatalf("stage = %s", p.Stage)
	}
}

type fakeRender struct{ fail bool }

func (f fakeRender) Capture(_ context.Context, url, device, out string) error {
	if f.fail {
		return errors.New("chrome tidak ada")
	}
	return nil
}

func (f fakeRender) Shot(_ context.Context, _, pageID, device, out string) (string, error) {
	if f.fail {
		return "", errors.New("chrome tidak ada")
	}
	return "Tampilan " + device + ":\n[0] header: Judul", nil
}

func types(pg *core.Page) string {
	var out []string
	for _, c := range pg.Components {
		out = append(out, c.Type)
	}
	return strings.Join(out, ",")
}

func scopePage1() *core.ChangeScope {
	return &core.ChangeScope{Pages: []core.ScopePage{{ID: "PAGE-001", Request: "Pindahkan tombol ke atas", Components: []int{2}, Device: "mobile"}}}
}

func TestDesignPatchRejectedRetriesOnce(t *testing.T) {
	s := newService(t, fakeAI{}, "reject_once")
	designReady(t, s)
	if err := s.ReviseDesign("Pindahkan tombol ke atas", nil, scopePage1()); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindDesign, core.JobDone)
	if got := types(findPage(p, "PAGE-001")); got != "button,header,list" {
		t.Fatalf("components = %s", got)
	}
	if !strings.Contains(p.Design.Rejected, "PATCH DITOLAK") && p.Design.Revision == nil {
		t.Fatalf("design = %+v", p.Design)
	}

	s = newService(t, fakeAI{}, "reject_always")
	designReady(t, s)
	if err := s.ReviseDesign("Pindahkan tombol ke atas", nil, scopePage1()); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	mustJob(t, p, work.KindDesign, core.JobFailed)
	if !strings.Contains(p.Job.Error, "PAGE-003 dikirim utuh") || p.Design.Revision != nil || types(findPage(p, "PAGE-001")) != "header,list,button" {
		t.Fatalf("rejected patch applied: %+v / %s", p.Design, p.Job.Error)
	}
}

// An invalid Design IR is rejected, not rendered; the designer gets the
// errors and one retry.
func TestDesignIRInvalidRetriesOnce(t *testing.T) {
	s := newService(t, fakeAI{}, "ir_invalid_once")
	p := designReady(t, s)
	if len(p.Design.Pages) != 3 || p.Design.Rejected != "" {
		t.Fatalf("design = %+v", p.Design)
	}
	for _, pg := range p.Design.Pages {
		if pg.Layout == nil {
			t.Fatalf("page %s without layout", pg.ID)
		}
	}
	if !strings.Contains(strings.Join(logTexts(p), "\n"), "DESIGN_IR_INVALID") {
		t.Fatal("rejection not logged")
	}
}

func logTexts(p *core.Project) []string {
	var out []string
	for _, e := range p.Events {
		out = append(out, e.Message)
	}
	return out
}

func TestDesignVisualQA(t *testing.T) {
	s := newService(t, fakeAI{}, "qa_fix")
	designReady(t, s)
	s.Render = fakeRender{}
	if err := s.ReviseDesign("Pindahkan tombol ke atas", nil, scopePage1()); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindDesignQA, core.JobDone)
	qa := p.Design.Revision.QA
	if qa.Status != "fixed" || qa.Round != 2 || qa.Notes != "tombol terlalu atas" || len(qa.Shots) != 1 || qa.Shots[0].Device != "mobile" {
		t.Fatalf("qa = %+v", qa)
	}
	if got := types(findPage(p, "PAGE-001")); got != "header,button,list" {
		t.Fatalf("components after QA fix = %s", got)
	}
	if err := s.UndoDesignChange(); err != nil {
		t.Fatal(err)
	}
	if got := types(findPage(load(t, s), "PAGE-001")); got != "header,list,button" {
		t.Fatalf("undo must restore the design from before the request: %s", got)
	}

	s = newService(t, fakeAI{}, "")
	designReady(t, s)
	s.Render = fakeRender{fail: true}
	if err := s.ReviseDesign("Pindahkan tombol ke atas", nil, scopePage1()); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	if qa := p.Design.Revision.QA; qa.Status != "skipped" || !strings.Contains(qa.Notes, "chrome tidak ada") {
		t.Fatalf("qa = %+v", qa)
	}
	if types(findPage(p, "PAGE-001")) != "button,header,list" {
		t.Fatal("a skipped check keeps the applied patch")
	}
}

// A new design is critiqued before the user sees it: every page on its base
// device plus the mobile view of web pages; a fix is an IR patch, the next
// round renders only the patched page on the device of its finding.
func TestInitialDesignCritic(t *testing.T) {
	s := newService(t, fakeAI{}, "initial_fix")
	s.Render = fakeRender{}
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindDesignQA, core.JobDone)
	qa := p.Design.QA
	if qa == nil || qa.Status != "fixed" || qa.Round != 2 || qa.Notes != "aksi utama tenggelam" {
		t.Fatalf("qa = %+v", qa)
	}
	if len(qa.Shots) != 1 || qa.Shots[0].PageID != "PAGE-001" || qa.Shots[0].Device != "mobile" {
		t.Fatalf("second round must render only the patched page: %+v", qa.Shots)
	}
	if got := types(findPage(p, "PAGE-001")); got != "button,header,list" {
		t.Fatalf("components after critic fix = %s", got)
	}
	if p.Design.Revision != nil {
		t.Fatal("the critic fix of a new design is not a user revision")
	}
	// Measured after generation, after the patch and after the last round.
	q := p.Design.Quality
	if q == nil || len(q.Pages) != 3 || q.Confidence == "low" || q.Compared == nil || q.Compared.Verdict == "degraded" || len(qa.Compared) != 1 {
		t.Fatalf("quality = %+v compared = %+v", q, qa.Compared)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, core.StateDir, "design-quality.json")); err != nil {
		t.Fatal(err)
	}
	var hist []work.QualityRecord
	b, _ := os.ReadFile(filepath.Join(p.Dir, core.StateDir, "design-quality-history.json"))
	if err := json.Unmarshal(b, &hist); err != nil || len(hist) != 9 || hist[0].Round != 0 ||
		hist[6].PageID != "PAGE-001" || hist[6].Round != 2 || !slices.Equal(hist[6].Devices, []string{"mobile"}) || len(hist[8].Devices) != 0 {
		t.Fatalf("history (%v) = %+v", err, hist)
	}

	// Round 1 checked 3 pages on their base device + the web page on mobile.
	s = newService(t, fakeAI{}, "initial_fix")
	s.Render = &countRender{}
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	shots := s.Render.(*countRender).shots
	want := []string{"PAGE-001 mobile", "PAGE-002 mobile", "PAGE-003 desktop", "PAGE-003 mobile", "PAGE-001 mobile"}
	if strings.Join(shots, ",") != strings.Join(want, ",") {
		t.Fatalf("shots = %v", shots)
	}
}

// A critic patch that lowers the quality score is rejected; the design the
// generator made stays.
func TestCriticPatchDegradesRejected(t *testing.T) {
	s := newService(t, fakeAI{}, "degrade_fix")
	s.Render = fakeRender{}
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	qa := p.Design.QA
	if qa.Status != "failed" || !strings.Contains(qa.Notes, "memperburuk kualitas") || len(qa.Compared) != 1 || !qa.Compared[0].Rejected {
		t.Fatalf("qa = %+v", qa)
	}
	if got := types(findPage(p, "PAGE-001")); got != "header,list,button" {
		t.Fatalf("components = %s", got)
	}
	if p.Design.Quality.Compared == nil || p.Design.Quality.Compared.Verdict != "degraded" {
		t.Fatalf("quality = %+v", p.Design.Quality)
	}
}

type countRender struct{ shots []string }

func (c *countRender) Shot(_ context.Context, _, pageID, device, _ string) (string, error) {
	c.shots = append(c.shots, pageID+" "+device)
	return "Tampilan " + device, nil
}

// A theme picked before the design is followed by it; picking another
// re-themes the draft without touching its pages.
func TestSetTheme(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	if _, err := s.SetTheme("nope"); err == nil || !strings.Contains(err.Error(), "tidak tersedia") {
		t.Fatalf("unknown theme err = %v", err)
	}
	if _, err := s.SetTheme("finance-minimal"); err != nil {
		t.Fatal(err)
	}
	p := designReady(t, s)
	if th := p.Design.Theme; th == nil || th.ID != "finance-minimal" || th.Source != "user" || p.Design.System.Colors[6].Value != "#0f766e" {
		t.Fatalf("theme = %+v", th)
	}
	pages, _ := json.Marshal(p.Design.Pages)
	p, err := s.SetTheme("trading-terminal")
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(p.Design.Pages)
	if string(after) != string(pages) || p.Design.Theme.ID != "trading-terminal" || p.Design.Theme.ColorMode != "dark" || p.Design.Quality == nil {
		t.Fatalf("re-theme: theme = %+v", p.Design.Theme)
	}
	// Back to automatic: selected from the detected domain.
	if p, err = s.SetTheme(""); err != nil || p.Theme != "" || p.Design.Theme.ID != "enterprise-dark" || p.Design.Theme.Source != "auto" {
		t.Fatalf("auto: err = %v theme = %+v", err, p.Design.Theme)
	}
	finalize(t, s)
	if _, err := s.SetTheme("trading-terminal"); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("approved design re-themed: %v", err)
	}
}

const referenceJSON = `{"reference": "Voiceflow", "category": "editor", "personality": ["modern", "creative", "technical"],
 "layout": ["canvas", "split-panel"], "navigation": "side", "density": "comfortable",
 "visual": {"color_mode": "light", "spacing": "comfortable", "surface": "soft", "radius": "medium"},
 "typography": {"scale": "balanced", "personality": "geometric"},
 "colors": {"primary": "#3d5afe", "background": "#f7f7f8", "surface": "#ffffff", "text": "#1a1a1a", "text_muted": "#5f6368", "border": "#e4e4e7",
            "success": "#16a34a", "warning": "#b45309", "danger": "#dc2626"},
 "components": {"button": "soft", "nav": "light"}, "avoid": ["hero_section"]}`

// A reference named before the story is analyzed after discovery; the
// first design follows its theme.
func TestReferenceBeforeDesign(t *testing.T) {
	s := newService(t, fakeAI{}, "ref_shots")
	s.Render = fakeRender{}
	if _, err := s.SetReference("ftp://x", ""); err == nil {
		t.Fatal("invalid reference accepted")
	}
	p, err := s.SetReference("voiceflow.com", "suka kanvasnya")
	if err != nil || p.Reference.Status != "pending" || p.Job != nil {
		t.Fatalf("err = %v ref = %+v job = %+v", err, p.Reference, p.Job)
	}
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	r := p.Reference
	if r.Status != "done" || r.ThemeID != "voiceflow-inspired" || len(r.Shots) != 2 || p.Theme != r.ThemeID {
		t.Fatalf("reference = %+v", r)
	}
	if th := p.Design.Theme; th == nil || th.ID != "voiceflow-inspired" || th.Source != "user" || p.Design.Quality.Dimensions["reference"] == 0 {
		t.Fatalf("design theme = %+v", th)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, core.StateDir, "themes", "voiceflow-inspired", "reference.json")); err != nil {
		t.Fatal(err)
	}
	// Removing the reference goes back to automatic selection.
	if p, err = s.SetReference("", ""); err != nil || p.Reference != nil || p.Theme != "" {
		t.Fatalf("remove: %v %+v", err, p.Reference)
	}
}

// A reference that cannot be analyzed fails; the design falls back to the
// theme selected from the requirement.
func TestReferenceInvalidFallsBack(t *testing.T) {
	s := newService(t, fakeAI{}, "ref_error")
	s.SetReference("voiceflow.com", "")
	p := designReady(t, s)
	if p.Reference.Status != "failed" || !strings.Contains(p.Reference.Error, "login") || len(p.Reference.Shots) != 0 {
		t.Fatalf("reference = %+v", p.Reference)
	}
	if p.Design.Theme == nil || p.Design.Theme.Source != "auto" {
		t.Fatalf("theme = %+v", p.Design.Theme)
	}
}

// A reference on an existing draft re-themes it; the pages stay.
func TestReferenceOnDraft(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	p := designReady(t, s)
	pages, _ := json.Marshal(p.Design.Pages)
	if _, err := s.SetReference("https://voiceflow.com", ""); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	after, _ := json.Marshal(p.Design.Pages)
	mustJob(t, p, work.KindReference, core.JobDone)
	if string(after) != string(pages) || p.Design.Theme.ID != "voiceflow-inspired" || p.Design.System.Colors[6].Value != "#3d5afe" {
		t.Fatalf("theme = %+v", p.Design.Theme)
	}
}

func TestDesignVersionsAndCanvas(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	if err := s.ReviseDesign("Tambahkan halaman profil", nil, nil); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if _, err := s.EditDesign("PAGE-001", []work.CanvasOp{{Op: "set_page", Fields: map[string]string{"density": "compact"}}}); err == nil {
		t.Fatal("canvas edit must wait for the pending revision")
	}
	if err := s.UndoDesignChange(); err != nil {
		t.Fatal(err)
	}
	res, err := s.EditDesign("PAGE-001", []work.CanvasOp{{Op: "set_page", Fields: map[string]string{"density": "compact"}}})
	if err != nil {
		t.Fatal(err)
	}
	p := res.Project
	if findPage(p, "PAGE-001").Density != "compact" || p.Design.Quality == nil {
		t.Fatalf("canvas edit = %+v", findPage(p, "PAGE-001"))
	}
	list, err := work.Versions(p)
	if err != nil {
		t.Fatal(err)
	}
	var sources []string
	for _, v := range list {
		sources = append(sources, v.Source)
	}
	if !slices.Equal(sources, []string{"canvas", "undo", "revision", "generate"}) || !list[0].Current {
		t.Fatalf("versions = %+v", list)
	}

	p, err = s.RestoreDesignVersion(list[3].ID)
	if err != nil {
		t.Fatal(err)
	}
	if findPage(p, "PAGE-001").Density == "compact" || len(p.Design.Pages) != 3 {
		t.Fatalf("restore = %+v", p.Design.Pages[0])
	}
	if list, _ = work.Versions(p); list[0].Source != "restore" || !list[0].Current {
		t.Fatalf("versions after restore = %+v", list)
	}
	if _, err := s.RestoreDesignVersion(99); err == nil {
		t.Fatal("unknown version must fail")
	}

	if err := approve(s); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if _, err := s.EditDesign("PAGE-001", []work.CanvasOp{{Op: "set_page", Fields: map[string]string{"name": "X"}}}); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("edit after approval err = %v", err)
	}
}

func TestUnclearChangeRequestDropped(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	before := load(t, s).Design.Pages
	if err := s.ReviseDesign("rubah ini", nil, nil); err != nil {
		t.Fatal(err)
	}
	// The AI finds nothing to change: the request is dropped, not retried.
	if err := s.SubmitWork(work.KindDesign, "```json\n{\"pages\": []}\n```"); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	mustJob(t, p, work.KindUnclear, core.JobFailed)
	if d := p.Design; d.Status != core.DesignDraft || len(d.Feedback) != 0 || d.Scope != nil || !reflect.DeepEqual(d.Pages, before) {
		t.Fatalf("design = %+v", d)
	}
	if !strings.Contains(p.Job.Error, "“rubah ini”") {
		t.Fatalf("job = %+v", p.Job)
	}
	if err := s.RetryJob(); !errors.Is(err, pipeline.ErrNothingToDo) {
		t.Fatalf("retry err = %v", err)
	}
	// The user closes the message; only a failed job can be closed.
	if err := s.DismissJob(); err != nil || load(t, s).Job != nil {
		t.Fatalf("dismiss err = %v", err)
	}
	if err := s.DismissJob(); !errors.Is(err, pipeline.ErrNothingToDo) {
		t.Fatalf("dismiss err = %v", err)
	}
	// A clearer request works as usual.
	if err := s.ReviseDesign("Tambahkan halaman profil", nil, nil); err != nil {
		t.Fatal(err)
	}
	mustJob(t, load(t, s), work.KindDesign, core.JobWaiting)
}

func TestTechStack(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
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
		len(pr.Changes) != 1 || pr.Changes[0].To != "Vue + TypeScript + Vite" || len(pr.Impact) != 1 || !pr.Impact[0].Design {
		t.Fatalf("proposal = %+v", p.Stack)
	}
	for _, bad := range [][2]string{{core.TechArchitecture, "Serverless"}, {core.TechDesktop, "Tauri + React + TypeScript"}, {"ui", "x"}} {
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
	if p.Stack.State() != core.TechImpactChecked || p.Stack.Version() != 2 || len(p.Stack.History[1].Changes) != 2 {
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
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	if p = load(t, s); p.Design.TechVersion != 3 {
		t.Fatalf("design tech version = %d", p.Design.TechVersion)
	}
	if err := s.ApproveDesign(); !errors.Is(err, pipeline.ErrStackNotApproved) {
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
	if err := s.ApproveDesign(); !errors.Is(err, pipeline.ErrStackNotApproved) {
		t.Fatalf("changed stack err = %v", err)
	}
	if _, err := s.ApproveStack(); err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveDesign(); !errors.Is(err, pipeline.ErrTechReview) {
		t.Fatalf("unreviewed design err = %v", err)
	}
	if _, err := s.AckTechReview(); err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveDesign(); !errors.Is(err, pipeline.ErrStructureNotApproved) {
		t.Fatalf("unapproved structure err = %v", err)
	}
	if _, err := s.ApproveStructure(); err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveDesign(); err != nil {
		t.Fatal(err)
	}
	// The final spec carries the approved decision.
	raw, err := os.ReadFile(filepath.Join(s.Store.Dir, "docs", "README.md"))
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
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.RequestStackChange("Ganti backend ke Rust"); !errors.Is(err, pipeline.ErrBusy) {
		t.Fatalf("request while design waits err = %v", err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
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
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
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
	raw, err := os.ReadFile(filepath.Join(s.Store.Dir, "docs", "README.md"))
	if err != nil || !strings.Contains(string(raw), "Tipe: modular") || !strings.Contains(string(raw), "apps/api") {
		t.Fatalf("spec = %s, %v", raw, err)
	}
}

func TestRegenerateDesign(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDiscovery, discoveryJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	features := load(t, s).Features
	if _, err := s.SetDesignTemplate("landing-page"); err == nil {
		t.Fatal("unknown template must fail")
	}
	// The user's template applies when the design is generated again.
	if _, err := s.SetDesignTemplate("trading-terminal"); err != nil {
		t.Fatal(err)
	}
	if err := s.RegenerateDesign(); err != nil {
		t.Fatal(err)
	}
	// A full design prompt, not a revision; discovery is not run again.
	w, _ := s.NextWork()
	if w.Kind != work.KindDesign || !strings.Contains(w.Prompt, "the user picked trading-terminal") || strings.Contains(w.Prompt, "CHANGE") {
		t.Fatalf("work = %+v", w)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if !reflect.DeepEqual(p.Features, features) || p.Design.Status != core.DesignDraft {
		t.Fatalf("project = %+v", p.Design)
	}
	if c := p.Design.Template; c == nil || c.ID != "trading-terminal" || c.Source != "user" {
		t.Fatalf("template = %+v", c)
	}
	if err := approve(s); err != nil {
		t.Fatal(err)
	}
	if err := s.RegenerateDesign(); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("approved err = %v", err)
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
	return s.ApproveDesign()
}
