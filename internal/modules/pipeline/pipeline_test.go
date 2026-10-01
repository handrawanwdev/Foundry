package pipeline_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
  {"id": "P2", "name": "Konfirmasi Janji", "platform": "android", "role": "Pasien", "feature_id": "FEAT-001", "sub_feature_ids": ["FEAT-001.2"], "purpose": "Konfirmasi",
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
		if mode == "qa_fix" && strings.Contains(prompt, "-r1.png") {
			fmt.Println(`{"status": "fix", "notes": "tombol terlalu atas", "patches": [{"page": "PAGE-001", "operations": [{"op": "move", "target": 0, "after": 1}]}]}`)
			return
		}
		fmt.Println(`{"status": "pass"}`)
	case strings.Contains(prompt, "tech lead"):
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
		want := "1"
		if mode == "fail_test" {
			want = "2"
		}
		os.WriteFile(filepath.Join("app", id+"_test.go"), []byte("package app\n\nimport \"testing\"\n\nfunc Test_"+id+"(t *testing.T) {\n\tif F_"+id+"() != "+want+" {\n\t\tt.Fatal(\"salah\")\n\t}\n}\n"), 0o644)
		fmt.Println("done")
	case strings.Contains(prompt, "reviewer"):
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
	if _, err := os.Stat(filepath.Join(p.Dir, "services", "api")); err != nil {
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
	p := designReady(t, s)
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
	if d.System == nil || len(d.System.Colors) != 1 {
		t.Fatalf("system = %+v", d.System)
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
	w, _ = s.NextWork()
	if w.Kind != work.KindTask || w.TaskID != "TASK-001" {
		t.Fatalf("work = %+v", w)
	}
	// Host claims done without writing code: build fails (no go.mod) → TODO.
	tk, b, err := s.SubmitTask("TASK-001", []work.Verdict{{Index: 1, Met: true, Evidence: "x"}})
	if err != nil || b.Passed || tk.Status != core.TaskTodo {
		t.Fatalf("task=%v build=%v err=%v", tk.Status, b.Passed, err)
	}
	// Host implements, but gives no evidence → not DONE.
	w, _ = s.NextWork()
	dir := s.Store.Dir
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/k\n\ngo 1.22\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	tk, _, _ = s.SubmitTask(w.TaskID, []work.Verdict{{Index: 1, Met: true}})
	if tk.Status == core.TaskDone {
		t.Fatal("DONE without evidence")
	}
	w, _ = s.NextWork()
	tk, b, _ = s.SubmitTask(w.TaskID, []work.Verdict{{Index: 1, Met: true, Evidence: "go.mod dibuat"}})
	if tk.Status != core.TaskDone || !b.Passed {
		t.Fatalf("task = %s build %+v", tk.Status, b)
	}
}

type fakeRender struct{ fail bool }

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
	if w, _ := s.NextWork(); w.Kind != work.KindDesign {
		t.Fatalf("work = %+v", w)
	}
	s.SubmitWork(work.KindDesign, designJSON)
	w, _ := s.NextWork()
	if w.Kind != work.KindSupport || !strings.Contains(w.Prompt, "Bagaimana alur janji?") {
		t.Fatalf("work = %+v", w)
	}
	if err := s.SubmitWork(work.KindSupport, supportJSON); err != nil {
		t.Fatal(err)
	}
	if m := load(t, s).Support.Messages[1]; m.Status != core.SupportDone {
		t.Fatalf("answer = %+v", m)
	}
	if w, _ := s.NextWork(); w.Kind != "none" {
		t.Fatalf("work after answer = %+v", w)
	}
}
