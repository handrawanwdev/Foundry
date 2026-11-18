package core

import (
	"slices"
	"testing"
)

func TestParseDataField(t *testing.T) {
	for _, c := range []struct {
		spec, name, kind, min, ref, def string
		options                         []string
		unique, optional, required      bool
		maxLen                          int
	}{
		{spec: "sku unique", name: "sku", kind: "text", unique: true},
		{spec: "min_stock >= 0", name: "min_stock", kind: "number", min: "0"},
		{spec: "qty > 0", name: "qty", kind: "number", min: "1"},
		{spec: "type (in|out)", name: "type", kind: "enum", options: []string{"in", "out"}},
		{spec: "category_id FK", name: "category_id", kind: "ref", ref: "Category"},
		{spec: "created_by FK User", name: "created_by", kind: "ref", ref: "User"},
		{spec: "reversal_of_id nullable FK self", name: "reversal_of_id", kind: "ref", ref: "StockMovement", optional: true},
		{spec: "note", name: "note", kind: "longtext"},
		{spec: "email unique", name: "email", kind: "email", unique: true},
		{spec: "is_active", name: "is_active", kind: "bool"},
		{spec: "amountIdr (integer, >0)", name: "amountIdr", kind: "money", min: "1"},
		{spec: "completedAt", name: "completedAt", kind: "datetime"},
		{spec: "deadline (date, nullable, tanggal kalender tanpa jam)", name: "deadline", kind: "date", optional: true},
		{spec: "status (enum: todo|in_progress|done, default todo)", name: "status", kind: "enum", options: []string{"todo", "in_progress", "done"}, def: "todo"},
		{spec: "title (text, wajib, 1-200 karakter setelah trim)", name: "title", kind: "text", required: true, maxLen: 200},
		{spec: "amount(integer rupiah)", name: "amount", kind: "money"},
		{spec: "account_number", name: "account_number", kind: "text"},
		{spec: "position", name: "position", kind: "text"},
		{spec: "timeframe (M1, M5, H1)", name: "timeframe", kind: "enum", options: []string{"M1", "M5", "H1"}},
		{spec: "market_category (forex, crypto, stocks)", name: "market_category", kind: "enum", options: []string{"forex", "crypto", "stocks"}},
		{spec: "email (unik, huruf kecil)", name: "email", kind: "email", unique: true},
		{spec: "starts (date, nullable)", name: "starts", kind: "date", optional: true},
		{spec: "enabled", name: "enabled", kind: "bool"},
		{spec: "interval_seconds", name: "interval_seconds", kind: "number"},
	} {
		f := ParseDataField("StockMovement", c.spec)
		if f.Name != c.name || f.Kind != c.kind || f.Min != c.min || f.Ref != c.ref || f.Default != c.def ||
			!slices.Equal(f.Options, c.options) || f.Unique != c.unique || f.Optional != c.optional || f.Required != c.required || f.MaxLen != c.maxLen {
			t.Errorf("%q: got %+v", c.spec, f)
		}
	}
}

func TestProjectDataField(t *testing.T) {
	p := &Project{Blueprint: &Blueprint{Database: DatabaseArch{Entities: []Entity{{Name: "Item", Fields: []string{"id", "sku unique", "min_stock >= 0"}}}}}}
	if f, ok := p.DataField("Item.min_stock"); !ok || f.Kind != "number" || f.Min != "0" {
		t.Fatalf("Item.min_stock: %+v %v", f, ok)
	}
	if _, ok := p.DataField("Item.price"); ok {
		t.Fatal("unknown field found")
	}
}
