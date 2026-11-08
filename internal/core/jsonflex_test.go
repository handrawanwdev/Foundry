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

// A revision wrote "criterion" instead of "name": every criterion lost its
// name and the evaluation was refused for scoring none of them.
func TestTechEvaluationAcceptsCriteriaAsModelsWriteThem(t *testing.T) {
	for name, raw := range map[string]string{
		"synonym": `{"criteria":[{"criterion":"security","score":"4","reason":["a","b"],"evidence":"Areas backend, database"},{"name":"performance","score":3.6,"reason":"r"}]}`,
		"keyed":   `{"criteria":{"performance":{"score":4,"reason":"r"},"security":4}}`,
	} {
		var e TechEvaluation
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(e.Criteria) != 2 {
			t.Fatalf("%s: criteria = %+v", name, e.Criteria)
		}
		for _, c := range e.Criteria {
			if c.Name == "" || c.Score < 3 {
				t.Fatalf("%s: criterion = %+v", name, c)
			}
		}
	}
	var e TechEvaluation
	json.Unmarshal([]byte(`{"criteria":[{"criterion":"security","score":"4","reason":["a","b"],"evidence":"Areas backend"}]}`), &e)
	if c := e.Criteria[0]; c.Reason != "a; b" || len(c.Evidence) != 1 || c.Score != 4 {
		t.Fatalf("criterion = %+v", c)
	}
}

func TestChecksAcceptDetailAsNote(t *testing.T) {
	var tc []TechCheck
	var rc []RepoCheck
	raw := `[{"area":"requirements","status":"consistent","detail":"semua REQ punya pemilik"}]`
	if json.Unmarshal([]byte(raw), &tc) != nil || json.Unmarshal([]byte(raw), &rc) != nil {
		t.Fatal("decode failed")
	}
	if tc[0].Note == "" || rc[0].Note == "" {
		t.Fatalf("tech = %+v repo = %+v", tc, rc)
	}
}

func TestRepoCriterionAcceptsSynonym(t *testing.T) {
	var e RepoEvaluation
	if err := json.Unmarshal([]byte(`{"criteria":[{"criterion":"cohesion","score":4.4,"reason":"r"}]}`), &e); err != nil {
		t.Fatal(err)
	}
	if c := e.Criteria[0]; c.Name != "cohesion" || c.Score != 4 {
		t.Fatalf("criterion = %+v", c)
	}
}
