package work

import (
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func TestVersionsRecordCompareRestore(t *testing.T) {
	p := canvasProject()
	p.Dir = t.TempDir()
	p.Design.Theme = &core.ThemeContext{ID: "modern-saas", Name: "Modern SaaS", Source: "user"}
	v1, err := RecordVersion(p, "generate", "")
	if err != nil || v1 == nil || v1.ID != 1 {
		t.Fatalf("v1 = %+v, %v", v1, err)
	}
	if v, _ := RecordVersion(p, "keep", ""); v != nil {
		t.Fatal("an unchanged design must not make a version")
	}
	if _, err := EditPage(p, "PAGE-001", []CanvasOp{{Op: "remove", ID: "text-3"}}); err != nil {
		t.Fatal(err)
	}
	p.Flows[0].PageIDs = []string{"PAGE-001"}
	v2, _ := RecordVersion(p, "canvas", "hapus catatan")
	if v2 == nil || v2.ID != 2 {
		t.Fatalf("v2 = %+v", v2)
	}

	list, err := Versions(p)
	if err != nil || len(list) != 2 || list[0].ID != 2 || !list[0].Current || list[1].Current || list[0].Pages != 2 {
		t.Fatalf("versions = %+v, %v", list, err)
	}
	c := CompareVersions(*v1, *v2)
	if len(c.Changes) != 1 || c.Changes[0].PageID != "PAGE-001" || len(c.Flows) != 1 || c.System || c.Theme != [2]string{} {
		t.Fatalf("compare = %+v", c)
	}

	if err := RestoreVersion(p, *v1); err != nil {
		t.Fatal(err)
	}
	if len(p.Design.Pages[0].Components) != 4 || len(p.Flows[0].PageIDs) != 2 || p.Theme != "modern-saas" {
		t.Fatalf("restored = %+v / %+v", p.Design.Pages[0], p.Flows)
	}
	// The restored pages share nothing with the saved version.
	p.Design.Pages[0].Components[0].Label = "ubah"
	if got, _ := FindVersion(p, 1); got.Pages[0].Components[0].Label == "ubah" {
		t.Fatal("restore must copy the version")
	}
	if _, err := FindVersion(p, 9); err == nil {
		t.Fatal("unknown version must fail")
	}
	p.Design.Revision = &core.DesignRevision{}
	if err := RestoreVersion(p, *v2); err == nil {
		t.Fatal("pending revision must block a restore")
	}
}

func TestVersionsCapped(t *testing.T) {
	p := canvasProject()
	p.Dir = t.TempDir()
	for i := range MaxDesignVersions + 3 {
		p.Design.Pages[1].Purpose = string(rune('a'+i%26)) + string(rune('a'+i/26))
		if _, err := RecordVersion(p, "canvas", ""); err != nil {
			t.Fatal(err)
		}
	}
	vs, _ := LoadVersions(p.Dir)
	if len(vs) != MaxDesignVersions || vs[0].ID != 4 || vs[len(vs)-1].ID != MaxDesignVersions+3 {
		t.Fatalf("versions = %d, first %d", len(vs), vs[0].ID)
	}
}
