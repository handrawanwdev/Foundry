package work

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// Page ids are renumbered on every design run, so pages are matched by
// platform + name, and link targets are compared by the target page's name.

var componentNames = map[string]string{
	"header": "Judul", "nav": "Navigasi", "text": "Teks", "list": "Daftar", "table": "Tabel",
	"form": "Form", "button": "Tombol", "card": "Kartu", "stat": "Angka", "input": "Isian",
	"section": "Bagian", "steps": "Langkah",
}

func pageKey(pg core.Page) string {
	return pg.Platform + "|" + strings.ToLower(strings.TrimSpace(pg.Name))
}

func componentKey(c core.PageComponent) string {
	return c.Type + "|" + strings.ToLower(strings.TrimSpace(c.Label))
}

func componentName(c core.PageComponent) string {
	n := componentNames[c.Type]
	if n == "" {
		n = c.Type
	}
	return fmt.Sprintf("%s “%s”", n, c.Label)
}

// DiffPages lists page-level changes from old to new: added and changed pages
// in new order, then removed pages.
func DiffPages(old, new []core.Page) []core.DesignChange {
	oldNames, newNames := pageNames(old), pageNames(new)
	oldByKey := map[string]core.Page{}
	for _, pg := range old {
		oldByKey[pageKey(pg)] = pg
	}
	seen := map[string]bool{}
	var out []core.DesignChange
	for _, pg := range new {
		k := pageKey(pg)
		seen[k] = true
		prev, ok := oldByKey[k]
		if !ok {
			out = append(out, core.DesignChange{Kind: "added", PageID: pg.ID, Page: pg.Name})
			continue
		}
		if d := pageDetails(prev, pg, oldNames, newNames); len(d) > 0 {
			out = append(out, core.DesignChange{Kind: "changed", PageID: pg.ID, Page: pg.Name, Details: d})
		}
	}
	for _, pg := range old {
		if !seen[pageKey(pg)] {
			out = append(out, core.DesignChange{Kind: "removed", Page: pg.Name})
		}
	}
	return out
}

func pageNames(pages []core.Page) map[string]string {
	m := map[string]string{}
	for _, pg := range pages {
		m[pg.ID] = pg.Name
	}
	return m
}

func pageDetails(old, new core.Page, oldNames, newNames map[string]string) []string {
	var d []string
	if old.Purpose != new.Purpose {
		d = append(d, "~ Tujuan halaman")
	}
	if old.Role != new.Role {
		d = append(d, "~ Peran: "+new.Role)
	}
	if old.FeatureID != new.FeatureID {
		d = append(d, "~ Fitur: "+new.FeatureID)
	}
	if old.Kind != new.Kind {
		d = append(d, "~ Jenis halaman: "+new.Kind)
	}
	if old.Density != new.Density {
		d = append(d, "~ Kepadatan: "+new.Density)
	}
	if !reflect.DeepEqual(old.States, new.States) {
		d = append(d, "~ Kondisi kosong/error/berhasil")
	}
	oldComp := map[string]core.PageComponent{}
	for _, c := range old.Components {
		oldComp[componentKey(c)] = c
	}
	newKeys := map[string]bool{}
	for _, c := range new.Components {
		k := componentKey(c)
		newKeys[k] = true
		prev, ok := oldComp[k]
		switch {
		case !ok:
			d = append(d, "+ "+componentName(c))
		case !slices.Equal(prev.Items, c.Items) || !slices.Equal(prev.Columns, c.Columns) || oldNames[prev.Target] != newNames[c.Target]:
			d = append(d, "~ "+componentName(c))
		}
	}
	for _, c := range old.Components {
		if !newKeys[componentKey(c)] {
			d = append(d, "− "+componentName(c))
		}
	}
	return d
}
