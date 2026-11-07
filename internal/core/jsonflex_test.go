package core

import (
	"encoding/json"
	"testing"
)

func TestCoerceFactsFromStrings(t *testing.T) {
	var c TechContext
	if err := json.Unmarshal([]byte(`{"summary":"s","facts":["satu","dua"]}`), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Facts) != 2 || c.Facts[0].Statement != "satu" {
		t.Fatalf("facts = %+v", c.Facts)
	}
}

func TestCoerceFactsFromObjects(t *testing.T) {
	var c TechContext
	raw := `{"facts":[{"category":"data","statement":"PostgreSQL","source":"requirements"}]}`
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Facts) != 1 || c.Facts[0].Category != "data" {
		t.Fatalf("facts = %+v", c.Facts)
	}
}

func TestCoerceRelationsFromArray(t *testing.T) {
	var n RepoNode
	if err := json.Unmarshal([]byte(`{"path":"apps/","type":"dir","relations":["web","api"]}`), &n); err != nil {
		t.Fatal(err)
	}
	if n.Relations != "web; api" {
		t.Fatalf("relations = %q", n.Relations)
	}
}

func TestCoerceWorkersFromStrings(t *testing.T) {
	var ir StructureIR
	if err := json.Unmarshal([]byte(`{"type":"modular","workers":["mailer","indexer"],"docs":"docs/"}`), &ir); err != nil {
		t.Fatal(err)
	}
	if len(ir.Workers) != 2 || ir.Workers[0].Name != "mailer" {
		t.Fatalf("workers = %+v", ir.Workers)
	}
}
