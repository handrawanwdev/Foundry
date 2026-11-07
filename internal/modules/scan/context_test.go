package scan

import (
	"path/filepath"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestContextReadsEvidenceNotFolderGuesses(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	write(t, filepath.Join(dir, "internal", "billing", "service.go"), "package billing\n")
	write(t, filepath.Join(dir, "internal", "auth", "jwt.go"), "package auth\n")
	write(t, filepath.Join(dir, "schema.prisma"), "datasource db { provider = \"postgresql\" }\nmodel User {\n  id    String\n  email String\n}\nmodel Invoice {\n  id     String\n  amount Int\n}\n")
	write(t, filepath.Join(dir, "openapi.yaml"), "openapi: 3.0.0\npaths:\n  /users:\n    get: {}\n  /invoices:\n    post: {}\n")
	write(t, filepath.Join(dir, "internal", "http", "router.go"), "package http\nfunc Routes() {\n  r.GET(\"/health\")\n  r.POST(\"/invoices\")\n}\n")
	write(t, filepath.Join(dir, "README.md"), "# Dompetku\n\n## Features\n- Login\n- Tagihan\n\n## Install\n- ignore me\n")
	write(t, filepath.Join(dir, "migrations", "001.sql"), "CREATE TABLE IF NOT EXISTS payments (id uuid);\n")

	r := Scan(dir)
	c := Context(dir, r.Components)
	if c == nil {
		t.Fatal("context")
	}
	if !hasFinding(c.Layout, "internal") || !hasFinding(c.Languages, "Go") {
		t.Fatalf("layout/lang %+v %+v", c.Layout, c.Languages)
	}
	if !hasModule(c.Modules, "internal/billing") || !hasModule(c.Modules, "internal/auth") {
		t.Fatalf("modules %+v", c.Modules)
	}
	if !hasEntity(c.Data, "User") || !hasEntity(c.Data, "Invoice") || !hasEntity(c.Data, "payments") {
		t.Fatalf("data %+v", c.Data)
	}
	if !hasAPI(c.APIs, "/users") || !hasAPI(c.APIs, "POST /invoices") {
		t.Fatalf("apis %+v", c.APIs)
	}
	if !hasFinding(c.Auth, "jwt.go") && !hasFinding(c.Auth, "auth") {
		t.Fatalf("auth %+v", c.Auth)
	}
	if !hasFinding(c.Docs, "Dompetku") {
		t.Fatalf("docs %+v", c.Docs)
	}
	if !hasFinding(c.Features, "Login") || !hasFinding(c.Features, "Tagihan") {
		t.Fatalf("features %+v", c.Features)
	}
	for _, m := range c.Modules {
		if m.Name == "billing" && m.Path != "internal/billing" {
			t.Fatalf("module path invented: %+v", m)
		}
	}
}

func TestContextNilOnEmptyDir(t *testing.T) {
	if c := Context(t.TempDir(), nil); c != nil {
		t.Fatalf("got %+v", c)
	}
}

func hasFinding(fs []core.ContextFinding, text string) bool {
	for _, f := range fs {
		if f.Text == text || f.Path == text {
			return true
		}
	}
	return false
}

func hasModule(ms []core.ContextModule, path string) bool {
	for _, m := range ms {
		if m.Path == path {
			return true
		}
	}
	return false
}

func hasEntity(es []core.ContextEntity, name string) bool {
	for _, e := range es {
		if e.Name == name {
			return true
		}
	}
	return false
}

func hasAPI(as []core.ContextAPI, detail string) bool {
	for _, a := range as {
		if a.Detail == detail || a.Path == detail {
			return true
		}
	}
	return false
}
