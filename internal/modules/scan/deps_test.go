package scan

import (
	"path/filepath"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestGoModuleImportsObserved(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	write(t, filepath.Join(dir, "internal", "billing", "service.go"), "package billing\nimport (\n\t\"example.com/app/internal/auth\"\n\t\"fmt\"\n)\n")
	write(t, filepath.Join(dir, "internal", "auth", "jwt.go"), "package auth\n")
	c := Context(dir, Scan(dir).Components)
	if !hasDep(c.Deps, "internal/billing", "internal/auth", core.DepKindModule) {
		t.Fatalf("deps %+v", c.Deps)
	}
	if hasDep(c.Deps, "internal/billing", "fmt", core.DepKindModule) {
		t.Fatalf("stdlib treated as module: %+v", c.Deps)
	}
	for _, m := range c.Modules {
		if m.Path == "internal/billing" && !contains(m.Depends, "internal/auth") {
			t.Fatalf("module depends %+v", m)
		}
	}
	for _, d := range c.Deps {
		if d.Confidence != core.ConfidenceObserved {
			t.Fatalf("scanner wrote %s", d.Confidence)
		}
	}
}

func TestTypeScriptPackageImport(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "package.json"), `{"name":"app","dependencies":{"next":"14.0.0"},"workspaces":["packages/*"]}`)
	write(t, filepath.Join(dir, "packages", "ui", "package.json"), `{"name":"@repo/ui"}`)
	write(t, filepath.Join(dir, "packages", "ui", "button.tsx"), "export const Button = () => null\n")
	write(t, filepath.Join(dir, "packages", "web", "page.tsx"), "import { Button } from '@repo/ui'\n")
	c := Context(dir, Scan(dir).Components)
	if !hasModule(c.Modules, "packages/ui") || !hasModule(c.Modules, "packages/web") {
		t.Fatalf("modules %+v", c.Modules)
	}
	if !hasDep(c.Deps, "packages/web", "packages/ui", core.DepKindPackage) &&
		!hasDep(c.Deps, "packages/web", "packages/ui", core.DepKindShared) {
		t.Fatalf("ts deps %+v", c.Deps)
	}
}

func TestPythonRelativeImport(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "requirements.txt"), "fastapi==0.110\n")
	write(t, filepath.Join(dir, "app", "auth", "jwt.py"), "def token():\n    return 1\n")
	write(t, filepath.Join(dir, "app", "billing", "service.py"), "from app.auth import jwt\n")
	c := Context(dir, Scan(dir).Components)
	if !hasModule(c.Modules, "app/auth") || !hasModule(c.Modules, "app/billing") {
		t.Fatalf("modules %+v", c.Modules)
	}
	if !hasDep(c.Deps, "app/billing", "app/auth", core.DepKindModule) {
		t.Fatalf("py deps %+v", c.Deps)
	}
}

func TestMonorepoSharedAndEntity(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/shop\n")
	write(t, filepath.Join(dir, "internal", "orders", "svc.go"), "package orders\nimport \"example.com/shop/internal/catalog\"\n")
	write(t, filepath.Join(dir, "internal", "catalog", "svc.go"), "package catalog\n")
	write(t, filepath.Join(dir, "internal", "catalog", "schema.prisma"), "model Product {\n  id String\n}\n")
	c := Context(dir, Scan(dir).Components)
	if !hasDep(c.Deps, "internal/orders", "internal/catalog", core.DepKindModule) {
		t.Fatalf("orders→catalog %+v", c.Deps)
	}
	if !hasDep(c.Deps, "internal/catalog", "Product", core.DepKindEntity) &&
		!hasDepKindTo(c.Deps, "Product", core.DepKindEntity) {
		t.Fatalf("entity %+v", c.Deps)
	}
}

func TestRelativeTSImport(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "package.json"), `{"dependencies":{"next":"14"}}`)
	write(t, filepath.Join(dir, "src", "auth", "session.ts"), "export const session = 1\n")
	write(t, filepath.Join(dir, "src", "billing", "page.tsx"), "import { session } from '../auth/session'\n")
	c := Context(dir, Scan(dir).Components)
	if !hasDep(c.Deps, "src/billing", "src/auth", core.DepKindModule) {
		t.Fatalf("relative ts %+v modules %+v", c.Deps, c.Modules)
	}
}

func hasDep(ds []core.ContextDep, from, to, kind string) bool {
	for _, d := range ds {
		if d.From == from && d.To == to && (kind == "" || d.Kind == kind) {
			return true
		}
	}
	return false
}

func hasDepKindTo(ds []core.ContextDep, to, kind string) bool {
	for _, d := range ds {
		if d.To == to && d.Kind == kind {
			return true
		}
	}
	return false
}
