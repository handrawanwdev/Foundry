package progress

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

func linkedProject() *core.Project {
	return &core.Project{
		Name: "Klinik", Dir: "/tmp/klinik",
		Features: []core.Feature{
			{ID: "FEAT-001", Name: "Janji", SourceID: "janji", RequirementIDs: []string{"REQ-001"},
				SubFeatures: []core.SubFeature{{ID: "FEAT-001.1", Name: "Form janji", FeatureID: "FEAT-001", SourceID: "form"}}},
			{ID: "FEAT-002", Name: "Jadwal", SourceID: "jadwal", RequirementIDs: []string{"REQ-002"}},
			{ID: "FEAT-003", Name: "Laporan", SourceID: "laporan", RequirementIDs: []string{"REQ-003"}},
		},
		FeatureMap: &core.FeatureMap{Modules: []core.FeatureModule{{ID: "antrean", Name: "Antrean", Features: []core.FeatureNode{
			{ID: "janji", Name: "Janji", Children: []core.FeatureNode{{ID: "form", Name: "Form janji"}}},
			{ID: "jadwal", Name: "Jadwal"},
			{ID: "laporan", Name: "Laporan"},
		}}}},
		Workflow: &core.Workflow{ID: "WF-001", Version: 1, Commands: []core.Command{
			{ID: "CMD-001", Name: "Skema", Instructions: "i", FeatureIDs: []string{"FEAT-001"}, Covers: []string{"REQ-001"},
				Acceptance: []string{"tabel ada", "migrasi jalan"}},
			{ID: "CMD-002", Name: "Form", Instructions: "i", DependsOn: []string{"CMD-001"},
				SubFeatureIDs: []string{"FEAT-001.1"}, Acceptance: []string{"form mengirim"}},
			{ID: "CMD-003", Name: "Jadwal dan janji", Instructions: "i", FeatureIDs: []string{"FEAT-001", "FEAT-002"}},
		}},
	}
}

func TestFeatureRollupsAreWeightedAndNeverDuplicateCommands(t *testing.T) {
	p := linkedProject()
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	s := View(p, now, Agent{})
	if s.Percent != 0 || s.Overall.Total != 3 || s.Overall.Status != RollupNotStarted {
		t.Fatalf("unreported overall = %+v", s.Overall)
	}
	if len(s.Modules) != 1 || s.Modules[0].ID != "antrean" || s.Modules[0].Total != 3 {
		t.Fatalf("modules = %+v", s.Modules)
	}
	var janji, jadwal, laporan *FeatureProgress
	for i := range s.Features {
		switch s.Features[i].ID {
		case "FEAT-001":
			janji = &s.Features[i]
		case "FEAT-002":
			jadwal = &s.Features[i]
		case "FEAT-003":
			laporan = &s.Features[i]
		}
	}
	if janji == nil || jadwal == nil || laporan == nil {
		t.Fatalf("features = %+v", s.Features)
	}
	if janji.Total != 3 || janji.Weight != 7 || len(janji.Commands) != 3 {
		t.Fatalf("janji = %+v", janji)
	}
	if len(janji.SubFeatures) != 1 || janji.SubFeatures[0].Total != 1 || janji.SubFeatures[0].FeatureMapID != "form" {
		t.Fatalf("subfeature = %+v", janji.SubFeatures)
	}
	if jadwal.Total != 1 || jadwal.Weight != 1 {
		t.Fatalf("jadwal = %+v", jadwal)
	}
	if laporan.Status != RollupEmpty || laporan.Total != 0 {
		t.Fatalf("feature without command = %+v", laporan)
	}

	in := report("CMD-001", core.RunDone)
	in.Evidence = []core.Evidence{{Kind: "files", Ref: "db/schema.sql"}}
	mustApply(t, p, in, now)
	s = View(p, now, Agent{})
	janji = featureByID(s.Features, "FEAT-001")
	if s.Percent != 43 || janji.Percent != 43 || janji.Completed != 1 || janji.Weight != 7 {
		t.Fatalf("weighted progress overall=%d feature=%+v", s.Percent, janji)
	}
	if s.Modules[0].Total != 3 || s.Modules[0].Completed != 1 {
		t.Fatalf("module counted a shared command twice: %+v", s.Modules[0])
	}
	if len(s.Next) != 2 || s.Next[0] != "CMD-002" || s.Next[1] != "CMD-003" {
		t.Fatalf("next = %v; CMD-002 becomes ready only after CMD-001 is verified", s.Next)
	}
}

func TestDependencyAndNoReportStayUnknown(t *testing.T) {
	p := linkedProject()
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	s := View(p, now, Agent{})
	if len(s.Workflow.Dependencies) != 1 || s.Workflow.Dependencies[0].Satisfied {
		t.Fatalf("dependencies = %+v", s.Workflow.Dependencies)
	}
	if s.Commands[1].Status != core.RunWaiting || s.Commands[0].Status != core.RunUnconfirmed {
		t.Fatalf("commands = %+v", s.Commands)
	}
	if s.Features[0].Status != RollupNotStarted {
		t.Fatalf("feature assumed progress: %+v", s.Features[0])
	}

	mustApply(t, p, report("CMD-002", core.RunStarted), now)
	s = View(p, now, Agent{})
	if !s.Commands[1].EarlyStart || len(s.Commands[1].Waiting) != 1 {
		t.Fatalf("early start = %+v", s.Commands[1])
	}
	if s.Workflow.Dependencies[0].Satisfied {
		t.Fatal("dependency marked satisfied before verification")
	}
}

func TestReportFilesAndFinishedTimeAreStored(t *testing.T) {
	p := linkedProject()
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	finish := now.Add(10 * time.Minute)
	in := report("CMD-001", core.RunDone)
	in.Summary = "migrasi selesai"
	in.Files = []string{"db/schema.sql"}
	in.Changes = []string{"migration 001"}
	in.FinishedAt = &finish
	out := mustApply(t, p, in, now.Add(11*time.Minute))
	if out.AppliedStatus != core.RunVerified {
		t.Fatalf("done with files evidence = %+v", out)
	}
	if ev := p.Progress.Commands["CMD-001"].Evidence; len(ev) != 2 || ev[0].Kind != "files" || ev[1].Kind != "diff" {
		t.Fatalf("evidence = %+v", ev)
	}
	s := View(p, now.Add(12*time.Minute), Agent{})
	if s.Commands[0].FinishedAt == nil || !s.Commands[0].FinishedAt.Equal(finish) {
		t.Fatalf("finished_at = %+v", s.Commands[0])
	}
}

func TestSnapshotJSONExposesRollups(t *testing.T) {
	p := linkedProject()
	b, err := json.Marshal(View(p, time.Now().UTC(), Agent{}))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{`"percent":`, `"overall":`, `"workflow":`, `"design_status"`, `"modules":[`, `"features":[`, `"sub_features":[`, `"feature_map_id":"janji"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("snapshot JSON lacks %s: %s", want, got)
		}
	}
}

func featureByID(feats []FeatureProgress, id string) *FeatureProgress {
	for i := range feats {
		if feats[i].ID == id {
			return &feats[i]
		}
	}
	return nil
}
