// Package verify checks the whole chain Story → Requirement → Feature →
// Page → Task → Test → Build and reports what is still missing.
package verify

import (
	"fmt"
	"time"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/scan"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// Report builds a verification from current state. lastBuild is the final
// full build run (nil when none ran).
func Report(p *core.Project, lastBuild *core.BuildRun) *core.Verification {
	v := &core.Verification{At: time.Now().UTC()}
	issue := func(kind, ref, msg string) {
		v.Issues = append(v.Issues, core.Issue{Kind: kind, Ref: ref, Message: msg})
	}

	pagesByFeature := map[string][]string{}
	if p.Design != nil {
		for _, pg := range p.Design.Pages {
			pagesByFeature[pg.FeatureID] = append(pagesByFeature[pg.FeatureID], pg.ID)
		}
	}
	tasksByFeature := map[string][]core.Task{}
	for _, t := range p.Tasks {
		tasksByFeature[t.FeatureID] = append(tasksByFeature[t.FeatureID], t)
	}

	// Requirements → features.
	for _, r := range p.Requirements {
		tr := core.Trace{RequirementID: r.ID, Done: true}
		for _, f := range p.Features {
			for _, rid := range f.RequirementIDs {
				if rid == r.ID {
					tr.FeatureIDs = append(tr.FeatureIDs, f.ID)
					tr.PageIDs = append(tr.PageIDs, pagesByFeature[f.ID]...)
					for _, t := range tasksByFeature[f.ID] {
						if t.Area != "test" {
							tr.TaskIDs = append(tr.TaskIDs, t.ID)
						}
						if t.Area == "test" || len(t.TestRequirements) > 0 {
							tr.TestTaskIDs = append(tr.TestTaskIDs, t.ID)
						}
						if t.Status != core.TaskDone {
							tr.Done = false
						}
					}
				}
			}
		}
		if len(tr.FeatureIDs) == 0 {
			issue("missing_feature", r.ID, r.ID+" tidak punya feature")
			tr.Done = false
		}
		if len(tr.TaskIDs) == 0 {
			tr.Done = false
		}
		v.Trace = append(v.Trace, tr)
	}

	// Features → pages, tasks, tests.
	uiFeature := map[string]bool{}
	for _, f := range work.UIFeatures(p) {
		uiFeature[f.ID] = true
	}
	for _, f := range p.Features {
		if uiFeature[f.ID] && len(pagesByFeature[f.ID]) == 0 {
			issue("missing_page", f.ID, fmt.Sprintf("%s %s belum punya halaman", f.ID, f.Name))
		}
		// Tests usually ship with the task that writes the code
		// (test_requirements); a separate test task also counts.
		var impl, test int
		for _, t := range tasksByFeature[f.ID] {
			if t.Area == "test" || len(t.TestRequirements) > 0 {
				test++
			}
			if t.Area != "test" {
				impl++
			}
		}
		if impl == 0 {
			issue("missing_task", f.ID, fmt.Sprintf("%s %s belum punya task implementasi", f.ID, f.Name))
		}
		if test == 0 {
			issue("missing_test", f.ID, fmt.Sprintf("%s %s belum punya test", f.ID, f.Name))
		}
	}

	// Flows must walk through real pages.
	for _, fl := range p.Flows {
		if uiFeature[fl.FeatureID] && len(fl.PageIDs) == 0 {
			issue("broken_flow", fl.ID, fmt.Sprintf("Alur %s tidak terhubung ke halaman", fl.Name))
		}
	}

	// Tasks.
	open, blocked := 0, 0
	for _, t := range p.Tasks {
		switch t.Status {
		case core.TaskDone:
		case core.TaskBlocked:
			blocked++
			issue("task_blocked", t.ID, fmt.Sprintf("%s %s terblokir", t.ID, t.Title))
		default:
			open++
			issue("task_open", t.ID, fmt.Sprintf("%s %s belum selesai (%s)", t.ID, t.Title, t.Status))
		}
	}

	// Build + test.
	buildOK, testOK := false, false
	switch {
	case lastBuild == nil:
		issue("no_commands", "", "Build/test belum dijalankan")
	case len(lastBuild.Steps) == 0:
		issue("no_commands", "", lastBuild.Reason)
	default:
		buildOK, testOK = true, false
		for _, s := range lastBuild.Steps {
			if s.Name == "test" {
				testOK = s.ExitCode == 0
			}
			if s.ExitCode != 0 && s.Name == "smoke" {
				issue("smoke_failed", lastBuild.ID, "Aplikasi tidak berjalan: "+s.Command)
				continue
			}
			if s.ExitCode != 0 && s.Name != "test" {
				buildOK = false
				issue("build_failed", lastBuild.ID, fmt.Sprintf("%s gagal: %s", s.Name, s.Command))
			}
			if s.ExitCode != 0 && s.Name == "test" {
				issue("test_failed", lastBuild.ID, "Test gagal: "+s.Command)
			}
		}
		hasTest := false
		for _, s := range lastBuild.Steps {
			hasTest = hasTest || s.Name == "test"
		}
		if !hasTest && lastBuild.Passed { // all steps ran and none was a test
			issue("missing_test", "", "Project tidak punya command test")
		}
	}

	// The apps must really start: a green build and test does not prove a
	// page renders, the API answers or the worker connects. A project that
	// can be started needs a smoke script that starts it and probes it.
	smokeOK := true
	if runnable(p) {
		smokeOK = false
		for _, s := range stepsOf(lastBuild) {
			if s.Name == "smoke" {
				smokeOK = s.ExitCode == 0
			}
		}
		if !smokeOK && countKind(v.Issues, "smoke_failed") == 0 && lastBuild != nil && len(lastBuild.Steps) > 0 {
			issue("no_smoke", "", "Belum ada script smoke yang menjalankan semua app (web, API, worker) dan memeriksanya")
		}
	}

	doneAll := open == 0 && blocked == 0 && len(p.Tasks) > 0
	designOK := p.Design != nil && p.Design.Status == core.DesignApproved
	v.Checks = []core.Check{
		{Name: "Requirements selesai", OK: len(p.Requirements) > 0 && countKind(v.Issues, "missing_feature") == 0, Info: fmt.Sprintf("%d requirement", len(p.Requirements))},
		{Name: "Design selesai", OK: designOK && countKind(v.Issues, "missing_page")+countKind(v.Issues, "broken_flow") == 0},
		{Name: "Semua task selesai", OK: doneAll, Info: fmt.Sprintf("%d terbuka, %d terblokir", open, blocked)},
		{Name: "Build berhasil", OK: buildOK},
		{Name: "Test berhasil", OK: testOK},
		{Name: "Aplikasi berjalan", OK: smokeOK},
	}
	v.Ready = len(v.Issues) == 0
	for _, c := range v.Checks {
		v.Ready = v.Ready && c.OK
	}
	v.Checks = append(v.Checks, core.Check{Name: "Verification berhasil", OK: v.Ready})
	return v
}

func countKind(is []core.Issue, kind string) int {
	n := 0
	for _, i := range is {
		if i.Kind == kind {
			n++
		}
	}
	return n
}

// runnable reports whether the project has something to start (a dev or
// start script). Projects without a directory are never runnable.
func runnable(p *core.Project) bool {
	return p.Dir != "" && len(scan.Scan(p.Dir).Run) > 0
}

func stepsOf(b *core.BuildRun) []core.StepResult {
	if b == nil {
		return nil
	}
	return b.Steps
}
