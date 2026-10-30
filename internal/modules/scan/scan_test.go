package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectsComponents(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module x\n")
	write(t, filepath.Join(dir, "web", "package.json"), `{"dependencies":{"react":"18"}}`)
	r := Scan(dir)
	kinds := map[string]bool{}
	for _, c := range r.Components {
		kinds[c.Kind+" "+c.Tech] = true
	}
	if !r.Existing || !kinds["backend Go"] || !kinds["web React"] {
		t.Fatalf("scan = %+v", r)
	}
}

func TestDetectsStack(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "package.json"), `{"workspaces":["apps/*"],"devDependencies":{"eslint":"9","prettier":"3"}}`)
	write(t, filepath.Join(dir, "apps", "api", "go.mod"), "module x\nrequire (\n\tgithub.com/gin-gonic/gin v1\n\tgithub.com/jackc/pgx/v5 v5\n)\n")
	write(t, filepath.Join(dir, "apps", "api", "main_test.go"), "package main\n")
	write(t, filepath.Join(dir, "apps", "web", "package.json"), `{"dependencies":{"react":"18","@mui/material":"5"},"devDependencies":{"typescript":"5","vitest":"2"}}`)
	write(t, filepath.Join(dir, "docker-compose.yml"), "services:\n  db:\n    image: postgres:16\n  cache:\n    image: \"redis:7\"\n")
	write(t, filepath.Join(dir, "requirements.txt"), "flask==3\nflask-cors\n")
	r := Scan(dir)
	got := map[string]bool{}
	for _, c := range r.Components {
		got[c.Kind+"|"+c.Path+"|"+c.Tech+"|"+c.Language] = true
	}
	for _, want := range []string{
		"backend|apps/api|Go + Gin|Go", "database|apps/api|PostgreSQL|", "testing|.|go test|Go",
		"web|apps/web|React|TypeScript", "ui|apps/web|MUI (Material UI)|", "testing|apps/web|Vitest|",
		"repository|.|Monorepo|", "quality|.|ESLint + Prettier|JavaScript",
		"deployment|.|Docker Compose (satu server)|", "database|.|PostgreSQL|", "cache|.|Redis|",
		"backend|.|Flask + Python|Python",
	} {
		if !got[want] {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	// "flask-cors" alone is not Flask; a dependency is a whole token.
	if !mentions("flask==3", "flask") || mentions("flask-cors", "flask") {
		t.Error("mentions")
	}
}

func TestMentionsBoundaries(t *testing.T) {
	cases := []struct {
		body, name string
		want       bool
	}{
		{"flask==3", "flask", true},
		{"Flask==3", "flask", true},
		{"FLASK", "flask", true},
		{"flask-cors", "flask", false},
		{"xflask", "flask", false},
		{"flasky", "flask", false},
		{" laravel/framework ", "laravel/framework", true},
		{"github.com/jackc/pgx/v5", "github.com/jackc/pgx", true},
		{"psycopg2-binary", "psycopg2", false},
		{"psycopg2-binary", "psycopg2-binary", true},
		{"", "flask", false},
		{"flask", "", false},
	}
	for _, c := range cases {
		if got := mentions(c.body, c.name); got != c.want {
			t.Errorf("mentions(%q, %q) = %v, want %v", c.body, c.name, got, c.want)
		}
	}
}
