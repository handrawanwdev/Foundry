package core

import (
	"slices"
	"strings"
	"testing"
)

func consistencyProject() *Project {
	return &Project{
		Discovery: &Discovery{Roles: []Role{{Name: "Pasien"}}},
		Platform:  &PlatformPlan{Targets: []PlatformTarget{{Kind: PlatformBackend}}},
		Requirements: []Requirement{
			{ID: "REQ-001", Text: "Buat janji", Priority: "MUST", Acceptance: []Criterion{{ID: "REQ-001-AC1", Kind: "behavior", Text: "Janji tersimpan"}}},
		},
		Features: []Feature{{ID: "FEAT-001", Name: "Janji", RequirementIDs: []string{"REQ-001"}}},
		Flows:    []Flow{{ID: "FLOW-001", Name: "Buat janji", FeatureID: "FEAT-001", Role: "Pasien", Goal: "Dapat jadwal", Steps: []string{"Pilih jadwal"}, Post: []string{"Janji tersimpan"}, Failures: []string{"Jadwal penuh"}}},
		Blueprint: &Blueprint{
			Modules: []Module{{ID: "identity", Responsibility: "Akun", Interface: []string{"POST /auth/login"}},
				{ID: "janji-temu", Responsibility: "Janji", Interface: []string{"POST /appointments"}, DependsOn: []string{"identity"}}},
			Database: DatabaseArch{Entities: []Entity{{Name: "Appointment", Module: "janji-temu", Fields: []string{"id"}}}},
			Authz:    AuthzArch{Model: "RBAC", Permissions: []string{"appointment:create"}},
		},
		Done: []string{"Test lulus"},
	}
}

func TestConsistencyIssuesClean(t *testing.T) {
	p := consistencyProject()
	if errs := Errors(ConsistencyIssues(p)); len(errs) != 0 {
		t.Fatalf("clean project has errors: %+v", errs)
	}
}

// Cross-artifact conflicts are found with a location, the consequence and a
// suggestion; a cycle or dangling module reference is an error that blocks.
func TestConsistencyIssuesDetectsConflicts(t *testing.T) {
	p := consistencyProject()
	p.Blueprint.Modules[1].DependsOn = []string{"missing"}
	p.Features[0].RequirementIDs = nil
	p.Flows[0].Role = "TidakDikenal"
	p.Flows[0].Permissions = []string{"appointment:delete"}
	p.Flows[0].Data = []string{"Ghost: dibuat"}

	issues := ConsistencyIssues(p)
	has := func(level, where, msg string) bool {
		return slices.ContainsFunc(issues, func(i Issue) bool {
			return i.Level == level && i.Where == where && i.Message == msg &&
				strings.TrimSpace(i.Impact) != "" && strings.TrimSpace(i.Suggestion) != ""
		})
	}
	for _, want := range []struct{ level, where, msg string }{
		{IssueError, "module:janji-temu", `modul janji-temu bergantung pada modul "missing" yang tidak ada`},
		{IssueWarning, "feature:FEAT-001", "fitur FEAT-001 Janji tidak terhubung ke requirement mana pun"},
		{IssueWarning, "FLOW-001", `aktor "TidakDikenal" belum terdaftar sebagai peran pengguna`},
		{IssueWarning, "FLOW-001", `permission "appointment:delete" dipakai use case tetapi tidak terdaftar di otorisasi`},
		{IssueWarning, "FLOW-001", `efek data "Ghost: dibuat" menyebut entity "Ghost" yang tidak ada di model data`},
	} {
		if !has(want.level, want.where, want.msg) {
			t.Errorf("missing finding (%s) %s %q in %+v", want.level, want.where, want.msg, issues)
		}
	}
	if errs := Errors(issues); len(errs) != 1 || errs[0].Where != "module:janji-temu" {
		t.Fatalf("errors = %+v", Errors(issues))
	}
}

// A module that references nothing outside itself is fine; depending on
// itself or forming a cycle is an error.
func TestConsistencyIssuesModuleCycle(t *testing.T) {
	p := consistencyProject()
	p.Blueprint.Modules[1].DependsOn = []string{"identity"}
	p.Blueprint.Modules[0].DependsOn = []string{"janji-temu"}
	issues := ConsistencyIssues(p)
	if !slices.ContainsFunc(issues, func(i Issue) bool {
		return i.Level == IssueError && i.Where == "architecture" && strings.Contains(i.Message, "dependency modul melingkar")
	}) {
		t.Fatalf("cycle not reported: %+v", issues)
	}
	p.Blueprint.Modules[0].DependsOn = []string{"identity"}
	if errs := Errors(ConsistencyIssues(p)); len(errs) == 0 || errs[0].Where != "module:identity" {
		t.Fatalf("self dependency not reported as error: %+v", errs)
	}
}

// A module without a public interface is an info recommendation, never a
// blocker.
func TestConsistencyIssuesInfoLevel(t *testing.T) {
	p := consistencyProject()
	p.Blueprint.Modules[0].Interface = nil
	issues := ConsistencyIssues(p)
	if !slices.ContainsFunc(issues, func(i Issue) bool {
		return i.Level == IssueInfo && i.Where == "module:identity" && strings.Contains(i.Message, "antarmuka publik")
	}) {
		t.Fatalf("missing info finding: %+v", issues)
	}
	if errs := Errors(issues); len(errs) != 0 {
		t.Fatalf("info must not block: %+v", errs)
	}
}
