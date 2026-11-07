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
		A string `json:"a"`
		B []int  `json:"b"`
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


