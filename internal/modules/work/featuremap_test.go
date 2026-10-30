package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

const clinicMapJSON = `{
  "summary": "Aplikasi klinik",
  "modules": [{
    "name": "Janji Temu",
    "features": [{
      "name": "Buat Janji",
      "description": "Pasien membuat janji",
      "purpose": "Mendapat jadwal periksa",
      "benefit": "Tanpa antrian kertas",
      "outcome": "Janji tersimpan",
      "children": [
        {"name": "Cari dokter", "description": "Pilih dokter dari daftar"},
        {"name": "Konfirmasi"}
      ]
    }]
  }]
}`

func TestFeatureMapPromptFingerprint(t *testing.T) {
	p := &core.Project{Story: "platform exchange cryptocurrency dengan spot dan futures"}
	s := FeatureMapPrompt(p)
	if !strings.Contains(s, "You are the feature mapper of Foundry") {
		t.Fatal("missing identity")
	}
	if strings.Contains(s, "discovery analyst") {
		t.Fatal("feature mapper prompt must not match discovery fakes")
	}
	if !strings.Contains(s, "ACTION: generate") || !strings.Contains(s, "spot dan futures") {
		t.Fatalf("prompt = %s", s)
	}
	if !strings.Contains(s, "simple everyday words") || !strings.Contains(s, "bukan bahasa baku") {
		t.Fatal("missing simple-language rule")
	}
}

func TestApplyFeatureMapGenerate(t *testing.T) {
	p := &core.Project{Story: "klinik", FeatureMap: &core.FeatureMap{Request: &core.FeatureMapRequest{Action: core.FeatureMapGenerate}}}
	if err := ApplyFeatureMap(p, clinicMapJSON); err != nil {
		t.Fatal(err)
	}
	if p.FeatureMap.Request != nil || p.FeatureMap.Summary != "Aplikasi klinik" {
		t.Fatalf("map = %+v", p.FeatureMap)
	}
	n := p.FeatureMap.Find("buat-janji")
	if n == nil || n.Purpose == "" || len(n.Children) != 2 {
		t.Fatalf("node = %+v", n)
	}
}

func TestApplyFeatureMapExpandAndGaps(t *testing.T) {
	p := &core.Project{Story: "klinik"}
	if err := ApplyFeatureMap(p, clinicMapJSON); err != nil {
		t.Fatal(err)
	}
	p.FeatureMap.Request = &core.FeatureMapRequest{Action: core.FeatureMapExpand, NodeID: "buat-janji"}
	if err := ApplyFeatureMap(p, `{"node":{"name":"Buat Janji","description":"Alur lengkap","purpose":"Jadwal pasti","benefit":"Lebih cepat","outcome":"Janji pending",
		"children":[{"name":"Pilih jam"}]}}`); err != nil {
		t.Fatal(err)
	}
	n := p.FeatureMap.Find("buat-janji")
	if n.Description != "Alur lengkap" || len(n.Children) != 3 {
		t.Fatalf("expanded = %+v", n)
	}

	p.FeatureMap.Request = &core.FeatureMapRequest{Action: core.FeatureMapGaps}
	if err := ApplyFeatureMap(p, `{"modules":[{"name":"Tagihan","features":[{"name":"Invoice","description":"Tagihan janji"}]}]}`); err != nil {
		t.Fatal(err)
	}
	if p.FeatureMap.FindModule("tagihan") == nil || p.FeatureMap.Find("invoice") == nil {
		t.Fatalf("gaps = %+v", p.FeatureMap)
	}

	p.FeatureMap.Request = &core.FeatureMapRequest{Action: core.FeatureMapGaps}
	if err := ApplyFeatureMap(p, `{"modules":[]}`); err != nil {
		t.Fatal(err)
	}
}

func TestApplyFeatureMapRejectsEmptyGenerate(t *testing.T) {
	p := &core.Project{FeatureMap: &core.FeatureMap{Request: &core.FeatureMapRequest{Action: core.FeatureMapGenerate}}}
	if err := ApplyFeatureMap(p, `{"modules":[]}`); err == nil {
		t.Fatal("empty generate accepted")
	}
}

func TestFeatureMapContext(t *testing.T) {
	p := &core.Project{Story: "klinik"}
	if FeatureMapContext(p) != "" {
		t.Fatal("empty")
	}
	ApplyFeatureMap(p, clinicMapJSON)
	s := FeatureMapContext(p)
	if !strings.Contains(s, "FEATURE MAP") || !strings.Contains(s, "Janji Temu") || !strings.Contains(s, "Cari dokter") {
		t.Fatalf("ctx = %s", s)
	}
}
