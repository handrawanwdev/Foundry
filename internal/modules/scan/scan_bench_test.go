package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeRepo(tb testing.TB, packages, filesPerPkg int) string {
	tb.Helper()
	dir := tb.TempDir()
	mustWrite := func(path, body string) {
		tb.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	mustWrite(filepath.Join(dir, "go.mod"), "module bench\nrequire (\n\tgithub.com/gin-gonic/gin v1\n\tgithub.com/jackc/pgx/v5 v5\n\tgithub.com/redis/go-redis/v9 v9\n)\n")
	mustWrite(filepath.Join(dir, "package.json"), `{"workspaces":["apps/*"],"devDependencies":{"eslint":"9","prettier":"3","typescript":"5"}}`)
	mustWrite(filepath.Join(dir, "apps", "web", "package.json"), `{"dependencies":{"react":"18","next":"14","@mui/material":"5"},"devDependencies":{"vitest":"2"}}`)
	mustWrite(filepath.Join(dir, "docker-compose.yml"), "services:\n  db:\n    image: postgres:16\n  cache:\n    image: redis:7\n")
	mustWrite(filepath.Join(dir, "requirements.txt"), "flask==3\npsycopg2-binary==2\ncelery==5\n")
	mustWrite(filepath.Join(dir, "Makefile"), "test:\n\tgo test ./...\n")
	body := "package p\nfunc F() {}\n"
	for i := 0; i < packages; i++ {
		pkg := filepath.Join(dir, "internal", "pkg"+itoa(i))
		for j := 0; j < filesPerPkg; j++ {
			mustWrite(filepath.Join(pkg, "f"+itoa(j)+".go"), body)
		}
		mustWrite(filepath.Join(pkg, "f_test.go"), "package p\nfunc TestF(t *testing.T) {}\n")
		mustWrite(filepath.Join(pkg, "readme.md"), "# pkg\n")
		mustWrite(filepath.Join(pkg, "icon.svg"), "<svg/>")
	}
	// Noise that Scan must skip as a directory.
	for i := 0; i < 50; i++ {
		mustWrite(filepath.Join(dir, "node_modules", "lib"+itoa(i), "index.js"), "module.exports=1\n")
	}
	return dir
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func BenchmarkScanLargeRepo(b *testing.B) {
	dir := makeRepo(b, 40, 15)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := Scan(dir)
		if !r.Existing {
			b.Fatal("expected existing project")
		}
	}
}

func BenchmarkScanFoundry(b *testing.B) {
	dir, err := filepath.Abs("../../..")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Scan(dir)
	}
}

func BenchmarkMentions(b *testing.B) {
	body := strings.Repeat("github.com/foo/bar flask-cors django-rest\n", 2000) + "flask==3\npsycopg2-binary\n"
	names := []string{"flask", "django", "psycopg2", "celery", "fastapi", "redis"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, n := range names {
			_ = mentions(body, n)
		}
	}
}
