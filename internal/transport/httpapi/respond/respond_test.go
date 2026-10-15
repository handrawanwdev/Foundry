package respond

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("bad json %q: %v", w.Body, err)
	}
	return m
}

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	Success(w, map[string]int{"a": 1})
	m := decode(t, w)
	if w.Code != 200 || m["success"] != true || m["meta"] != nil || m["data"].(map[string]any)["a"] != float64(1) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"meta":null`) {
		t.Fatalf("meta must be explicit null: %s", w.Body)
	}
	w = httptest.NewRecorder()
	Success(w, 1, map[string]string{"k": "v"})
	if decode(t, w)["meta"].(map[string]any)["k"] != "v" {
		t.Fatalf("meta lost: %s", w.Body)
	}
}

func TestCreatedAndNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	Created(w, "x")
	if w.Code != http.StatusCreated || decode(t, w)["success"] != true {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	NoContent(w)
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("%d %q", w.Code, w.Body)
	}
}

func TestFail(t *testing.T) {
	w := httptest.NewRecorder()
	Fail(w, CodeValidation, "Request tidak valid", 400)
	m := decode(t, w)
	e := m["error"].(map[string]any)
	if w.Code != 400 || m["success"] != false || e["code"] != CodeValidation || e["message"] != "Request tidak valid" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if d, ok := e["details"].([]any); !ok || len(d) != 0 {
		t.Fatalf("details must be [] not null: %s", w.Body)
	}
	w = httptest.NewRecorder()
	Fail(w, CodeValidation, "m", 400, Detail{Field: "dir", Message: "wajib"})
	if decode(t, w)["error"].(map[string]any)["details"].([]any)[0].(map[string]any)["field"] != "dir" {
		t.Fatalf("details lost: %s", w.Body)
	}
}

func TestPaginated(t *testing.T) {
	cases := []struct{ total, limit, pages int }{{0, 10, 0}, {10, 10, 1}, {11, 10, 2}, {5, 0, 0}}
	for _, c := range cases {
		w := httptest.NewRecorder()
		Paginated(w, []int{}, Pagination{Page: 1, Limit: c.limit, Total: c.total})
		meta := decode(t, w)["meta"].(map[string]any)
		if w.Code != 200 || meta["totalPages"] != float64(c.pages) || meta["page"] != float64(1) || meta["total"] != float64(c.total) || meta["limit"] != float64(c.limit) {
			t.Fatalf("%+v → %s", c, w.Body)
		}
	}
}

func TestCodeFor(t *testing.T) {
	for status, want := range map[int]string{400: CodeValidation, 403: CodeForbidden, 404: CodeNotFound, 409: CodeConflict, 412: CodePrecondition, 500: CodeInternal} {
		if got := CodeFor(status); got != want {
			t.Errorf("%d: %s != %s", status, got, want)
		}
	}
}
