package uiscan_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/uiscan"
)

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func colors(tk *core.DesignTokens) map[string]string {
	out := map[string]string{}
	for _, c := range tk.Colors {
		out[c.Name] = c.Value + " " + c.Use
	}
	return out
}

func TestNextShadcn(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"web/app/layout.tsx":                  `export default function RootLayout() {}`,
		"web/app/page.tsx":                    ``,
		"web/app/(dashboard)/orders/page.tsx": ``,
		"web/app/orders/[id]/page.tsx":        ``,
		"web/components/ui/button.tsx":        ``,
		"web/components/ui/sidebar.tsx":       `title: "Bukan menu"`,
		"web/components/app-sidebar.tsx":      `const items = [{ title: "Pesanan", url: "/orders" }, { title: "Laporan", url: "/reports" }]`,
		"web/components/page-header.tsx":      ``,
		"web/components/button.test.tsx":      ``,
		"web/app/globals.css": `/* shadcn */
:root {
  --background: oklch(1 0 0);
  --foreground: oklch(0.145 0 0);
  --primary: oklch(0.205 0 0);
  --muted: oklch(0.97 0 0);
  --muted-foreground: oklch(0.556 0 0);
  --border: oklch(0.922 0 0);
  --destructive: oklch(0.577 0.245 27.325);
  --radius: 0.625rem;
}
.dark { --background: oklch(0.145 0 0); --primary: oklch(0.922 0 0); }
body { font-family: "Inter", sans-serif; }`,
	})
	ui := uiscan.Scan(dir, []core.Component{{Kind: "web", Path: "web", Tech: "Next.js"}, {Kind: "backend", Path: "api"}})
	if ui == nil {
		t.Fatal("no ui")
	}
	if !slices.Equal(ui.Routes, []string{"/", "/orders", "/orders/[id]"}) {
		t.Errorf("routes = %v", ui.Routes)
	}
	if ui.Navigation != "side" || !slices.Equal(ui.NavItems, []string{"Pesanan", "Laporan"}) {
		t.Errorf("nav = %s %v", ui.Navigation, ui.NavItems)
	}
	if !slices.Equal(ui.Shell, []string{"web/app/layout.tsx", "web/components/app-sidebar.tsx"}) {
		t.Errorf("shell = %v", ui.Shell)
	}
	if len(ui.Components) != 4 || !strings.HasPrefix(ui.Components[0], "Button (web/components/ui/button.tsx)") || !slices.Contains(ui.Components, "PageHeader (web/components/page-header.tsx)") {
		t.Errorf("components = %v", ui.Components)
	}

	tk, sources := uiscan.Tokens(dir, ui)
	if tk == nil {
		t.Fatal("no tokens")
	}
	c := colors(tk)
	if c["surface.page"] != "#ffffff dari web/app/globals.css" || c["action.primary"] != "#171717 dari web/app/globals.css" ||
		c["text.secondary"] != "#737373 dari web/app/globals.css" || c["status.success"] != "" {
		t.Errorf("colors = %v", c)
	}
	if len(tk.Radius) != 1 || tk.Radius[0] != (core.Token{Name: "radius.md", Value: "0.625rem", Use: "dari web/app/globals.css"}) {
		t.Errorf("radius = %v", tk.Radius)
	}
	if len(tk.Typography) != 1 || tk.Typography[0].Value != "Inter" || len(tk.Spacing) != 0 {
		t.Errorf("type = %v spacing = %v", tk.Typography, tk.Spacing)
	}
	if !slices.Equal(sources, []string{"web/app/globals.css"}) {
		t.Errorf("sources = %v", sources)
	}
}

func TestViteBootstrapTailwind(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"src/router.tsx":             `const routes = [{ path: "/tickets", element: <Tickets /> }]; <Route path="/login" element={<Login />} />`,
		"src/pages/Tickets.tsx":      ``,
		"src/components/Navbar.tsx":  `<NavLink to="/tickets">Tiket</NavLink>`,
		"src/styles/_variables.scss": "$primary: #0d6efd !default;\n$danger: #dc3545;\n$body-bg: #fff;\n$body-color: #212529;\n:root { --bs-primary: #{$primary}; }",
		"tailwind.config.js": `module.exports = { theme: { extend: {
  colors: { success: { DEFAULT: "hsl(var(--ok))", 500: "#00ff00" }, gray: colors.gray, ...rest },
  fontFamily: { sans: ["Plus Jakarta Sans", "sans-serif"] },
} } }`,
		"src/index.css": `:root { --ok: 142 71% 45%; } @media (prefers-color-scheme: dark) { :root { --bs-primary: #000; } }`,
	})
	ui := uiscan.Scan(dir, []core.Component{{Kind: "web", Path: ".", Tech: "React"}})
	if !slices.Equal(ui.Routes, []string{"/login", "/tickets"}) || ui.Navigation != "top" || !slices.Equal(ui.NavItems, []string{"Tiket"}) {
		t.Fatalf("ui = %+v", ui)
	}
	tk, sources := uiscan.Tokens(dir, ui)
	if tk == nil {
		t.Fatal("no tokens")
	}
	c := colors(tk)
	if c["action.primary"] != "#0d6efd dari src/styles/_variables.scss" || c["action.danger"] != "#dc3545 dari src/styles/_variables.scss" ||
		c["text.primary"] != "#212529 dari src/styles/_variables.scss" || c["status.success"] != "hsl(142 71% 45%) dari tailwind.config.js" {
		t.Errorf("colors = %v", c)
	}
	if len(tk.Typography) != 1 || tk.Typography[0].Value != "Plus Jakarta Sans" {
		t.Errorf("type = %v", tk.Typography)
	}
	if !slices.Contains(sources, "tailwind.config.js") || !slices.Contains(sources, "src/styles/_variables.scss") {
		t.Errorf("sources = %v", sources)
	}
}

func TestNoFrontendOrTokens(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"web/src/App.tsx": ``, "web/src/app.css": `:root { --primary: #123456; }`})
	if uiscan.Scan(dir, []core.Component{{Kind: "backend", Path: "."}}) != nil {
		t.Fatal("backend only must give no ui")
	}
	ui := uiscan.Scan(dir, []core.Component{{Kind: "web", Path: "web"}})
	if tk, _ := uiscan.Tokens(dir, ui); tk != nil {
		t.Fatalf("one color is not a design system: %+v", tk)
	}
}
