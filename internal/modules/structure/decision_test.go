package structure

import (
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func project(features ...string) *core.Project {
	p := &core.Project{Platform: &core.PlatformPlan{Targets: []core.PlatformTarget{{Kind: "web"}, {Kind: "backend"}}},
		Structure: &core.Structure{Mode: core.StructureNew}}
	for _, f := range features {
		p.Features = append(p.Features, core.Feature{Name: f})
	}
	return p
}

func TestDomainsFollowCapabilities(t *testing.T) {
	p := project("Buat Booking", "Batalkan Booking", "Manajemen Pembayaran", "Laporan")
	if got := Domains(p); !slices.Equal(got, []string{"booking", "pembayaran", "laporan"}) {
		t.Fatalf("domains = %v", got)
	}
}

// designed is a structure an AI analysis could have chosen: its own style
// name and its own folders, not a Foundry template.
func designed() *core.StructureIR {
	ir := &core.StructureIR{Type: "Hexagonal modular monolith",
		Apps: []core.StructurePart{{Name: "web", Path: "frontend/", Reason: "UI pasien"}},
		Services: []core.StructurePart{{Name: "core", Path: "backend/", Reason: "Satu proses backend",
			Modules:     []string{"booking", "pembayaran"},
			ModulePaths: map[string]string{"booking": "backend/internal/booking/", "pembayaran": "backend/internal/pembayaran"}}},
		Docs: "documentation"}
	Normalize(ir)
	return ir
}

// Foundry keeps the folders the design chose and derives none from a type.
func TestNormalizeKeepsDesignedFolders(t *testing.T) {
	ir := designed()
	want := []string{"frontend/", "backend/", "backend/internal/booking/", "backend/internal/pembayaran/", "documentation/"}
	if !slices.Equal(Layout(ir), want) {
		t.Fatalf("layout = %v", Layout(ir))
	}
	if ir.Type != "Hexagonal modular monolith" {
		t.Fatalf("type = %q", ir.Type)
	}
	// A part without a folder goes next to its siblings, not into a template.
	ir.Apps = append(ir.Apps, core.StructurePart{Name: "admin"})
	ir.Services = append(ir.Services, core.StructurePart{Name: "Reporting Engine", Path: "../escape"})
	Normalize(ir)
	if ir.Apps[1].Path != "admin/" || ir.Services[1].Name != "reporting-engine" || ir.Services[1].Path != "reporting-engine/" {
		t.Fatalf("defaults = %+v %+v", ir.Apps[1], ir.Services[1])
	}
	if CleanDir("/etc") != "" || CleanDir("a/../b") != "" || CleanDir(` src\\app `) != "src/app/" {
		t.Fatalf("clean dir")
	}
}

func TestEditFollowsTheDesign(t *testing.T) {
	p := project("Booking", "Pembayaran")
	ir := designed()
	for _, op := range []core.StructureOp{
		{Op: "add", Kind: "module", Name: "Reporting", To: "core"},
		{Op: "rename", Kind: "module", Name: "pembayaran", To: "payment"},
		{Op: "type", Name: "Cell-based services"},
		{Op: "add", Kind: "service", Name: "notifier", Path: "services/notifier/"},
		{Op: "add", Kind: "service", Name: "audit"},
		{Op: "path", Kind: "service", Name: "core", Path: "server/"},
	} {
		if err := ApplyOp(ir, op, p); err != nil {
			t.Fatalf("%+v: %v", op, err)
		}
	}
	want := []string{"frontend/", "server/", "server/internal/booking/", "server/internal/payment/", "server/internal/reporting/",
		"services/notifier/", "audit/", "documentation/"}
	if ir.Type != "Cell-based services" || !slices.Equal(Layout(ir), want) {
		t.Fatalf("type %q layout = %v", ir.Type, Layout(ir))
	}
	if err := ApplyOp(ir, core.StructureOp{Op: "type", Name: "  "}, p); err == nil {
		t.Fatal("empty style must fail")
	}
	if err := ApplyOp(ir, core.StructureOp{Op: "add", Kind: "service", Name: "core"}, p); err == nil {
		t.Fatal("duplicate service must fail")
	}
	if err := ApplyOp(ir, core.StructureOp{Op: "path", Kind: "app", Name: "web", Path: "/abs"}, p); err == nil {
		t.Fatal("absolute folder must fail")
	}
	// A domain service merged into another becomes its module, next to the
	// other modules.
	before := ir.Clone()
	if err := ApplyOp(ir, core.StructureOp{Op: "move", Kind: "service", Name: "audit", To: "core"}, p); err != nil {
		t.Fatal(err)
	}
	if got := ModulePath(ir.Services[0], "audit"); got != "server/internal/audit/" {
		t.Fatalf("merged module path = %s", got)
	}
	if d := strings.Join(Diff(before, ir), "; "); !strings.Contains(d, "Domain audit: service audit/ → modul server/internal/audit/") {
		t.Fatalf("diff = %s", d)
	}
	if tree := Tree(ir); !strings.Contains(tree, "server/\n└── internal/\n    ├── booking/") {
		t.Fatalf("tree = %s", tree)
	}
}

// Validation checks what holds for any style; it judges no style itself.
func TestValidateIsStyleFree(t *testing.T) {
	p := project("Booking", "Pembayaran")
	p.Blueprint = &core.Blueprint{Modules: []core.Module{{ID: "booking"}, {ID: "pembayaran"}}}
	if issues := Validate(designed(), p, nil); len(issues) != 0 {
		t.Fatalf("issues = %+v", issues)
	}
	ir := designed()
	ir.Type = ""
	ir.Apps = append(ir.Apps, core.StructurePart{Name: "admin", Path: "backend/"})
	var errs []string
	for _, i := range Validate(ir, p, nil) {
		if i.Level == "error" {
			errs = append(errs, i.Message)
		}
	}
	if len(errs) != 2 || !strings.Contains(strings.Join(errs, ";"), "Folder backend/ dipakai") {
		t.Fatalf("errors = %v", errs)
	}
}

func TestRules(t *testing.T) {
	p := project("Booking")
	p.Structure.IR = designed()
	ops := []core.StructureOp{
		{Op: "add", Kind: "rule", Name: "  Tiap modul pakai layer domain, application,  infrastructure, presentation. "},
		{Op: "add", Kind: "rule", Name: "tiap modul pakai layer domain, application, infrastructure, presentation."},
		{Op: "remove", Kind: "module", Name: "tidak-ada"},
		{Op: "add", Kind: "rule", Name: "Dependensi antar-modul lewat public API."},
	}
	ok, skipped := Usable(p.Structure, p, ops)
	if len(ok) != 2 || len(skipped) != 2 {
		t.Fatalf("ok = %v, skipped = %v", ok, skipped)
	}
	if err := Propose(p.Structure, p, ok, "ai", "standar", ""); err != nil {
		t.Fatal(err)
	}
	pr := p.Structure.Proposal
	if !slices.Equal(pr.IR.Rules, []string{"Tiap modul pakai layer domain, application, infrastructure, presentation.", "Dependensi antar-modul lewat public API."}) {
		t.Fatalf("rules = %q", pr.IR.Rules)
	}
	if d := strings.Join(pr.Changes, "; "); !strings.Contains(d, "Aturan ditambah: Dependensi antar-modul lewat public API.") {
		t.Fatalf("changes = %s", d)
	}
	if err := Propose(p.Structure, p, []core.StructureOp{{Op: "rename", Kind: "rule", Name: "dependensi antar-modul lewat public API.", To: "Dependensi antar-modul lewat contract atau event."}}, "user", "", ""); err != nil {
		t.Fatal(err)
	}
	if got := p.Structure.Proposal.IR.Rules[1]; got != "Dependensi antar-modul lewat contract atau event." {
		t.Fatalf("renamed = %q", got)
	}
}
