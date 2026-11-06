package repoplan_test

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
)

// The validator judges a plan by the project's own modules and structure
// decision, not by a layout it likes. These projects have different
// architectures and the plans organize their code differently; every one of
// them must validate, and the checks must still bite on each.

func withPart(n core.RepoNode, part string) core.RepoNode { n.Part = part; return n }

func deps(n core.RepoNode, to ...string) core.RepoNode { n.DependsOn = to; return n }

// microservices: one deployable per domain, a mobile app, a worker, and a
// contracts package every service shares. Services talk through contracts
// and events only.
func microservices() (*core.Project, *core.RepoPlan) {
	p := project([]string{"orders", "payments", "notifications"}, &core.StructureIR{Type: core.StructMicroservices,
		Apps: []core.StructurePart{{Name: "mobile", Reason: "pelanggan memesan lewat HP"}},
		Services: []core.StructurePart{
			{Name: "orders", Reason: "o"}, {Name: "payments", Reason: "p"},
		},
		Workers:  []core.StructurePart{{Name: "notifier", Reason: "kirim notifikasi dari event"}},
		Packages: []core.StructurePart{{Name: "contracts", Reason: "skema event dan API"}}})
	plan := &core.RepoPlan{
		Nodes: []core.RepoNode{
			dir("services/", core.OwnerPlatform, "layanan yang di-deploy sendiri-sendiri"),
			withPart(dir("services/orders/", "orders", "layanan pemesanan"), "service:orders"),
			deps(dir("services/orders/internal/", "orders", "kode privat layanan orders"), "contracts/"),
			file("services/orders/api.proto", "orders", "kontrak publik layanan orders"),
			withPart(dir("services/payments/", "payments", "layanan pembayaran"), "service:payments"),
			deps(dir("services/payments/internal/", "payments", "kode privat layanan payments"), "contracts/"),
			file("services/payments/api.proto", "payments", "kontrak publik layanan payments"),
			withPart(dir("workers/notifier/", "notifications", "konsumer event notifikasi"), "worker:notifier"),
			withPart(dir("mobile/", core.OwnerPlatform, "aplikasi mobile pelanggan"), "app:mobile"),
			withPart(dir("contracts/", core.OwnerShared, "skema event dan API bersama"), "package:contracts"),
			dir("deploy/", core.OwnerPlatform, "manifest deployment per layanan"),
		},
		Modules: []core.RepoModuleMap{
			{Module: "orders", Locations: []core.RepoLocation{{Path: "services/orders/", Role: "api"}, {Path: "mobile/", Role: "ui"}}, Frontend: true, Backend: true},
			{Module: "payments", Locations: []core.RepoLocation{{Path: "services/payments/", Role: "api"}}, Backend: true},
			{Module: "notifications", Locations: []core.RepoLocation{{Path: "workers/notifier/", Role: "worker"}}, Backend: true},
		},
		Dependency: core.RepoDependency{
			Allowed:       []core.RepoDepRule{{From: "orders", To: "shared"}, {From: "payments", To: "shared"}, {From: "notifications", To: "shared"}},
			Forbidden:     []core.RepoDepRule{{From: "services/orders/internal/", To: "services/payments/internal/"}, {From: "shared", To: "orders"}},
			Communication: []core.RepoDepRule{{From: "orders", To: "payments", Via: "services/payments/api.proto", Mode: core.DepSync}, {From: "payments", To: "notifications", Via: "payment.settled", Mode: core.DepEvent}, {From: "orders", To: "notifications", Via: "order.placed", Mode: core.DepEvent}},
			Interfaces:    []core.RepoInterface{{Module: "payments", Path: "services/payments/api.proto"}},
			CycleGuard:    "Layanan hilir memberi tahu hulu lewat event, tidak memanggil balik.",
		},
	}
	evaluated(plan)
	return p, plan
}

// firmware: no frontend, no backend, no services. Modules are firmware
// components; one module is only documentation.
func firmware() (*core.Project, *core.RepoPlan) {
	p := project([]string{"sensing", "transport", "calibration"}, &core.StructureIR{Type: core.StructSimple})
	plan := &core.RepoPlan{
		Nodes: []core.RepoNode{
			dir("firmware/", core.OwnerPlatform, "kode yang berjalan di perangkat"),
			deps(dir("firmware/sensing/", "sensing", "pembacaan sensor suhu"), "firmware/hal/"),
			deps(dir("firmware/transport/", "transport", "pengiriman data lewat MQTT"), "firmware/hal/"),
			dir("firmware/hal/", core.OwnerShared, "akses perangkat keras untuk semua komponen"),
			dir("tools/", core.OwnerPlatform, "skrip flash dan debug"),
			dir("docs/calibration/", "calibration", "prosedur kalibrasi sensor dilakukan di lab"),
		},
		Modules: []core.RepoModuleMap{
			{Module: "sensing", Locations: []core.RepoLocation{{Path: "firmware/sensing/", Role: "firmware"}}},
			{Module: "transport", Locations: []core.RepoLocation{{Path: "firmware/transport/", Role: "firmware"}}},
			{Module: "calibration", Locations: []core.RepoLocation{{Path: "docs/calibration/", Role: "docs"}}, Note: "Kalibrasi manual di lab; tidak ada kode aplikasi."},
		},
		Dependency: core.RepoDependency{
			Allowed:   []core.RepoDepRule{{From: "sensing", To: "shared"}, {From: "transport", To: "shared"}},
			Forbidden: []core.RepoDepRule{{From: "sensing", To: "transport"}},
		},
	}
	evaluated(plan)
	return p, plan
}

// cli: a single Go module with one command and two internal packages.
func cli() (*core.Project, *core.RepoPlan) {
	p := project([]string{"parser", "renderer"}, &core.StructureIR{Type: core.StructSimple,
		Apps: []core.StructurePart{{Name: "cli", Reason: "alat baris perintah"}}})
	plan := &core.RepoPlan{
		Nodes: []core.RepoNode{
			withPart(dir("apps/cli/", core.OwnerPlatform, "titik masuk perintah"), "app:cli"),
			deps(file("apps/cli/main.go", core.OwnerPlatform, "merangkai parser dan renderer"), "internal/parser/", "internal/renderer/"),
			dir("internal/", core.OwnerPlatform, "paket privat"),
			dir("internal/parser/", "parser", "membaca masukan menjadi model"),
			deps(dir("internal/renderer/", "renderer", "menulis model menjadi keluaran"), "internal/parser/"),
		},
		Modules: []core.RepoModuleMap{
			{Module: "parser", Locations: []core.RepoLocation{{Path: "internal/parser/"}}, Backend: true},
			{Module: "renderer", Locations: []core.RepoLocation{{Path: "internal/renderer/"}}, Backend: true},
		},
		Dependency: core.RepoDependency{
			Allowed:   []core.RepoDepRule{{From: "renderer", To: "parser"}},
			Forbidden: []core.RepoDepRule{{From: "parser", To: "renderer"}},
		},
	}
	evaluated(plan)
	return p, plan
}

func TestPlansOfDifferentArchitecturesValidate(t *testing.T) {
	for name, build := range map[string]func() (*core.Project, *core.RepoPlan){
		"monolith": monolith, "microservices": microservices, "firmware": firmware, "cli": cli,
	} {
		t.Run(name, func(t *testing.T) {
			p, plan := build()
			if errs := errorsOf(p, plan); len(errs) != 0 {
				t.Fatalf("errors = %v", errs)
			}
			if err := repoplan.Accept(p, plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMicroservicesDependencyRules(t *testing.T) {
	p, plan := microservices()
	errorsOf(p, plan)
	var kinds = map[string]int{}
	for _, e := range plan.Analysis.Edges {
		kinds[e.Kind]++
	}
	if kinds["communication"] != 3 || kinds["forbidden"] != 2 {
		t.Fatalf("edges = %+v", plan.Analysis.Edges)
	}
	// payments calling back into orders synchronously closes a cycle with
	// the orders → payments call.
	plan.Dependency.Communication = append(plan.Dependency.Communication, core.RepoDepRule{From: "payments", To: "orders", Via: "services/orders/api.proto", Mode: core.DepSync})
	if errs := errorsOf(p, plan); !hasError(errs, "payments ↔ orders") && !hasError(errs, "orders ↔ payments") {
		t.Fatalf("errors = %v", errs)
	}
	// A service reaching into another's internals breaks the forbidden rule.
	p, plan = microservices()
	for i := range plan.Nodes {
		if plan.Nodes[i].Path == "services/orders/internal/" {
			plan.Nodes[i].DependsOn = append(plan.Nodes[i].DependsOn, "services/payments/internal/")
		}
	}
	if errs := errorsOf(p, plan); !hasError(errs, "dilarang oleh aturan services/orders/internal/ → services/payments/internal/") {
		t.Fatalf("errors = %v", errs)
	}
}

func TestFirmwareModulesNeedNeitherFrontendNorBackend(t *testing.T) {
	p, plan := firmware()
	errorsOf(p, plan)
	if plan.Analysis.Dirs != len(plan.Nodes)-0 && plan.Analysis.Files != 0 {
		t.Fatalf("unexpected counts: %+v", plan.Analysis)
	}
	// A module that needs neither side and has no place must say why.
	plan.Modules[2] = core.RepoModuleMap{Module: "calibration"}
	if errs := errorsOf(p, plan); !hasError(errs, "tanpa lokasi dan tanpa alasan") {
		t.Fatalf("errors = %v", errs)
	}
	// And a module that claims code without a place is refused.
	plan.Modules[2] = core.RepoModuleMap{Module: "calibration", Backend: true}
	if errs := errorsOf(p, plan); !hasError(errs, "tanpa lokasi di repository") {
		t.Fatalf("errors = %v", errs)
	}
}

func TestCLIPartMustBeMapped(t *testing.T) {
	p, plan := cli()
	plan.Nodes[0].Part = ""
	if errs := errorsOf(p, plan); !hasError(errs, "app:cli") {
		t.Fatalf("errors = %v", errs)
	}
	// Placing the part away from its default folder is allowed; the plan is
	// then the source of truth, and says so as a warning.
	p, plan = cli()
	plan.Nodes[0].Path = "cmd/tool/"
	plan.Nodes[1].Path = "cmd/tool/main.go"
	plan.Modules = plan.Modules[:2]
	if errs := errorsOf(p, plan); len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	var moved bool
	for _, i := range plan.Analysis.Issues {
		moved = moved || i.Level == "warning" && strings.Contains(i.Message, "luar lokasi default")
	}
	if !moved {
		t.Fatalf("issues = %+v", plan.Analysis.Issues)
	}
}
