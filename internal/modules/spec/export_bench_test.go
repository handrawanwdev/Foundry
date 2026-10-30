package spec

import (
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func benchProject() *core.Project {
	p := &core.Project{
		Version: 42,
		Name:    "RBAC Management System",
		Stage:   core.StageDesign,
		Story:   "Sistem manajemen peran dan akses untuk perusahaan.",
		Discovery: &core.Discovery{
			Summary:  "RBAC untuk admin dan auditor.",
			Business: "Kontrol akses terpusat.",
			Roles:    []core.Role{{Name: "Admin", Description: "mengelola peran", Platform: "web"}},
		},
		Survey: &core.Survey{Kinds: []string{core.SurveyKindWeb}, Users: core.SurveyUsersInternal, Stage: core.SurveyStageMVP, Architecture: core.SurveyArchModular},
		UI:     sampleUI(),
	}
	body := "Instruksi implementasi yang merujuk artefak blueprint, kriteria penerimaan, dan batasan teknis yang harus diikuti agent.\n"
	for i := 0; i < 80; i++ {
		id := "REQ-" + pad3(i+1)
		p.Requirements = append(p.Requirements, core.Requirement{
			ID: id, Text: "Kebutuhan " + id + " tentang otorisasi dan jejak audit.", Priority: "MUST",
			Acceptance: []core.Criterion{{ID: id + "-AC1", Kind: "functional", Text: "Sistem menolak akses tanpa izin.", Evidence: "test"}},
		})
	}
	for i := 0; i < 40; i++ {
		id := "FEAT-" + pad3(i+1)
		p.Features = append(p.Features, core.Feature{
			ID: id, Name: "Fitur " + id, Description: "Deskripsi fitur " + id,
			RequirementIDs: []string{"REQ-" + pad3(i+1), "REQ-" + pad3((i+1)%80+1)},
		})
	}
	for i := 0; i < 30; i++ {
		id := "FLOW-" + pad3(i+1)
		p.Flows = append(p.Flows, core.Flow{
			ID: id, Name: "Alur " + id, FeatureID: "FEAT-" + pad3(i%40+1), Role: "Admin",
			Goal: "Menyelesaikan " + id, Steps: []string{"buka halaman", "isi form", "simpan"},
			Post: []string{"data tersimpan"}, Pre: []string{"sudah login"},
			Permissions: []string{"role.write"}, PageIDs: []string{"antrean"},
		})
	}
	cmds := make([]core.Command, 50)
	for i := range cmds {
		id := "CMD-" + pad3(i+1)
		cmds[i] = core.Command{
			ID: id, Name: "Kerjakan " + id, Objective: "Implementasikan " + id,
			Instructions: body + body + body, Status: core.CommandDraft,
			Covers:    []string{"REQ-" + pad3(i%80+1)},
			Artifacts: []core.ArtifactRef{{ID: "requirements", Hash: "abc123"}},
		}
		if i > 0 {
			cmds[i].DependsOn = []string{"CMD-" + pad3(i)}
		}
	}
	p.Workflow = &core.Workflow{Status: "draft", Version: 3, Commands: cmds}
	for i := 0; i < 20; i++ {
		p.Artifacts = append(p.Artifacts, core.Artifact{
			ID: "extra-" + pad3(i+1), Title: "Dokumen " + pad3(i+1), Category: core.ArtifactOther,
			Body: body + body + body + body, Version: 1,
		})
	}
	return p
}

func pad3(n int) string {
	s := itoa(n)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func BenchmarkArtifacts(b *testing.B) {
	p := benchProject()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		docs := Artifacts(p)
		if len(docs) < 10 {
			b.Fatalf("docs=%d", len(docs))
		}
	}
}

func BenchmarkBlueprintView(b *testing.B) {
	p := benchProject()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		docs := Artifacts(p)
		issues := append(p.AreaIssues(core.AreaUseCases), p.AreaIssues(core.AreaAcceptance)...)
		issues = append(issues, core.DecisionIssues(p.Decisions)...)
		issues = append(issues, core.ConsistencyIssues(p)...)
		_ = issues
		_ = BlueprintHash(docs)
		_, _ = core.BlueprintChecks(p, docs)
		_ = p.BindingDecisions()
	}
}

func BenchmarkFiles(b *testing.B) {
	p := benchProject()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		files := Files(p)
		if len(files) < 10 {
			b.Fatalf("files=%d", len(files))
		}
	}
}

func BenchmarkExportGET(b *testing.B) {
	p := benchProject()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, content := Package(p)
		n := 0
		for _, body := range content {
			n += len(body)
		}
		_ = m
		if n == 0 {
			b.Fatal("empty export")
		}
	}
}
