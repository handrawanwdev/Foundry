package render_test

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	web "github.com/handrawanw/foundry/apps/web"
	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/render"
	"github.com/handrawanw/foundry/internal/transport/httpapi"
)

// Needs the built UI (make web) and Chrome; skipped otherwise.
func TestShotRendersTheWireframe(t *testing.T) {
	if web.FS() == nil {
		t.Skip("UI belum di-build")
	}
	t.Setenv("FOUNDRY_HOME", t.TempDir())
	dir := t.TempDir()
	st, _ := core.NewStore(dir)
	if _, err := st.Init("Klinik Sehat"); err != nil {
		t.Fatal(err)
	}
	st.Update(func(p *core.Project) error {
		p.Design = &core.Design{Status: core.DesignDraft, Pages: []core.Page{{ID: "PAGE-001", Name: "Masuk", Platform: "web", Kind: "auth",
			PrimaryAction: &core.PageAction{Label: "Masuk"},
			Components: []core.PageComponent{{Type: "header", Label: "Masuk ke Klinik"},
				{Type: "form", Fields: []core.FormField{{Label: "Email", Type: "email", Required: true}}},
				{Type: "button", Label: "Masuk"}}}}}
		return nil
	})
	srv := httpapi.New(web.FS(), "")
	if _, err := srv.Open(dir, "", false); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	c := render.Find(ts.URL)
	if c == nil {
		t.Skip("Chrome tidak ada")
	}
	out := filepath.Join(t.TempDir(), "shot.png")
	outline, err := c.Shot(context.Background(), st.Dir, "PAGE-001", "mobile", out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Tampilan mobile:", "[1] form:", "Tombol bawaan form:", `[2] button: Tombol "Masuk"`} {
		if !strings.Contains(outline, want) {
			t.Errorf("outline missing %q:\n%s", want, outline)
		}
	}
	if info, err := os.Stat(out); err != nil || info.Size() < 1000 {
		t.Fatalf("screenshot: %v", err)
	}
	if os.Getenv("KEEP_SHOT") != "" {
		b, _ := os.ReadFile(out)
		os.WriteFile(os.Getenv("KEEP_SHOT"), b, 0o644)
	}
	if _, err := c.Shot(context.Background(), t.TempDir(), "PAGE-001", "mobile", out); err == nil || !strings.Contains(err.Error(), "project lain") {
		t.Fatalf("another project must be refused: %v", err)
	}
}
