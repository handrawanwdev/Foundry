package core

import (
	"strconv"
	"testing"
)

func TestFeatureMapNormalizeAndFind(t *testing.T) {
	m := &FeatureMap{
		Summary: "  Exchange  ",
		Modules: []FeatureModule{
			{Name: " Trading ", Features: []FeatureNode{
				{Name: "Spot Trading", Description: " beli/jual ", Purpose: "eksekusi", Benefit: "likuid", Outcome: "order terisi",
					Children: []FeatureNode{{Name: "Market Order"}, {Name: ""}, {Name: "Limit Order"}}},
				{Name: ""},
			}},
			{Name: "  "},
		},
	}
	m.Normalize()
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if m.Summary != "Exchange" || len(m.Modules) != 1 || m.Modules[0].ID != "trading" {
		t.Fatalf("modules = %+v", m.Modules)
	}
	spot := m.Find("spot-trading")
	if spot == nil || len(spot.Children) != 2 || spot.Children[0].ID != "market-order" || m.Count() != 3 {
		t.Fatalf("spot = %+v count=%d", spot, m.Count())
	}
	mod, node := m.Locate("trading")
	if mod == nil || node != nil || mod.Name != "Trading" {
		t.Fatalf("locate module = %+v %+v", mod, node)
	}
	mod, node = m.Locate("limit-order")
	if mod == nil || node == nil || node.Name != "Limit Order" {
		t.Fatalf("locate node = %+v %+v", mod, node)
	}
}

func TestFeatureMapCapsDepthAndCount(t *testing.T) {
	var nest FeatureNode
	cur := &nest
	cur.Name = "n1"
	for i := 2; i <= 6; i++ {
		cur.Children = []FeatureNode{{Name: "n" + strconv.Itoa(i)}}
		cur = &cur.Children[0]
	}
	m := &FeatureMap{Modules: []FeatureModule{{Name: "M", Features: []FeatureNode{nest}}}}
	m.Normalize()
	n := m.Find("n1")
	for d := 1; d < MaxFeatureDepth; d++ {
		if n == nil || len(n.Children) != 1 {
			t.Fatalf("depth %d: %+v", d, n)
		}
		n = &n.Children[0]
	}
	if n == nil || len(n.Children) != 0 {
		t.Fatalf("depth cap: %+v", n)
	}

	var feats []FeatureNode
	for i := 0; i < MaxFeatureNodes+10; i++ {
		feats = append(feats, FeatureNode{Name: "F" + strconv.Itoa(i)})
	}
	big := &FeatureMap{Modules: []FeatureModule{{Name: "M", Features: feats}}}
	big.Normalize()
	if big.Count() != MaxFeatureNodes {
		t.Fatalf("count = %d", big.Count())
	}
}

func TestFeatureMapFromFeatures(t *testing.T) {
	if FeatureMapFromFeatures(nil) != nil {
		t.Fatal("empty")
	}
	m := FeatureMapFromFeatures([]Feature{
		{ID: "FEAT-001", Name: "Janji", Description: "buat janji", SubFeatures: []SubFeature{{ID: "FEAT-001.1", Name: "Cari dokter"}}},
	})
	if m.Empty() || m.Modules[0].Name != "Fitur" || m.Find("feat-001") == nil || m.Find("feat-001-1") == nil || m.Count() != 2 {
		t.Fatalf("map = %+v", m)
	}
}

func TestFeatureMapDuplicateIDs(t *testing.T) {
	m := &FeatureMap{Modules: []FeatureModule{
		{ID: "same", Name: "A", Features: []FeatureNode{{ID: "same", Name: "B"}}},
	}}
	m.Normalize()
	if m.Modules[0].ID == m.Modules[0].Features[0].ID {
		t.Fatalf("ids not unique: %+v", m)
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
}


