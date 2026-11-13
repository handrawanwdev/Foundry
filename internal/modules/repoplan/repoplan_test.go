package repoplan_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
)

// project builds a new project with the given modules and structure
// decision, the way discovery and the structure workspace leave it.
func project(modules []string, ir *core.StructureIR) *core.Project {
	bp := &core.Blueprint{}
	for _, m := range modules {
		bp.Modules = append(bp.Modules, core.Module{ID: m, Responsibility: "Tanggung jawab " + m})
	}
	structure.Normalize(ir)
	return &core.Project{Name: "demo", Story: "cerita", Blueprint: bp,
		Structure: &core.Structure{Mode: core.StructureNew, IR: ir, Status: core.TechApproved, Approved: true}}
}

func dir(path, owner, resp string) core.RepoNode {
	return core.RepoNode{Path: path, Type: core.NodeDir, Owner: owner, Responsibility: resp, Reason: "Dibutuhkan untuk " + resp,
		Allowed: []string{"kode " + resp}, Forbidden: []string{"kode di luar " + resp}}
}

func file(path, owner, resp string) core.RepoNode {
	return core.RepoNode{Path: path, Type: core.NodeFile, Owner: owner, Responsibility: resp, Reason: "Dibutuhkan untuk " + resp}
}

func evaluated(plan *core.RepoPlan) {
	for _, c := range core.RepoCriteria {
		plan.Evaluation.Criteria = append(plan.Evaluation.Criteria, core.RepoCriterion{Name: c, Score: 3, Reason: "Alasan konkret " + c})
	}
	for _, a := range core.RepoConsistencyAreas {
		plan.Consistency = append(plan.Consistency, core.RepoCheck{Area: a, Status: core.CheckConsistent, Note: "ok"})
	}
	plan.Approach, plan.Topology, plan.Summary = "Modular per fitur", "Satu repo, dua aplikasi", "Ringkasan"
	plan.Decisions = []core.RepoDecision{{Title: "Satu repo", Choice: "monorepo", Reason: "tim kecil"}}
	plan.Dependency.Direction = "UI → API → modul → shared"
}

// monolith is a web app and one API whose modules are the business domains.
func monolith() (*core.Project, *core.RepoPlan) {
	p := project([]string{"booking", "billing"}, &core.StructureIR{Type: "Modular monolith",
		Apps:     []core.StructurePart{{Name: "web", Reason: "web"}},
		Services: []core.StructurePart{{Name: "api", Reason: "api", Modules: []string{"booking", "billing"}, ModuleReasons: map[string]string{"booking": "b", "billing": "c"}}},
		Packages: []core.StructurePart{{Name: "contracts", Reason: "kontrak"}}})
	web := dir("apps/web/", core.OwnerPlatform, "aplikasi web")
	web.Part = "app:web"
	api := dir("apps/api/", core.OwnerPlatform, "API")
	api.Part = "service:api"
	booking := dir("apps/api/modules/booking/", "booking", "domain pemesanan")
	booking.Part, booking.DependsOn = "module:booking", []string{"packages/contracts/"}
	billing := dir("apps/api/modules/billing/", "billing", "domain tagihan")
	billing.Part, billing.DependsOn = "module:billing", []string{"packages/contracts/", "apps/api/modules/booking/index.ts"}
	contracts := dir("packages/contracts/", core.OwnerShared, "kontrak API dan event")
	contracts.Part, contracts.SharedBy = "package:contracts", []string{"booking", "billing"}
	plan := &core.RepoPlan{
		Nodes: []core.RepoNode{
			dir("apps/", core.OwnerPlatform, "semua aplikasi"), web, api, dir("apps/api/modules/", core.OwnerPlatform, "modul bisnis API"),
			booking, file("apps/api/modules/booking/index.ts", "booking", "public interface modul booking"), billing,
			dir("packages/", core.OwnerPlatform, "kode bersama"), contracts, dir("docs/", core.OwnerPlatform, "dokumentasi"),
		},
		Modules: []core.RepoModuleMap{
			{Module: "booking", Locations: []core.RepoLocation{{Path: "apps/api/modules/booking/", Role: "api"}, {Path: "apps/web/", Role: "ui"}}, Frontend: true, Backend: true},
			{Module: "billing", Locations: []core.RepoLocation{{Path: "apps/api/modules/billing/", Role: "api"}}, Backend: true},
		},
		Dependency: core.RepoDependency{
			Allowed:    []core.RepoDepRule{{From: "billing", To: "booking", Via: "apps/api/modules/booking/index.ts"}, {From: "booking", To: "shared"}, {From: "billing", To: "shared"}},
			Forbidden:  []core.RepoDepRule{{From: "booking", To: "billing"}, {From: "shared", To: "booking"}},
			Interfaces: []core.RepoInterface{{Module: "booking", Path: "apps/api/modules/booking/index.ts"}},
		},
	}
	evaluated(plan)
	return p, plan
}

func errorsOf(p *core.Project, plan *core.RepoPlan) []string {
	repoplan.Normalize(plan)
	repoplan.Refresh(p, plan)
	var out []string
	for _, e := range plan.Errors() {
		out = append(out, e.Message)
	}
	return out
}

func hasError(errs []string, part string) bool {
	return slices.ContainsFunc(errs, func(e string) bool { return strings.Contains(e, part) })
}

func TestValidMonolith(t *testing.T) {
	p, plan := monolith()
	if errs := errorsOf(p, plan); len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	if plan.Evaluation.Score != 50 {
		t.Fatalf("score = %d, want 50 for all 3s", plan.Evaluation.Score)
	}
	if plan.Analysis.Dirs != 9 || plan.Analysis.Files != 1 {
		t.Fatalf("dirs=%d files=%d", plan.Analysis.Dirs, plan.Analysis.Files)
	}
	if len(plan.Analysis.Edges) == 0 || len(plan.Analysis.Units) < 3 {
		t.Fatalf("diagram: units=%v edges=%v", plan.Analysis.Units, plan.Analysis.Edges)
	}
	// Foundry verifies its own checks next to the AI's.
	var computed int
	for _, c := range plan.Analysis.Checks {
		if c.Source == "computed" {
			computed++
			if c.Status != core.CheckConsistent {
				t.Fatalf("check %+v", c)
			}
		}
	}
	if computed != 4 {
		t.Fatalf("computed checks = %d", computed)
	}
}

func TestNormalizeCleansPaths(t *testing.T) {
	_, plan := monolith()
	plan.Nodes = append(plan.Nodes,
		core.RepoNode{Path: `src\deep\thing.go`, Responsibility: "x", Reason: "y", Owner: "booking"},                   // backslashes, type inferred
		core.RepoNode{Path: "../escape/", Responsibility: "x", Reason: "y", Owner: "booking"},                          // refused
		core.RepoNode{Path: "docs/", Type: "folder", Owner: "platform", Responsibility: "dokumen revisi", Reason: "r"}, // duplicate: last wins
	)
	repoplan.Normalize(plan)
	paths := map[string]core.RepoNode{}
	for _, n := range plan.Nodes {
		paths[n.Path] = n
	}
	if _, ok := paths["../escape/"]; ok || paths["src/deep/thing.go"].Type != core.NodeFile {
		t.Fatalf("paths = %v", paths)
	}
	if paths["docs/"].Responsibility != "dokumen revisi" {
		t.Fatalf("duplicate not resolved: %+v", paths["docs/"])
	}
	for _, parent := range []string{"src/", "src/deep/"} {
		if n := paths[parent]; !n.Implied || n.Type != core.NodeDir {
			t.Fatalf("parent %s = %+v", parent, n)
		}
	}
	if !slices.IsSortedFunc(plan.Nodes, func(a, b core.RepoNode) int { return strings.Compare(a.Path, b.Path) }) {
		t.Fatal("nodes not in tree order")
	}
}

func TestNormalizeHealsFileLocationsAndDuplicateParts(t *testing.T) {
	p := project([]string{"identity", "task"}, &core.StructureIR{Type: "Modular monolith",
		Apps: []core.StructurePart{{Name: "web", Reason: "web"}},
		Services: []core.StructurePart{{Name: "api", Reason: "api", Modules: []string{"identity", "task"},
			ModuleReasons: map[string]string{"identity": "akun", "task": "tugas"}}}})
	web := dir("apps/web/", core.OwnerPlatform, "aplikasi web")
	web.Part = "app:web"
	api := dir("apps/api/", core.OwnerPlatform, "API")
	api.Part = "service:api"
	ident := dir("apps/api/modules/identity/", "identity", "akun")
	ident.Part = "module:identity"
	taskd := dir("apps/api/modules/task/", "task", "tugas")
	taskd.Part = "module:task"
	auth := dir("apps/web/src/features/auth/", "identity", "UI autentikasi")
	auth.Part = "module:identity"
	tasks := dir("apps/web/src/features/tasks/", "task", "UI tugas")
	tasks.Part = "module:task"
	schema := file("apps/api/modules/identity/schema.ts", "identity", "skema akun")
	schema.Part = "module:identity"
	plan := &core.RepoPlan{
		Nodes: []core.RepoNode{
			dir("apps/", core.OwnerPlatform, "semua aplikasi"), web, api,
			dir("apps/api/modules/", core.OwnerPlatform, "modul bisnis API"),
			ident, schema, file("apps/api/modules/identity/password.ts", "identity", "hash password"),
			taskd, file("apps/api/modules/task/schema.ts", "task", "skema tugas"),
			file("apps/api/modules/task/service.ts", "task", "layanan tugas"),
			dir("apps/web/src/", core.OwnerPlatform, "sumber web"),
			dir("apps/web/src/features/", core.OwnerPlatform, "fitur UI"),
			auth, tasks,
		},
		Modules: []core.RepoModuleMap{
			{Module: "identity", Frontend: true, Backend: true, Locations: []core.RepoLocation{
				{Path: "apps/api/modules/identity/schema.ts", Role: "api"},
				{Path: "apps/api/modules/identity/password.ts", Role: "api"},
				{Path: "apps/web/src/features/auth/", Role: "ui"},
			}},
			{Module: "task", Frontend: true, Backend: true, Locations: []core.RepoLocation{
				{Path: "apps/api/modules/task/schema.ts", Role: "api"},
				{Path: "apps/api/modules/task/service.ts", Role: "api"},
				{Path: "apps/web/src/features/tasks/", Role: "ui"},
			}},
		},
		Dependency: core.RepoDependency{
			Interfaces: []core.RepoInterface{{Module: "identity", Path: "apps/api/modules/identity/schema.ts/"}},
		},
	}
	evaluated(plan)
	if err := repoplan.Accept(p, plan); err != nil {
		t.Fatalf("typical architect answer refused: %v", err)
	}

	locs := func(id string) []string {
		for _, m := range plan.Modules {
			if m.Module == id {
				var out []string
				for _, l := range m.Locations {
					out = append(out, l.Path)
				}
				return out
			}
		}
		return nil
	}
	if got := locs("identity"); !slices.Equal(got, []string{"apps/api/modules/identity/", "apps/web/src/features/auth/"}) {
		t.Fatalf("identity locations = %v", got)
	}
	if got := locs("task"); !slices.Equal(got, []string{"apps/api/modules/task/", "apps/web/src/features/tasks/"}) {
		t.Fatalf("task locations = %v", got)
	}
	if n := plan.Node("apps/api/modules/identity/"); n == nil || n.Part != "module:identity" {
		t.Fatalf("identity part = %+v", n)
	}
	if n := plan.Node("apps/web/src/features/auth/"); n == nil || n.Part != "" {
		t.Fatalf("auth still carries module part: %+v", n)
	}
	if n := plan.Node("apps/api/modules/task/"); n == nil || n.Part != "module:task" {
		t.Fatalf("task part = %+v", n)
	}
	if n := plan.Node("apps/web/src/features/tasks/"); n == nil || n.Part != "" {
		t.Fatalf("tasks UI still carries module part: %+v", n)
	}
	if n := plan.Node("apps/api/modules/identity/schema.ts"); n == nil || n.Part != "" {
		t.Fatalf("file still carries part: %+v", n)
	}
	if len(plan.Dependency.Interfaces) != 1 || plan.Dependency.Interfaces[0].Path != "apps/api/modules/identity/schema.ts" {
		t.Fatalf("interface = %+v", plan.Dependency.Interfaces)
	}
}

func TestNormalizeDedupsPartPreferringMatchingOwner(t *testing.T) {
	p, plan := monolith()
	plan.Nodes[len(plan.Nodes)-1].Part = "app:web" // docs/, owner platform
	if errs := errorsOf(p, plan); len(errs) != 0 {
		t.Fatalf("duplicate part not healed: %v", errs)
	}
	if n := plan.Node("apps/web/"); n == nil || n.Part != "app:web" {
		t.Fatalf("canonical app:web lost: %+v", n)
	}
	if n := plan.Node("docs/"); n == nil || n.Part != "" {
		t.Fatalf("extra part stamp remains: %+v", n)
	}
}

func TestValidationRefusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(plan *core.RepoPlan)
		want   string
	}{
		{"module without location", func(pl *core.RepoPlan) { pl.Modules = pl.Modules[:1] }, "belum punya lokasi di repository: billing"},
		{"location not in tree", func(pl *core.RepoPlan) {
			pl.Modules[1].Locations = []core.RepoLocation{{Path: "apps/api/modules/ghost/"}}
		}, "tidak ada di struktur folder"},
		{"module with neither side and no note", func(pl *core.RepoPlan) {
			pl.Modules[1] = core.RepoModuleMap{Module: "billing"}
		}, "tanpa lokasi dan tanpa alasan"},
		{"unknown module mapped", func(pl *core.RepoPlan) {
			pl.Modules = append(pl.Modules, core.RepoModuleMap{Module: "ghost", Backend: true, Locations: pl.Modules[0].Locations})
		}, "bukan Project Module"},
		{"ir part not mapped", func(pl *core.RepoPlan) { pl.Nodes[1].Part = "" }, "belum dipetakan ke folder: app:web"},
		{"unknown part", func(pl *core.RepoPlan) { pl.Nodes[0].Part = "service:ghost" }, "tidak ada di keputusan struktur"},
		{"no responsibility", func(pl *core.RepoPlan) { pl.Nodes[0].Responsibility = "" }, "tanpa responsibility"},
		{"no reason", func(pl *core.RepoPlan) { pl.Nodes[0].Reason = "" }, "tanpa alasan keberadaan"},
		{"bad owner", func(pl *core.RepoPlan) { pl.Nodes[0].Owner = "nobody" }, "owner yang bukan modul project"},
		{"dependency on missing path", func(pl *core.RepoPlan) { pl.Nodes[4].DependsOn = []string{"nowhere/x.go"} }, "bukan modul atau path"},
		{"forbidden edge used", func(pl *core.RepoPlan) {
			for i := range pl.Nodes {
				if pl.Nodes[i].Path == "apps/api/modules/booking/" {
					pl.Nodes[i].DependsOn = append(pl.Nodes[i].DependsOn, "apps/api/modules/billing/")
				}
			}
		}, "dilarang oleh aturan booking → billing"},
		{"allowed contradicts forbidden", func(pl *core.RepoPlan) {
			pl.Dependency.Allowed = append(pl.Dependency.Allowed, core.RepoDepRule{From: "booking", To: "billing"})
		}, "bertentangan dengan aturan forbidden"},
		{"no direction", func(pl *core.RepoPlan) { pl.Dependency.Direction = "" }, "arah dependency belum dijelaskan"},
		{"criterion missing", func(pl *core.RepoPlan) { pl.Evaluation.Criteria = pl.Evaluation.Criteria[1:] }, "belum menilai (skor 1-5): cohesion"},
		{"criterion without reason", func(pl *core.RepoPlan) { pl.Evaluation.Criteria[2].Reason = "" }, "skor tanpa alasan konkret: ownership"},
		{"consistency missing", func(pl *core.RepoPlan) { pl.Consistency = pl.Consistency[1:] }, "belum mencakup: architecture_identity"},
		{"consistency conflict", func(pl *core.RepoPlan) {
			pl.Consistency[3] = core.RepoCheck{Area: "technology", Status: core.CheckConflict, Note: "Go vs TypeScript"}
		}, "konflik technology belum diselesaikan: Go vs TypeScript"},
		{"no decisions", func(pl *core.RepoPlan) { pl.Decisions = nil }, "keputusan desain belum dicatat"},
		{"interface unknown path", func(pl *core.RepoPlan) {
			pl.Dependency.Interfaces = append(pl.Dependency.Interfaces, core.RepoInterface{Module: "billing", Path: "nope.ts"})
		}, "public interface nope.ts"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, plan := monolith()
			c.mutate(plan)
			if errs := errorsOf(p, plan); !hasError(errs, c.want) {
				t.Fatalf("errors = %v, want one containing %q", errs, c.want)
			}
		})
	}
}

func TestCircularDependencies(t *testing.T) {
	p, plan := monolith()
	// billing already uses booking; booking → billing closes the cycle. The
	// forbidden rule is dropped so the cycle itself is what fails.
	plan.Dependency.Forbidden = nil
	for i := range plan.Nodes {
		if plan.Nodes[i].Path == "apps/api/modules/booking/" {
			plan.Nodes[i].DependsOn = append(plan.Nodes[i].DependsOn, "apps/api/modules/billing/")
		}
	}
	errs := errorsOf(p, plan)
	if !hasError(errs, "dependensi melingkar antar-unit: billing ↔ booking") {
		t.Fatalf("errors = %v", errs)
	}
	found := false
	for _, f := range plan.Analysis.Findings {
		found = found || f.Category == "circular-dependency" && f.Severity == "error"
	}
	if !found {
		t.Fatalf("no circular finding: %+v", plan.Analysis.Findings)
	}

	// An event breaks the cycle: communication by event is not coupling.
	p, plan = monolith()
	plan.Dependency.Forbidden = nil
	plan.Dependency.Communication = []core.RepoDepRule{{From: "booking", To: "billing", Via: "booking.confirmed", Mode: core.DepEvent}}
	if errs := errorsOf(p, plan); len(errs) != 0 {
		t.Fatalf("event link treated as a cycle: %v", errs)
	}

	// A synchronous link is.
	plan.Dependency.Communication[0].Mode = core.DepSync
	if errs := errorsOf(p, plan); !hasError(errs, "dependensi melingkar") {
		t.Fatalf("sync link accepted: %v", errs)
	}
}

func TestSharedAndOwnershipWarnings(t *testing.T) {
	p, plan := monolith()
	// billing's code inside booking's folder: ownership is mixed.
	plan.Nodes = append(plan.Nodes, file("apps/api/modules/booking/invoice.ts", "billing", "faktur"))
	// Two modules claim one folder without saying it is shared.
	plan.Modules[1].Locations = append(plan.Modules[1].Locations, core.RepoLocation{Path: "apps/web/"})
	errorsOf(p, plan)
	var warns []string
	for _, i := range plan.Analysis.Issues {
		if i.Level == "warning" {
			warns = append(warns, i.Message)
		}
	}
	if !hasError(warns, "pemiliknya bercampur") || !hasError(warns, "dipakai modul booking, billing") {
		t.Fatalf("warnings = %v", warns)
	}
}

func TestPublicInterfaceBypass(t *testing.T) {
	p, plan := monolith()
	for i := range plan.Nodes {
		if plan.Nodes[i].Path == "apps/api/modules/billing/" {
			plan.Nodes[i].DependsOn = []string{"apps/api/modules/booking/"} // folder, not the interface file
		}
	}
	plan.Nodes = append(plan.Nodes, dir("apps/api/modules/booking/internal/", "booking", "isi privat booking"))
	for i := range plan.Nodes {
		if plan.Nodes[i].Path == "apps/api/modules/billing/" {
			plan.Nodes[i].DependsOn = []string{"apps/api/modules/booking/internal/"}
		}
	}
	errorsOf(p, plan)
	var warn bool
	for _, i := range plan.Analysis.Issues {
		warn = warn || i.Level == "warning" && strings.Contains(i.Message, "melewati public interface")
	}
	if !warn {
		t.Fatalf("issues = %+v", plan.Analysis.Issues)
	}
}

func TestComputedFindings(t *testing.T) {
	p, plan := monolith()
	plan.Nodes = append(plan.Nodes,
		dir("apps/api/modules/booking/utils/", "booking", "kode pembantu pemesanan secara umum"),
		dir("apps/api/modules/billing/helpers/", "billing", "kode pembantu pemesanan secara umum"),
		dir("tools/", core.OwnerPlatform, "alat bantu pengembangan"),
		dir("tools/scripts/", core.OwnerPlatform, "skrip pengembangan"),
	)
	errorsOf(p, plan)
	got := map[string]bool{}
	for _, f := range plan.Analysis.Findings {
		got[f.Category] = true
	}
	for _, want := range []string{"poor-naming", "duplication", "unnecessary-directory"} {
		if !got[want] {
			t.Errorf("finding %s missing: %+v", want, plan.Analysis.Findings)
		}
	}
}

func TestOpsRewriteReferences(t *testing.T) {
	p, plan := monolith()
	repoplan.Normalize(plan)
	changes, err := repoplan.Apply(plan, []core.RepoOp{{Op: repoplan.OpMoveNode, Path: "apps/api/modules/booking/", To: "apps/api/domains/booking/"}})
	if err != nil || len(changes) != 1 {
		t.Fatalf("move: %v %v", changes, err)
	}
	moved := plan.Node("apps/api/domains/booking/")
	if moved == nil || plan.Node("apps/api/domains/booking/index.ts") == nil || plan.Node("apps/api/modules/booking/") != nil {
		t.Fatalf("subtree not moved: %+v", plan.Nodes)
	}
	if plan.Modules[0].Locations[0].Path != "apps/api/domains/booking/" || plan.Dependency.Interfaces[0].Path != "apps/api/domains/booking/index.ts" {
		t.Fatalf("references not rewritten: %+v %+v", plan.Modules[0], plan.Dependency.Interfaces)
	}
	var billing *core.RepoNode = plan.Node("apps/api/modules/billing/")
	if !slices.Contains(billing.DependsOn, "apps/api/domains/booking/index.ts") {
		t.Fatalf("depends_on = %v", billing.DependsOn)
	}
	if plan.Node("apps/api/modules/") == nil {
		t.Fatal("billing still lives in modules/")
	}
	// The new parent folder exists, described by Foundry until the AI does.
	if d := plan.Node("apps/api/domains/"); d == nil || !d.Implied {
		t.Fatalf("domains/ = %+v", d)
	}
	if errs := errorsOf(p, plan); len(errs) != 0 {
		t.Fatalf("plan broken after move: %v", errs)
	}

	// Removing a folder takes everything below it and the references.
	changes, err = repoplan.Apply(plan, []core.RepoOp{{Op: repoplan.OpRemoveNode, Path: "apps/api/domains/booking"}})
	if err != nil || !strings.Contains(changes[0], "2 node") {
		t.Fatalf("remove: %v %v", changes, err)
	}
	if len(plan.Dependency.Interfaces) != 0 || len(plan.Modules[0].Locations) != 1 {
		t.Fatalf("references remain: %+v %+v", plan.Dependency.Interfaces, plan.Modules[0])
	}
	// The removed module's location is gone, so the plan now says so.
	plan.Nodes = slices.DeleteFunc(plan.Nodes, func(n core.RepoNode) bool { return n.Path == "apps/web/" })
	plan.Modules[0].Locations = nil
	repoplan.Apply(plan, []core.RepoOp{{Op: repoplan.OpAddRule, Text: "Tiap modul hanya diakses lewat public interface"}})
	if errs := errorsOf(p, plan); !hasError(errs, "booking") {
		t.Fatalf("errors = %v", errs)
	}
}

func TestOpsRefusals(t *testing.T) {
	_, plan := monolith()
	repoplan.Normalize(plan)
	for _, op := range []core.RepoOp{
		{Op: repoplan.OpRemoveNode, Path: "ghost/"},
		{Op: repoplan.OpMoveNode, Path: "docs/", To: "apps/"},
		{Op: repoplan.OpMoveNode, Path: "apps/", To: "apps/api/x/"},
		{Op: repoplan.OpSetNode},
		{Op: repoplan.OpSetNode, Node: &core.RepoNode{Path: "../x"}},
		{Op: repoplan.OpAddDependency, Kind: "sometimes", Dep: &core.RepoDepRule{From: "a", To: "b"}},
		{Op: repoplan.OpRemoveRule, Text: "tidak ada"},
		{Op: "explode"},
	} {
		if _, err := repoplan.Apply(plan.Clone(), []core.RepoOp{op}); err == nil {
			t.Errorf("op %+v accepted", op)
		}
	}
	if _, err := repoplan.Apply(plan.Clone(), nil); err == nil {
		t.Error("empty ops accepted")
	}
}

func TestOpsAddDependencyAndMeta(t *testing.T) {
	_, plan := monolith()
	repoplan.Normalize(plan)
	c := plan.Clone()
	_, err := repoplan.Apply(c, []core.RepoOp{
		{Op: repoplan.OpAddDependency, Kind: "forbidden", Dep: &core.RepoDepRule{From: "billing", To: "apps/web/", Reason: "API tidak memakai UI"}},
		{Op: repoplan.OpSetMeta, Approach: "Fitur vertikal"},
		{Op: repoplan.OpSetNode, Node: &core.RepoNode{Path: "apps/api/modules/billing/policy.ts", Owner: "billing", Responsibility: "aturan tagihan", Reason: "isolasi aturan"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Approach != "Fitur vertikal" || len(c.Dependency.Forbidden) != 3 || c.Node("apps/api/modules/billing/policy.ts") == nil {
		t.Fatalf("plan = %+v", c)
	}
	if plan.Approach == "Fitur vertikal" {
		t.Fatal("the clone shares state with the original")
	}
}

func TestStaleAreasPointAtAffectedSections(t *testing.T) {
	p, plan := monolith()
	repoplan.Normalize(plan)
	repoplan.MarkCurrent(p, plan)
	repoplan.Refresh(p, plan)
	if plan.Stale || len(plan.Analysis.StaleAreas) != 0 {
		t.Fatalf("fresh plan is stale: %v", plan.Analysis.StaleAreas)
	}
	p.Blueprint.Modules = append(p.Blueprint.Modules, core.Module{ID: "reporting", Responsibility: "laporan"})
	repoplan.Refresh(p, plan)
	if !plan.Stale || !slices.Equal(plan.Analysis.StaleAreas, []string{"modules"}) {
		t.Fatalf("stale areas = %v", plan.Analysis.StaleAreas)
	}
	if !slices.Contains(plan.Analysis.Affected, "modules") || !slices.Contains(plan.Analysis.Affected, "evaluation") || slices.Contains(plan.Analysis.Affected, "rules") {
		t.Fatalf("affected = %v", plan.Analysis.Affected)
	}
	if !hasError(errorsOf(p, plan), "reporting") {
		t.Fatalf("the new module is not reported as unplaced")
	}
	// A change of the structure decision is its own area.
	p, plan = monolith()
	repoplan.MarkCurrent(p, plan)
	p.Structure.IR.Packages = nil
	repoplan.Refresh(p, plan)
	if !slices.Equal(plan.Analysis.StaleAreas, []string{"structure"}) {
		t.Fatalf("stale areas = %v", plan.Analysis.StaleAreas)
	}
}

func TestAcceptRefusesWithConcreteReasonsButKeepsReportedConflicts(t *testing.T) {
	p, plan := monolith()
	plan.Modules = nil
	err := repoplan.Accept(p, plan)
	if err == nil || !strings.Contains(err.Error(), "belum punya lokasi di repository: booking, billing") {
		t.Fatalf("err = %v", err)
	}
	p, plan = monolith()
	plan.Consistency[3] = core.RepoCheck{Area: "technology", Status: core.CheckConflict, Note: "belum terselesaikan"}
	if err := repoplan.Accept(p, plan); err != nil {
		t.Fatalf("a reported conflict is not a refusal: %v", err)
	}
	if len(plan.Errors()) != 1 {
		t.Fatalf("errors = %v", plan.Errors())
	}
	// It cannot be approved until resolved.
	if err := repoplan.Approve(p, plan); err == nil || !strings.Contains(err.Error(), "konflik technology") {
		t.Fatalf("approve = %v", err)
	}
}

func TestApproveNeedsApprovedStructureAndFreshSources(t *testing.T) {
	p, plan := monolith()
	repoplan.Accept(p, plan)
	p.Structure.SetStatus(core.TechChanged)
	if err := repoplan.Approve(p, plan); err != repoplan.ErrNeedStructure {
		t.Fatalf("approve = %v", err)
	}
	p.Structure.SetStatus(core.TechApproved)
	if err := repoplan.Approve(p, plan); err != nil || plan.State() != core.TechApproved {
		t.Fatalf("approve = %v, state %s", err, plan.State())
	}
	if err := repoplan.Ready(p); err != nil {
		t.Fatalf("ready = %v", err)
	}
	p.Structure.Repo = plan
	p.Blueprint.Architecture.Pattern = "Hexagonal"
	if err := repoplan.Ready(p); err == nil || !strings.Contains(err.Error(), "kedaluwarsa") {
		t.Fatalf("stale plan is ready: %v", err)
	}
	// Editing an approved plan puts it back under review.
	repoplan.Changed(plan)
	if plan.State() != core.TechChanged {
		t.Fatalf("state = %s", plan.State())
	}
}

func TestTechPlanExtrasChangeTechnologyHash(t *testing.T) {
	p, _ := monolith()
	p.Stack = &core.TechStack{Decisions: []core.TechDecision{{Area: core.TechBackend, Recommended: "Go + Gin"}}}
	h1 := repoplan.SourceAreas(p)["technology"]
	p.Stack.Plan = &core.TechPlan{Items: []core.TechItem{{Area: "orm", Technology: "GORM", Language: "Go"}}}
	h2 := repoplan.SourceAreas(p)["technology"]
	if h1 == "" || h1 == h2 {
		t.Fatalf("extras of the technology plan did not change the repository source hash: %s → %s", h1, h2)
	}
}

func TestEvaluationScore(t *testing.T) {
	_, plan := monolith()
	repoplan.Normalize(plan)
	if plan.Evaluation.Score != 50 {
		t.Fatalf("score = %d", plan.Evaluation.Score)
	}
	for i := range plan.Evaluation.Criteria {
		plan.Evaluation.Criteria[i].Score = 5
		if plan.Evaluation.Criteria[i].Name == "complexity" {
			plan.Evaluation.Criteria[i].Score = 1
		}
	}
	repoplan.Normalize(plan)
	if plan.Evaluation.Score != 100 {
		t.Fatalf("best score = %d", plan.Evaluation.Score)
	}
	plan.Evaluation.Criteria = plan.Evaluation.Criteria[:3]
	repoplan.Normalize(plan)
	if plan.Evaluation.Score != 0 {
		t.Fatalf("incomplete evaluation scores %d", plan.Evaluation.Score)
	}
}

// worker is a plan where a worker folder has an implied lib/ folder that only
// holds one file, the shape of the revision that failed on roda-sense.
func worker() *core.RepoPlan {
	plan := &core.RepoPlan{Nodes: []core.RepoNode{
		dir("apps/", core.OwnerPlatform, "aplikasi"),
		dir("apps/mobile/", core.OwnerPlatform, "aplikasi mobile"),
		dir("apps/mobile/src/", core.OwnerPlatform, "kode aplikasi"),
		dir("apps/read-model-rebuild/", core.OwnerPlatform, "worker rebuild"),
		file("apps/read-model-rebuild/lib/read_model_rebuild.dart", core.OwnerPlatform, "pemicu rebuild"),
		file("apps/read-model-rebuild/lib/checkpoint.dart", core.OwnerPlatform, "checkpoint"),
	}}
	repoplan.Normalize(plan)
	return plan
}

func TestOpsSeeEarlierOpsOfTheSameRevision(t *testing.T) {
	plan := worker()
	if n := plan.Node("apps/read-model-rebuild/lib/"); n == nil || !n.Implied {
		t.Fatalf("lib/ = %+v", n)
	}
	// Exactly what the AI sent: move the worker, move both files out of the
	// implied lib/ (Foundry drops the emptied folder), then remove lib/.
	changes, err := repoplan.Apply(plan, []core.RepoOp{
		{Op: repoplan.OpSetNode, Node: &core.RepoNode{Path: "apps/mobile/src/workers/", Type: "folder", Owner: core.OwnerPlatform, Responsibility: "job in-process", Reason: "worker"}},
		{Op: repoplan.OpMoveNode, Path: "apps/read-model-rebuild/", To: "apps/mobile/src/workers/read-model-rebuild/"},
		{Op: repoplan.OpMoveNode, Path: "apps/mobile/src/workers/read-model-rebuild/lib/read_model_rebuild.dart", To: "apps/mobile/src/workers/read-model-rebuild/index.ts"},
		{Op: repoplan.OpMoveNode, Path: "apps/mobile/src/workers/read-model-rebuild/lib/checkpoint.dart", To: "apps/mobile/src/workers/read-model-rebuild/checkpoint.ts"},
		{Op: repoplan.OpRemoveNode, Path: "apps/mobile/src/workers/read-model-rebuild/lib/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(changes[4], "Lewati") || !strings.Contains(changes[4], "perubahan 4") {
		t.Fatalf("remove of the emptied folder = %q", changes[4])
	}
	if plan.Node("apps/mobile/src/workers/read-model-rebuild/index.ts") == nil || plan.Node("apps/mobile/src/workers/read-model-rebuild/lib/") != nil || plan.Node("apps/read-model-rebuild/") != nil {
		t.Fatalf("nodes = %+v", plan.Nodes)
	}
}

func TestOpsFollowMovesAndRefuseGonePaths(t *testing.T) {
	p, plan := monolith()
	repoplan.Normalize(plan)
	// Later ops name the old place of a moved folder: they mean the node at
	// its new place, references included.
	changes, err := repoplan.Apply(plan, []core.RepoOp{
		{Op: repoplan.OpMoveNode, Path: "apps/api/modules/booking/", To: "apps/api/domains/booking/"},
		{Op: repoplan.OpSetInterface, Interface: &core.RepoInterface{Module: "booking", Path: "apps/api/modules/booking/index.ts", Reason: "pintu masuk"}},
		{Op: repoplan.OpSetModule, Module: &core.RepoModuleMap{Module: "booking", Locations: []core.RepoLocation{{Path: "apps/api/modules/booking/", Role: "api"}, {Path: "apps/web/", Role: "ui"}}, Frontend: true, Backend: true}},
		{Op: repoplan.OpAddDependency, Kind: "allowed", Dep: &core.RepoDepRule{From: "apps/api/modules/billing/", To: "apps/api/modules/booking/index.ts"}},
		{Op: repoplan.OpMoveNode, Path: "apps/api/modules/booking/index.ts", To: "apps/api/domains/booking/public.ts"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(changes[4], "ditulis sebagai apps/api/modules/booking/index.ts") {
		t.Fatalf("move through the earlier move = %q", changes[4])
	}
	if plan.Node("apps/api/domains/booking/public.ts") == nil {
		t.Fatalf("nodes = %+v", plan.Nodes)
	}
	if plan.Modules[0].Locations[0].Path != "apps/api/domains/booking/" {
		t.Fatalf("module location = %+v", plan.Modules[0].Locations)
	}
	if !slices.ContainsFunc(plan.Dependency.Interfaces, func(i core.RepoInterface) bool { return i.Path == "apps/api/domains/booking/public.ts" }) {
		t.Fatalf("interfaces = %+v", plan.Dependency.Interfaces)
	}
	if !slices.ContainsFunc(plan.Dependency.Allowed, func(r core.RepoDepRule) bool { return r.To == "apps/api/domains/booking/public.ts" }) {
		t.Fatalf("allowed = %+v", plan.Dependency.Allowed)
	}
	if errs := errorsOf(p, plan); len(errs) != 0 {
		t.Fatalf("plan broken: %v", errs)
	}

	// A removed folder is gone for every later op: removing it (or below it)
	// again is a no-op, moving it is refused.
	_, plan = monolith()
	repoplan.Normalize(plan)
	changes, err = repoplan.Apply(plan.Clone(), []core.RepoOp{
		{Op: repoplan.OpRemoveNode, Path: "apps/api/modules/booking/"},
		{Op: repoplan.OpRemoveNode, Path: "apps/api/modules/booking/index.ts"},
	})
	if err != nil || !strings.Contains(changes[1], "sejak perubahan 1") {
		t.Fatalf("remove below a removed folder: %v %v", changes, err)
	}
	_, err = repoplan.Apply(plan.Clone(), []core.RepoOp{
		{Op: repoplan.OpRemoveNode, Path: "apps/api/modules/booking/"},
		{Op: repoplan.OpMoveNode, Path: "apps/api/modules/booking/index.ts", To: "apps/api/index.ts"},
	})
	if err == nil || !strings.Contains(err.Error(), "perubahan 2") || !strings.Contains(err.Error(), "sejak perubahan 1") {
		t.Fatalf("move of a removed path: %v", err)
	}

	// A folder created by an earlier op is a real target for the next ones.
	c := plan.Clone()
	if _, err := repoplan.Apply(c, []core.RepoOp{
		{Op: repoplan.OpSetNode, Node: &core.RepoNode{Path: "apps/api/jobs/", Type: core.NodeDir, Owner: core.OwnerPlatform, Responsibility: "job", Reason: "job"}},
		{Op: repoplan.OpMoveNode, Path: "apps/api/jobs/", To: "apps/jobs/"},
		{Op: repoplan.OpRemoveNode, Path: "apps/jobs"},
	}); err != nil || c.Node("apps/jobs/") != nil || c.Node("apps/api/jobs/") != nil {
		t.Fatalf("create, move, remove: %v", err)
	}

	// Recreating a moved path makes it a new node: later ops mean that one.
	c = plan.Clone()
	if _, err := repoplan.Apply(c, []core.RepoOp{
		{Op: repoplan.OpMoveNode, Path: "docs/", To: "documentation/"},
		{Op: repoplan.OpSetNode, Node: &core.RepoNode{Path: "docs/", Type: core.NodeDir, Owner: core.OwnerPlatform, Responsibility: "docs baru", Reason: "docs"}},
		{Op: repoplan.OpRemoveNode, Path: "docs/"},
	}); err != nil || c.Node("documentation/") == nil || c.Node("docs/") != nil {
		t.Fatalf("recreate after move: %v %+v", err, c.Nodes)
	}
}

func TestOpsRefuseUnknownPathWithWhatExists(t *testing.T) {
	_, plan := monolith()
	repoplan.Normalize(plan)
	_, err := repoplan.Apply(plan, []core.RepoOp{{Op: repoplan.OpRemoveNode, Path: "apps/api/modules/payments/lib/"}})
	if err == nil {
		t.Fatal("unknown path accepted")
	}
	for _, want := range []string{"apps/api/modules/payments/lib/", "folder terdekat yang ada apps/api/modules/", "billing/", "booking/"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
}
