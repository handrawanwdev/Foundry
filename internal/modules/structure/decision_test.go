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

func TestRecommendAndEdit(t *testing.T) {
	p := project("Booking", "Pembayaran")
	ir, _ := Recommend(p)
	want := []string{"apps/web/", "apps/api/", "apps/api/modules/booking/", "apps/api/modules/pembayaran/",
		"packages/domain/", "packages/contracts/", "packages/ui/", "docs/"}
	if ir.Type != core.StructModular || !slices.Equal(Layout(ir), want) {
		t.Fatalf("ir = %v", Layout(ir))
	}
	for _, op := range []core.StructureOp{
		{Op: "add", Kind: "module", Name: "Reporting"},
		{Op: "rename", Kind: "module", Name: "pembayaran", To: "payment"},
		{Op: "type", Name: "microservices"},
		{Op: "move", Kind: "service", Name: "reporting", To: "payment"},
	} {
		if err := ApplyOp(ir, op, p); err != nil {
			t.Fatalf("%+v: %v", op, err)
		}
	}
	if !slices.Equal(Layout(ir), []string{"apps/web/", "services/booking/", "services/payment/", "services/payment/payment/", "services/payment/reporting/",
		"packages/domain/", "packages/contracts/", "packages/ui/", "docs/"}) {
		t.Fatalf("layout = %v", Layout(ir))
	}
	if err := ApplyOp(ir, core.StructureOp{Op: "add", Kind: "service", Name: "booking"}, p); err == nil {
		t.Fatal("duplicate service must fail")
	}
	back := Convert(ir, core.StructModular, p)
	if !slices.Equal(back.Services[0].Modules, []string{"booking", "payment", "reporting"}) {
		t.Fatalf("modular = %+v", back.Services)
	}
	if d := strings.Join(Diff(ir, back), "; "); !strings.Contains(d, "Domain booking: service services/booking/ → modul di apps/api/") {
		t.Fatalf("diff = %s", d)
	}
	if tree := Tree(back); !strings.Contains(tree, "api/\n    └── modules/\n        ├── booking/\n        ├── payment/\n        └── reporting/\n") {
		t.Fatalf("tree = %s", tree)
	}
}

func TestRules(t *testing.T) {
	p := project("Booking")
	ir, _ := Recommend(p)
	p.Structure.IR = ir
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
