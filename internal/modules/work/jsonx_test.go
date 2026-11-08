package work

import (
	"strings"
	"testing"
)

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

type extractCase struct {
	name string
	raw  string
	ok   bool
	want string
}

func TestExtractJSONCases(t *testing.T) {
	type obj struct {
		A string            `json:"a"`
		B []int             `json:"b"`
		N map[string]string `json:"n"`
	}
	cases := []extractCase{
		{name: "valid", raw: `{"a":"ok","b":[1]}`, ok: true, want: "ok"},
		{name: "fence", raw: "here\n```json\n{\"a\":\"ok\"}\n```\n", ok: true, want: "ok"},
		{name: "prose before", raw: "Berikut hasilnya:\n{\"a\":\"ok\"}", ok: true, want: "ok"},
		{name: "prose after", raw: "{\"a\":\"ok\"}\nTerima kasih.", ok: true, want: "ok"},
		{name: "nested", raw: `{"a":"ok","n":{"k":"v"}}`, ok: true, want: "ok"},
		{name: "escaped", raw: `{"a":"kutip \"dalam\" dan \\ slash"}`, ok: true, want: `kutip "dalam" dan \ slash`},
		{name: "trailing comma", raw: `{"a":"ok","b":[1,2,],}`, ok: true, want: "ok"},
		{name: "extra paren", raw: `{"a":"awal.")"}`, ok: true, want: "awal."},
		{name: "extra quote", raw: `{"a":"ditentukan.""}`, ok: true, want: "ditentukan."},
		{name: "brace for bracket", raw: `{"a":"ok","b":[1},"n":{"k":"v"}}`, ok: true, want: "ok"},
		{name: "truncated", raw: `{"a":"ok","b":[1`, ok: false},
		{name: "empty", raw: "   ", ok: false},
		{name: "no json", raw: "maaf, saya tidak bisa", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got obj
			err := ExtractJSONFunc(tc.raw, &got, func() bool { return got.A != "" || len(got.B) > 0 })
			if tc.ok {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				if tc.want != "" && got.A != tc.want {
					t.Fatalf("a = %q want %q", got.A, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("decoded %+v", got)
			}
			if tc.name == "empty" && err.Error() != "output AI kosong" {
				t.Fatalf("empty err = %v", err)
			}
			if tc.name == "truncated" && !strings.Contains(err.Error(), "terpotong") {
				t.Fatalf("truncated err = %v", err)
			}
		})
	}
}

func techRevisionMatch(a *techRevision) func() bool {
	return func() bool {
		return a.Set != nil || a.Remove != nil || a.Evaluation != nil || a.Compat != nil || a.Consistency != nil
	}
}

// A mistyped value used to fail the whole answer, and the scan then took a
// compatibility entry (it has a "note") for the answer: the revision was
// refused for leaving out what it had included.
func TestExtractJSONCoercesMistypedValues(t *testing.T) {
	raw := "```json\n" + `{"set": {"area": "backend", "technology": "Go", "version": 1.22, "purpose": "api", "driver": "d", "reasoning": ["fast", "simple"]},
	 "remove": "",
	 "compatibility": [{"area": "backend", "status": "compatible", "note": "fine"}],
	 "evaluation": {"summary": "s", "criteria": [{"name": "security", "score": 4.5, "reason": "r", "evidence": "backend"}, {"name": "performance", "score": "3", "reason": "r"}], "findings": []},
	 "consistency": [{"area": "requirements", "status": "consistent", "note": "n"}],
	 "note": "x"}` + "\n```"
	var a techRevision
	if err := ExtractJSONFunc(raw, &a, techRevisionMatch(&a)); err != nil {
		t.Fatal(err)
	}
	if a.Note != "x" || a.Evaluation == nil || len(a.Compat) != 1 || len(a.Consistency) != 1 {
		t.Fatalf("answer = %+v", a)
	}
	if len(a.Set) != 1 || a.Set[0].Version != "1.22" || a.Set[0].Reasoning != "fast; simple" || len(a.Remove) != 0 {
		t.Fatalf("set = %+v remove = %v", a.Set, a.Remove)
	}
	c := a.Evaluation.Criteria
	if len(c) != 2 || c[0].Score != 5 || len(c[0].Evidence) != 1 || c[1].Score != 3 {
		t.Fatalf("criteria = %+v", c)
	}
}

func TestExtractJSONNeverTakesPartOfABrokenAnswer(t *testing.T) {
	for name, raw := range map[string]string{
		// "set" cannot become decisions: the error must say so.
		"mistyped":  `{"set": ["Go"], "compatibility": [{"area": "backend", "status": "compatible", "note": "fine"}], "note": "x"}`,
		"truncated": `{"set": [], "compatibility": [{"area": "backend", "status": "compatible", "note": "fine"}], "evaluation": {"summary": "s", "crit`,
	} {
		var a techRevision
		match := func() bool { return a.Set != nil || a.Note != "" }
		err := ExtractJSONFunc(raw, &a, match)
		if err == nil {
			t.Fatalf("%s: took %+v as the answer", name, a)
		}
		if name == "mistyped" && !strings.Contains(err.Error(), "tidak sesuai format") {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}

func TestExtractJSONStillUnwrapsEnvelope(t *testing.T) {
	var a techRevision
	raw := `{"answer": {"set": [], "compatibility": [{"area": "backend", "status": "compatible"}], "evaluation": {"criteria": []}, "consistency": [{"area": "requirements", "status": "consistent"}]}}`
	if err := ExtractJSONFunc(raw, &a, techRevisionMatch(&a)); err != nil || a.Evaluation == nil {
		t.Fatalf("err = %v answer = %+v", err, a)
	}
}
