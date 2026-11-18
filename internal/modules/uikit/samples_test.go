package uikit

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A table of records shows data a layperson recognizes: the design's
// examples, the blueprint's choices, yes/no as Aktif, figures for seconds,
// and a hint on the column of what it holds. "Daftar item" in a table
// section is still a table.
func TestTableShowsRealisticData(t *testing.T) {
	k := kit(t)
	p := &core.Project{Name: "Trade",
		Blueprint: &core.Blueprint{Database: core.DatabaseArch{Entities: []core.Entity{
			{Name: "Instrument", Fields: []string{"id", "symbol", "market_category (forex, crypto)"}},
			{Name: "MonitorItem", Fields: []string{"id", "timeframe (M1, M5, H1)", "interval_seconds", "enabled"}},
		}}},
		UI: &core.UIDesign{Layout: &core.LayoutSpec{
			Shells: []core.LayoutShell{{ID: "app", Name: "App", Pattern: "sidebar"}},
			Pages: []core.LayoutPage{{ID: "pantau", Name: "Pantau", Pattern: "table", Shell: "app",
				Fields: []core.FieldRule{
					{Field: "Instrument.symbol", Label: "Instrumen", Presentation: "table-column", Note: "pasangan mata uang atau aset", Examples: []string{"EURUSD", "XAUUSD"}},
					{Field: "MonitorItem.timeframe", Label: "Timeframe", Presentation: "table-column"},
					{Field: "MonitorItem.interval_seconds", Label: "Interval (detik)", Presentation: "table-column"},
					{Field: "MonitorItem.enabled", Label: "Aktif", Presentation: "badge"},
				},
				Sections: []core.Section{{ID: "tabel", Name: "Daftar item", Slot: "table",
					Content: []string{"Instrumen, Timeframe, Interval, Aktif"}, Components: []string{"Table", "Badge"}}}}},
		}},
	}
	out, err := k.RenderPage(DocOf(p), "pantau", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<table class="fk-table">`, ">EURUSD<", ">XAUUSD<", ">M1<", ">M5<", ">300<",
		`fk-badge-success">Aktif<`, `title="Pasangan mata uang atau aset. Contoh: EURUSD; XAUUSD."`, `title="Contoh: M1; M5; H1."`} {
		if !strings.Contains(out, want) {
			t.Errorf("tabel tanpa %s", want)
		}
	}
	for _, bad := range []string{"Instrumen 1", "Timeframe 1", "menit lalu", "Interval (detik) 1"} {
		if strings.Contains(out, bad) {
			t.Errorf("tabel memuat placeholder %q", bad)
		}
	}
}
