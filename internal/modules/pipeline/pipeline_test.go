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
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
	"github.com/handrawanw/foundry/internal/modules/pipeline"
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

// Plan deliberately omits tests for FEAT-002 so coverage filling is exercised.
const planJSON = `{"tasks": [
  {"id": "T1", "title": "Scaffold project", "description": "Buat go.mod", "area": "scaffold", "feature_id": "", "depends_on": [],
   "acceptance_criteria": ["go build lulus"], "test_requirements": ["go test lulus"]},
  {"id": "T2", "title": "API appointment", "description": "POST /appointments", "area": "api", "requirement_ids": ["REQ-001"], "feature_id": "FEAT-001",
   "platform": "backend", "depends_on": ["T1"], "acceptance_criteria": ["Endpoint membuat janji", "Validasi jadwal bentrok"], "files": ["services/api"], "test_requirements": ["unit test"]},
  {"id": "T3", "title": "Test appointment", "description": "Test API", "area": "test", "feature_id": "FEAT-001", "depends_on": ["T2"],
   "acceptance_criteria": ["Test lulus"], "test_requirements": ["go test"]},
  {"id": "T4", "title": "API jadwal", "description": "CRUD jadwal", "area": "api", "feature_id": "FEAT-002", "depends_on": ["T1"],
   "acceptance_criteria": ["CRUD jadwal"], "test_requirements": ["unit test"]}]}`

const supportJSON = `{"kind": "flow", "summary": "Pasien memilih dokter lalu mengonfirmasi janji.",
 "flow": {"flow_id": "FLOW-001", "name": "Buat janji", "steps": [{"name": "Pilih dokter"}, {"name": "Pilih jadwal"}, {"name": "Konfirmasi"}]},
 "sources": ["FLOW-001", "FEAT-404"]}`

var taskIDRe = regexp.MustCompile(`TASK (TASK-\d+):`)

func fakeAgent() {
	in, _ := io.ReadAll(os.Stdin)
	prompt := string(in)
	mode := os.Getenv("FAKE_MODE")
	switch {
	case strings.HasPrefix(prompt, "Balas hanya"):
		fmt.Println("OK")
	case strings.Contains(prompt, "Technical Support Assistant"):
		if mode == "support_fail" {
			fmt.Println("tidak tahu")
			return
		}
		fmt.Println(supportJSON)
	case strings.Contains(prompt, "discovery analyst"):
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
	case strings.Contains(prompt, "tech lead"):
		if mode == "plan_fail" {
			fmt.Println("maaf, saya tidak bisa")
			return
		}
		fmt.Println(planJSON)
	case strings.Contains(prompt, "coding agent"):
		if mode == "crash" {
			fmt.Fprintln(os.Stderr, "agent crashed")
			os.Exit(1)
		}
		if mode == "hang" {
			time.Sleep(time.Minute)
		}
		m := taskIDRe.FindStringSubmatch(prompt)
		id := strings.ToLower(strings.ReplaceAll(m[1], "-", "_"))
		if _, err := os.Stat("go.mod"); err != nil {
			os.WriteFile("go.mod", []byte("module example.com/klinik\n\ngo 1.22\n"), 0o644)
		}
		os.MkdirAll("app", 0o755)
		os.WriteFile(filepath.Join("app", id+".go"), []byte("package app\n\nfunc F_"+id+"() int { return 1 }\n"), 0o644)
		if mode == "clash" { // every task adds the same file
			os.WriteFile("notes.txt", []byte(id+"\n"), 0o644)
		}
		if mode == "cross" && id == "task_001" {
			os.MkdirAll("b", 0o755)
			os.WriteFile(filepath.Join("b", "y.go"), []byte("package b\n"), 0o644)
		}
		want := "1"
		if mode == "fail_test" {
			want = "2"
		}
		os.WriteFile(filepath.Join("app", id+"_test.go"), []byte("package app\n\nimport \"testing\"\n\nfunc Test_"+id+"(t *testing.T) {\n\tif F_"+id+"() != "+want+" {\n\t\tt.Fatal(\"salah\")\n\t}\n}\n"), 0o644)
		if mode == "self_review" {
			var parts []string
			for i := 1; i <= 10; i++ {
				parts = append(parts, fmt.Sprintf(`{"index": %d, "met": true, "evidence": "app/%s.go"}`, i, id))
			}
			fmt.Printf("```json\n{\"criteria\": [%s]}\n```\n", strings.Join(parts, ","))
			return
		}
		fmt.Println("done")
	case strings.Contains(prompt, "reviewer"):
		if mode == "self_review" {
			fmt.Fprintln(os.Stderr, "reviewer should be skipped")
			os.Exit(3)
		}
		met := mode != "reject"
		var parts []string
		for i := 1; i <= 10; i++ {
			parts = append(parts, fmt.Sprintf(`{"index": %d, "met": %v, "evidence": "app/*.go"}`, i, met))
		}
		fmt.Printf("```json\n{\"criteria\": [%s]}\n```\n", strings.Join(parts, ","))
	default:
		fmt.Fprintln(os.Stderr, "unknown prompt")
		os.Exit(3)
	}
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
	h := work.Handoffs(p)[0]
	if h.Route != "/dokter" || h.PrimaryAction != "Pilih → Konfirmasi Janji (/janji/konfirmasi)" ||
		!slices.Contains(h.Tests, "Tanpa login diarahkan ke halaman masuk") || !slices.Contains(work.Handoffs(p)[1].Validation, "Tanggal: wajib, date") {
		t.Fatalf("handoff = %+v", h)
	}

	// User override: template.
	if _, err := s.SetTemplate("modular"); err != nil {
		t.Fatal(err)
	}
	// Tasks must not exist before approval.
	if len(load(t, s).Tasks) != 0 {
		t.Fatal("tasks created before approval")
	}
	if err := s.ApproveDesign(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	mustJob(t, p, work.KindPlan, core.JobDone)
	if p.Stage != core.StageBuild {
		t.Fatalf("stage = %s", p.Stage)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "apps", "api", "modules", "janji-temu")); err != nil {
		t.Fatalf("scaffold missing: %v", err)
	}
	// Coverage filler adds a test task for FEAT-002 and UI tasks for pages.
	var feat2Test, uiTasks int
	for _, tk := range p.Tasks {
		if tk.FeatureID == "FEAT-002" && tk.Area == "test" {
			feat2Test++
		}
		if tk.Area == "ui" {
			uiTasks++
		}
		if len(tk.AcceptanceCriteria) == 0 {
			t.Fatalf("%s has no acceptance criteria", tk.ID)
		}
	}
	if feat2Test != 1 || uiTasks != 3 {
		t.Fatalf("coverage: feat2Test=%d uiTasks=%d", feat2Test, uiTasks)
	}
	// DONE is gated.
	if _, err := s.MoveTask("TASK-002", core.TaskDone); !errors.Is(err, pipeline.ErrGate) {
		t.Fatalf("gate err = %v", err)
	}

	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	for _, tk := range p.Tasks {
		if tk.Status != core.TaskDone {
			t.Fatalf("%s status %s notes %v", tk.ID, tk.Status, tk.Notes)
		}
	}
	if p.Stage != core.StageReady || p.Verification == nil || !p.Verification.Ready {
		t.Fatalf("stage=%s verification=%+v", p.Stage, p.Verification)
	}
	for _, tr := range p.Verification.Trace {
		if !tr.Done || len(tr.TestTaskIDs) == 0 {
			t.Fatalf("trace %+v", tr)
		}
	}
}

func TestNoAIConnected(t *testing.T) {
	s := newService(t, fakeAI{none: true}, "")
	if _, err := s.SubmitStory("Aplikasi laundry untuk pelanggan"); !errors.Is(err, pipeline.ErrNoAI) {
		t.Fatalf("err = %v", err)
	}
	mustJob(t, load(t, s), work.KindDiscovery, core.JobFailed)
}

// Approval is the user's decision: a later scaffold or plan error must not
// put the design back to draft.
func TestApproveKeepsDesignOnLaterError(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	p := designReady(t, s)
	first := strings.TrimSuffix(p.Structure.Layout[0], "/")
	top := strings.Split(first, "/")[0]
	block := filepath.Join(p.Dir, top)
	if err := os.WriteFile(block, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveDesign(); err == nil {
		t.Fatal("expected scaffold error")
	}
	p = load(t, s)
	if p.Design.Status != core.DesignApproved || p.Stage != core.StagePlan {
		t.Fatalf("approval rolled back: status=%s stage=%s", p.Design.Status, p.Stage)
	}
	if p.Platform == nil || !p.Platform.Approved || p.Structure == nil || !p.Structure.Approved || p.Stack == nil || !p.Stack.Approved {
		t.Fatalf("locks undone: platform=%+v structure=%+v stack=%+v", p.Platform, p.Structure, p.Stack)
	}
	mustJob(t, p, work.KindPlan, core.JobFailed)
	if err := s.ReviseDesign("ubah warna", nil, nil); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("revise after approval err = %v", err)
	}

	if err := os.Remove(block); err != nil {
		t.Fatal(err)
	}
	if err := s.RetryJob(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	if !p.Structure.Scaffolded {
		t.Fatal("scaffold not retried")
	}
	mustJob(t, p, work.KindPlan, core.JobDone)

	s = newService(t, fakeAI{}, "plan_fail")
	designReady(t, s)
	if err := s.ApproveDesign(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	mustJob(t, p, work.KindPlan, core.JobFailed)
	if p.Design.Status != core.DesignApproved || p.Stage != core.StagePlan {
		t.Fatalf("plan error cancelled approval: status=%s stage=%s", p.Design.Status, p.Stage)
	}
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
	if err := s.ApproveDesign(); !errors.Is(err, pipeline.ErrIncomplete) {
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

func approveAndPlan(t *testing.T, s *pipeline.Service) {
	t.Helper()
	designReady(t, s)
	if err := s.ApproveDesign(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
}

func TestFailingTestsBlockTasks(t *testing.T) {
	s := newService(t, fakeAI{}, "fail_test")
	approveAndPlan(t, s)
	s.StartBuild()
	s.Wait()
	p := load(t, s)
	scaffold := p.Task("TASK-001")
	if scaffold.Status != core.TaskBlocked || scaffold.Attempts != pipeline.MaxAttempts {
		t.Fatalf("scaffold = %s attempts %d", scaffold.Status, scaffold.Attempts)
	}
	if !strings.Contains(strings.Join(scaffold.Notes, "\n"), "test gagal") {
		t.Fatalf("notes = %v", scaffold.Notes)
	}
	if p.Stage != core.StageVerify || p.Verification.Ready {
		t.Fatalf("stage = %s", p.Stage)
	}
	kinds := map[string]bool{}
	for _, is := range p.Verification.Issues {
		kinds[is.Kind] = true
	}
	if !kinds["task_blocked"] || !kinds["task_open"] || !kinds["test_failed"] {
		t.Fatalf("issues = %+v", p.Verification.Issues)
	}
	// Auto fix: blocked tasks reopen and a repair task is added.
	n := len(p.Tasks)
	os.Setenv("FAKE_MODE", "")
	if err := s.Fix(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	if len(p.Tasks) <= n {
		t.Fatal("fix added no task")
	}
	if p.Stage != core.StageReady {
		for _, is := range p.Verification.Issues {
			t.Log(is)
		}
		t.Fatalf("after fix stage = %s", p.Stage)
	}
}

func TestClampAgents(t *testing.T) {
	if pipeline.ClampAgents(0) != pipeline.DefaultAgents || pipeline.ClampAgents(-1) != pipeline.DefaultAgents {
		t.Fatal("zero/negative should be default")
	}
	if pipeline.ClampAgents(1) != 1 || pipeline.ClampAgents(pipeline.MaxAgents+5) != pipeline.MaxAgents {
		t.Fatal("clamp range")
	}
	t.Setenv("FOUNDRY_AGENTS", "2")
	n, fromEnv := pipeline.ParallelInfo(8)
	if n != 2 || !fromEnv {
		t.Fatalf("env override = %d %v", n, fromEnv)
	}
	t.Setenv("FOUNDRY_AGENTS", "")
	n, fromEnv = pipeline.ParallelInfo(5)
	if n != 5 || fromEnv {
		t.Fatalf("stored = %d %v", n, fromEnv)
	}
}

func TestMCPAgentLimit(t *testing.T) {
	t.Setenv("FOUNDRY_AGENTS", "1")
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.Stage = core.StageBuild
		p.Tasks = []core.Task{
			{ID: "T1", Title: "a", Area: "api", Files: []string{"a/x.go"}, AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo},
			{ID: "T2", Title: "b", Area: "api", Files: []string{"b/y.go"}, AcceptanceCriteria: []string{"ok"}, Status: core.TaskTodo},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	w1, err := s.NextWork("w1")
	if err != nil || w1.Kind != work.KindTask || w1.TaskID != "T1" {
		t.Fatalf("first = %+v %v", w1, err)
	}
	if len(w1.AlsoReady) != 0 {
		t.Fatalf("also_ready past cap: %v", w1.AlsoReady)
	}
	w2, err := s.NextWork("w2")
	if err != nil || w2.Kind != "wait" {
		t.Fatalf("second = %+v %v", w2, err)
	}
}

func TestSelfReviewSkipsReviewer(t *testing.T) {
	s := newService(t, fakeAI{}, "self_review")
	approveAndPlan(t, s)
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	for _, tk := range p.Tasks {
		if tk.Status != core.TaskDone {
			t.Fatalf("%s status %s notes %v", tk.ID, tk.Status, tk.Notes)
		}
	}
	if p.Stage != core.StageReady {
		t.Fatalf("stage = %s", p.Stage)
	}
}

func TestReviewerRejects(t *testing.T) {
	s := newService(t, fakeAI{}, "reject")
	approveAndPlan(t, s)
	s.StartBuild()
	s.Wait()
	tk := load(t, s).Task("TASK-001")
	if tk.Status != core.TaskBlocked || !strings.Contains(tk.Notes[0], "Acceptance criteria") {
		t.Fatalf("task = %s %v", tk.Status, tk.Notes)
	}
}

func TestAgentCrashStopsBuild(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	approveAndPlan(t, s)
	os.Setenv("FAKE_MODE", "crash")
	s.StartBuild()
	s.Wait()
	p := load(t, s)
	mustJob(t, p, "build", core.JobFailed)
	if !strings.Contains(p.Job.Error, "agent gagal") {
		t.Fatalf("error = %s", p.Job.Error)
	}
}

func TestStopBuildReleasesTask(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	approveAndPlan(t, s)
	os.Setenv("FAKE_MODE", "hang")
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	var running *core.Task
	for deadline := time.Now().Add(10 * time.Second); running == nil; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("no task started")
		}
		for i, tk := range load(t, s).Tasks {
			if tk.Status == core.TaskInProgress {
				running = &load(t, s).Tasks[i]
			}
		}
	}
	if err := s.StopBuild(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	tk := p.Task(running.ID)
	if tk.Status != core.TaskTodo || tk.Attempts != 0 || len(tk.Notes) != 0 {
		t.Fatalf("task = %s attempts %d notes %v", tk.Status, tk.Attempts, tk.Notes)
	}
	if p.Job.Message != "Build dihentikan" {
		t.Fatalf("job = %+v", p.Job)
	}
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
	if !strings.Contains(work.PlanPrompt(p), "register their routes in the existing router") {
		t.Fatalf("plan prompt misses the existing ui")
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
	var npmTest bool
	for _, c := range p.Structure.Commands {
		npmTest = npmTest || strings.Join(c.Argv, " ") == "npm run test"
	}
	if !npmTest {
		t.Fatalf("commands = %+v", p.Structure.Commands)
	}
	if _, err := s.SetTemplate("simple"); err == nil {
		t.Fatal("template must not replace existing structure")
	}
}

func TestMCPHostFlow(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	mustJob(t, load(t, s), work.KindDiscovery, core.JobWaiting)

	w, _ := s.NextWork("")
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
	if err := s.ApproveDesign(); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindPlan, planJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	w, _ = s.NextWork("")
	if w.Kind != work.KindTask || w.TaskID != "TASK-001" {
		t.Fatalf("work = %+v", w)
	}
	// Host claims done without writing code: build fails (no go.mod) → TODO.
	tk, b, err := s.SubmitTask("TASK-001", "", []work.Verdict{{Index: 1, Met: true, Evidence: "x"}}, nil)
	if err != nil || b.Passed || tk.Status != core.TaskTodo {
		t.Fatalf("task=%v build=%v err=%v", tk.Status, b.Passed, err)
	}
	// Host implements, but gives no evidence → not DONE.
	w, _ = s.NextWork("")
	dir := s.Store.Dir
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/k\n\ngo 1.22\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	tk, _, _ = s.SubmitTask(w.TaskID, "", []work.Verdict{{Index: 1, Met: true}}, nil)
	if tk.Status == core.TaskDone {
		t.Fatal("DONE without evidence")
	}
	w, _ = s.NextWork("")
	tk, b, _ = s.SubmitTask(w.TaskID, "", []work.Verdict{{Index: 1, Met: true, Evidence: "go.mod dibuat"}}, nil)
	if tk.Status != core.TaskDone || !b.Passed {
		t.Fatalf("task = %s build %+v", tk.Status, b)
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

// Two MCP hosts build in parallel: a task waits while another worker holds
// what it needs, and tasks that do not overlap run side by side.
func TestParallelWorkersQueueOnLocks(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	if _, err := s.SubmitStory("Aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct{ kind, json string }{{work.KindDiscovery, discoveryJSON}, {work.KindDesign, designJSON}} {
		if err := s.SubmitWork(step.kind, step.json); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ApproveDesign(); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitWork(work.KindPlan, planJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.StartBuild(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(logTexts(load(t, s)), "\n"), "Analisa: 1 task independen siap paralel (TASK-001)") {
		t.Fatalf("analysis = %v", logTexts(load(t, s)))
	}
	// Scaffold holds the whole project: worker b waits.
	if w, _ := s.NextWork("a"); w.TaskID != "TASK-001" {
		t.Fatalf("a = %+v", w)
	}
	if p := load(t, s).Task("TASK-001"); p == nil || p.Progress != "Host mengerjakan" {
		t.Fatalf("progress = %+v", p)
	}
	if w, _ := s.NextWork("b"); w.Kind != "wait" {
		t.Fatalf("b = %+v", w)
	}
	if w, _ := s.NextWork("a"); w.TaskID != "TASK-001" {
		t.Fatalf("a again = %+v", w)
	}
	dir := s.Store.Dir
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/k\n\ngo 1.22\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	s.Store.Update(func(p *core.Project) error { p.Task("TASK-002").Files = []string{"services/api/main.go"}; return nil })
	patches := []work.Patch{{File: "services/api/main.go", Change: "daftarkan route /health"}, {File: "", Change: "x"}}
	if tk, _, err := s.SubmitTask("TASK-001", "", []work.Verdict{{Index: 1, Met: true, Evidence: "go.mod"}}, patches); err != nil || tk.Status != core.TaskDone {
		t.Fatalf("task = %v err %v", tk, err)
	}
	// The host's patch for a file TASK-002 owns becomes TASK-002's feedback.
	if n := load(t, s).Task("TASK-002").Notes; len(n) != 1 || !strings.Contains(n[0], "Patch dari TASK-001 untuk services/api/main.go: daftarkan route /health") {
		t.Fatalf("notes = %v", n)
	}
	// TASK-002 (services/api) and TASK-004 (FEAT-002) do not overlap.
	wa, _ := s.NextWork("a")
	wb, _ := s.NextWork("b")
	if wa.TaskID != "TASK-002" || wb.TaskID != "TASK-004" {
		t.Fatalf("a = %s, b = %s", wa.TaskID, wb.TaskID)
	}
	if !slices.Contains(wa.AlsoReady, "TASK-004") {
		t.Fatalf("also_ready a=%v (want TASK-004)", wa.AlsoReady)
	}
	if w, _ := s.NextWork("c"); w.Kind != "wait" {
		t.Fatalf("c = %+v", w)
	}
	p := load(t, s)
	if p.Task("TASK-002").Owner != "a" || p.Task("TASK-004").Owner != "b" || len(p.ActiveTasks()) != 2 {
		t.Fatalf("active = %d", len(p.ActiveTasks()))
	}
	// Host a goes silent past the lease: c gets its task, a can no longer
	// submit it; b stays alive through next_work.
	s.Store.Update(func(p *core.Project) error {
		p.Task("TASK-002").Seen = time.Now().Add(-pipeline.HostLease - time.Minute)
		return nil
	})
	if w, _ := s.NextWork("b"); w.TaskID != "TASK-004" {
		t.Fatalf("b again = %+v", w)
	}
	if w, _ := s.NextWork("c"); w.TaskID != "TASK-002" {
		t.Fatalf("c after lease = %+v", w)
	}
	if tk := load(t, s).Task("TASK-002"); tk.Owner != "c" || tk.Attempts != 1 {
		t.Fatalf("reclaimed = %+v", tk)
	}
	if _, _, err := s.SubmitTask("TASK-002", "a", nil, nil); err == nil || !strings.Contains(err.Error(), "dipegang c") {
		t.Fatalf("stale submit err = %v", err)
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

func TestSupportAgent(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	if err := s.AskSupport("Bagaimana alur janji?"); !errors.Is(err, work.ErrNoSupportIntel) {
		t.Fatalf("before discovery err = %v", err)
	}
	designReady(t, s)
	if err := s.AskSupport("Bagaimana alur janji?"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	m := p.Support.Messages[1]
	if m.Status != core.SupportDone || m.Answer.Flow.FlowID != "FLOW-001" || len(m.Answer.Flow.Steps) != 3 ||
		!slices.Equal(m.Answer.Sources, []string{"FLOW-001"}) {
		t.Fatalf("answer = %+v", m)
	}
	mustJob(t, p, work.KindDesign, core.JobDone) // the pipeline job is untouched
	if err := s.ClearSupport(); err != nil || len(load(t, s).Support.Messages) != 0 {
		t.Fatalf("clear err = %v", err)
	}
}

func TestSupportAgentFailsAndRetries(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	t.Setenv("FAKE_MODE", "support_fail")
	s.AskSupport("Siapa yang bisa mengubah jadwal?")
	s.Wait()
	if m := load(t, s).Support.Messages[1]; m.Status != core.SupportFailed || m.Error == "" {
		t.Fatalf("message = %+v", m)
	}
	t.Setenv("FAKE_MODE", "")
	if err := s.RetrySupport(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if m := load(t, s).Support.Messages[1]; m.Status != core.SupportDone {
		t.Fatalf("retried = %+v", m)
	}
}

func TestSupportMCPHost(t *testing.T) {
	s := newService(t, fakeAI{mcp: true}, "")
	s.SubmitStory("Aplikasi klinik untuk janji temu pasien")
	s.SubmitWork(work.KindDiscovery, discoveryJSON)
	if err := s.AskSupport("Bagaimana alur janji?"); err != nil {
		t.Fatal(err)
	}
	// A waiting pipeline job goes first; the question follows.
	if w, _ := s.NextWork(""); w.Kind != work.KindDesign {
		t.Fatalf("work = %+v", w)
	}
	s.SubmitWork(work.KindDesign, designJSON)
	w, _ := s.NextWork("")
	if w.Kind != work.KindSupport || !strings.Contains(w.Prompt, "Bagaimana alur janji?") {
		t.Fatalf("work = %+v", w)
	}
	if err := s.SubmitWork(work.KindSupport, supportJSON); err != nil {
		t.Fatal(err)
	}
	if m := load(t, s).Support.Messages[1]; m.Status != core.SupportDone {
		t.Fatalf("answer = %+v", m)
	}
	if w, _ := s.NextWork(""); w.Kind != "none" {
		t.Fatalf("work after answer = %+v", w)
	}
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
	approveAndPlan(t, s)
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

	if err := s.ApproveDesign(); err != nil {
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
	if p.Structure.Template != "monorepo" || !slices.Contains(p.Structure.Layout, "apps/mobile/") {
		t.Fatalf("structure = %+v", p.Structure)
	}
	if _, err := s.SetStack(core.TechFrontend, "vue + typescript + vite", "Tim sudah pakai Vue"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][2]string{{core.TechArchitecture, "Serverless"}, {core.TechDesktop, "Tauri + React + TypeScript"}, {"ui", "x"}} {
		if _, err := s.SetStack(bad[0], bad[1], ""); err == nil {
			t.Fatalf("%v must fail", bad)
		}
	}
	p, err := s.SetStack(core.TechRepository, core.RepoSimple, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Structure.Template != "simple" || !slices.Equal(p.Structure.Layout, []string{"frontend/", "mobile/", "backend/", "docs/"}) {
		t.Fatalf("structure = %+v", p.Structure)
	}
	// A platform change decides again but keeps the user's choices.
	p, err = s.SetPlatform([]core.PlatformTarget{{Kind: "web"}, {Kind: "backend"}})
	if err != nil {
		t.Fatal(err)
	}
	d := p.Stack.Decision(core.TechFrontend)
	if p.Stack.Decision(core.TechMobile) != nil || d.Choice() != "Vue + TypeScript + Vite" || d.Lang() != "TypeScript" ||
		d.Override.Reason != "Tim sudah pakai Vue" || p.Stack.Choice(core.TechRepository) != core.RepoSimple {
		t.Fatalf("stack = %+v", p.Stack)
	}
	if err := s.SubmitWork(work.KindDesign, designJSON); err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveDesign(); err != nil {
		t.Fatal(err)
	}
	// The plan is made with the same decision.
	w, _ := s.NextWork("")
	if w.Kind != work.KindPlan || !strings.Contains(w.Prompt, "- frontend: Vue + TypeScript + Vite (language TypeScript) — chosen by the user: Tim sudah pakai Vue") {
		t.Fatalf("work = %+v", w)
	}
	if _, err := s.SetStack(core.TechFrontend, "", ""); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("approved err = %v", err)
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
	w, _ := s.NextWork("")
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
	if err := s.ApproveDesign(); err != nil {
		t.Fatal(err)
	}
	if err := s.RegenerateDesign(); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("approved err = %v", err)
	}
}
