package core

import (
	"strings"
	"testing"
)

// The design keeps what it was drawn from; a feature added, a use case
// changed and an entity changed since are its drift. The pages a use case
// is drawn on belong to the design and are no drift.
func TestDesignDrift(t *testing.T) {
	p := &Project{
		Features:  []Feature{{ID: "F1", Name: "Barang"}},
		Flows:     []Flow{{ID: "UC1", Name: "Catat barang", Steps: []string{"Isi form"}}},
		Blueprint: &Blueprint{Database: DatabaseArch{Entities: []Entity{{Name: "Item", Fields: []string{"sku"}}}}},
		UI:        &UIDesign{},
	}
	if d := p.DesignDrift(); d.Known || !strings.Contains(d.SyncRequest(), "Periksa semua") {
		t.Fatalf("desain lama tanpa basis: %+v", d)
	}
	p.UI.Basis = DesignBasis(p)
	p.Flows[0].PageIDs = []string{"barang-form"}
	if d := p.DesignDrift(); !d.Known || len(d.Items) != 0 {
		t.Fatalf("halaman use case dihitung drift: %+v", d.Items)
	}
	p.Features = append(p.Features, Feature{ID: "F2", Name: "Mutasi"})
	p.Flows[0].Steps = append(p.Flows[0].Steps, "Simpan")
	p.Blueprint.Database.Entities[0].Fields = append(p.Blueprint.Database.Entities[0].Fields, "name")
	d := p.DesignDrift()
	want := []DriftItem{{Kind: DriftFeature, ID: "F2", Name: "Mutasi", Change: "added"}, {Kind: DriftFlow, ID: "UC1", Name: "Catat barang", Change: "changed"}, {Kind: DriftData, Change: "changed"}}
	if len(d.Items) != len(want) {
		t.Fatalf("drift: %+v", d.Items)
	}
	for i, w := range want {
		if d.Items[i] != w {
			t.Errorf("drift %d: %+v, mau %+v", i, d.Items[i], w)
		}
	}
	p.Features = p.Features[1:]
	if d := p.DesignDrift(); !strings.Contains(d.SyncRequest(), "Fitur F1 (Barang): dihapus") {
		t.Errorf("permintaan sinkron tanpa fitur dihapus:\n%s", d.SyncRequest())
	}
}
