package work

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func supportProject(t *testing.T) *core.Project {
	p := revisionProject()
	p.Dir = t.TempDir()
	p.Discovery = &core.Discovery{Roles: []core.Role{{Name: "Admin Klinik"}}, Users: []string{"Pasien"}}
	p.Flows[0].Name, p.Flows[0].Role, p.Flows[0].Steps = "Buat janji", "Pasien", []string{"Pilih dokter", "Konfirmasi"}
	os.MkdirAll(filepath.Join(p.Dir, "api"), 0o755)
	os.WriteFile(filepath.Join(p.Dir, "api", "janji.go"), []byte("package api"), 0o644)
	return p
}

func TestAskValidatesAndQueues(t *testing.T) {
	p := supportProject(t)
	if err := Ask(&core.Project{}, "Bagaimana alur janji?", core.SupportRunning); !errors.Is(err, ErrNoSupportIntel) {
		t.Fatalf("no discovery err = %v", err)
	}
	if err := Ask(p, "  ", core.SupportRunning); err == nil {
		t.Fatal("empty question accepted")
	}
	if err := Ask(p, "Bagaimana alur janji?", core.SupportRunning); err != nil {
		t.Fatal(err)
	}
	if err := Ask(p, "lagi", core.SupportRunning); !errors.Is(err, ErrSupportBusy) {
		t.Fatalf("second question err = %v", err)
	}
	if err := ClearSupport(p); !errors.Is(err, ErrSupportBusy) {
		t.Fatalf("clear while answering err = %v", err)
	}
	got := SupportPrompt(p)
	for _, want := range []string{"QUESTION:\nBagaimana alur janji?", `"Pilih dokter"`, "Never invent", "NO access to live data"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	FailSupport(p, errors.New("timeout"))
	if m := p.Support.Messages[1]; m.Status != core.SupportFailed || m.Error != "timeout" {
		t.Fatalf("failed = %+v", m)
	}
	if err := RetrySupport(p, core.SupportWaiting); err != nil || PendingSupport(p) == nil {
		t.Fatalf("retry err = %v", err)
	}
}

func TestApplySupportDropsInventedReferences(t *testing.T) {
	p := supportProject(t)
	Ask(p, "Kenapa janji saya pending?", core.SupportRunning)
	err := ApplySupport(p, "```json\n"+`{"kind": "?", "summary": " Janji menunggu konfirmasi. ",
	 "flow": {"flow_id": "FLOW-009", "name": "Buat janji", "steps": [{"name": "Pilih dokter", "page_id": "PAGE-001"},
	   {"name": "Konfirmasi", "page_id": "PAGE-404", "current": true}, {"name": "Bayar", "current": true}, {"name": " "}]},
	 "trouble": {"problem": "Janji pending", "causes": [{"text": "Belum dikonfirmasi admin", "confirmed": true},
	   {"text": "Validasi jadwal", "evidence": "api/janji.go:12", "confirmed": true}, {"text": ""}]},
	 "roles": [{"role": "admin klinik", "access": "konfirmasi janji", "source": "FLOW-001"}, {"role": "Supervisor", "access": "approve"}],
	 "sources": ["FLOW-001", "FLOW-001", "REQ-999", "api/janji.go:12", "../etc/passwd", "/etc/passwd", "api"]}`+"\n```")
	if err != nil {
		t.Fatal(err)
	}
	m := p.Support.Messages[1]
	a := m.Answer
	if m.Status != core.SupportDone || a.Kind != "troubleshoot" || a.Summary != "Janji menunggu konfirmasi." {
		t.Fatalf("answer = %+v", m)
	}
	if a.Flow.FlowID != "" || len(a.Flow.Steps) != 3 || a.Flow.Steps[0].PageID != "PAGE-001" || a.Flow.Steps[1].PageID != "" ||
		!a.Flow.Steps[1].Current || a.Flow.Steps[2].Current {
		t.Fatalf("flow = %+v", a.Flow)
	}
	if c := a.Trouble.Causes; len(c) != 2 || c[0].Confirmed || !c[1].Confirmed {
		t.Fatalf("causes = %+v", c)
	}
	if len(a.Roles) != 1 || a.Roles[0].Role != "Admin Klinik" {
		t.Fatalf("roles = %+v", a.Roles)
	}
	if !slices.Equal(a.Sources, []string{"FLOW-001", "api/janji.go:12"}) {
		t.Fatalf("sources = %v", a.Sources)
	}
	if err := ApplySupport(p, `{"summary": "x"}`); err == nil {
		t.Fatal("answer without a pending question accepted")
	}
}

func TestEscalate(t *testing.T) {
	p := supportProject(t)
	if _, err := Escalate(p, core.Escalation{Issue: " "}); err == nil {
		t.Fatal("empty issue accepted")
	}
	e, err := Escalate(p, core.Escalation{Issue: "Janji pending", Evidence: []string{"FLOW-001", " "}})
	if err != nil || e.ID != "ESC-001" || len(e.Evidence) != 1 {
		t.Fatalf("escalation = %+v err = %v", e, err)
	}
}
