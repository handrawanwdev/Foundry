package work

import "testing"

func TestCloseJSONTruncatedObject(t *testing.T) {
	in := `{"rules": {"source": "new", "principles": ["Satu"`
	got := closeJSON(in)
	if got != `{"rules": {"source": "new", "principles": ["Satu"]}}` {
		t.Fatalf("closeJSON = %q", got)
	}
}

func TestExtractJSONUnclosedFence(t *testing.T) {
	var out designOut
	raw := "```json\n" + `{"rules": {"source": "new", "principles": ["Satu"], "ux_guidelines": ["A"],
	  "accessibility": ["AA"], "responsive": ["R"],
	  "tokens": {"colors": [{"name": "action.primary", "value": "#000"}], "typography": [{"name": "type.body", "value": "14px"}],
	              "spacing": [{"name": "space.md", "value": "16px"}]}}}`
	if err := ExtractJSONFunc(raw, &out, func() bool { return out.Rules != nil }); err != nil {
		t.Fatal(err)
	}
	if out.Rules == nil || out.Rules.Source != "new" {
		t.Fatalf("rules = %+v", out.Rules)
	}
}

func TestExtractJSONSkipsEmptyEnvelope(t *testing.T) {
	var out designOut
	raw := `{"type":"result","ok":true}` + "\n" + `{"rules": {"source": "new", "principles": ["Satu"], "ux_guidelines": ["A"],
	  "accessibility": ["AA"], "responsive": ["R"],
	  "tokens": {"colors": [{"name": "action.primary", "value": "#000"}], "typography": [{"name": "type.body", "value": "14px"}],
	              "spacing": [{"name": "space.md", "value": "16px"}]}},
	  "pages": [{"id": "home", "name": "Home"}]}`
	if err := ExtractJSONFunc(raw, &out, func() bool { return out.Rules != nil || len(out.Pages) > 0 }); err != nil {
		t.Fatal(err)
	}
	if out.Rules == nil || len(out.Pages) != 1 {
		t.Fatalf("out = %+v", out)
	}
}
