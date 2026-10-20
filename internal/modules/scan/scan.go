// Package scan inspects a project directory: which components exist
// (backend, web, mobile, ...) and which technology they already use
// (language, framework, database, cache, messaging, UI library, test and
// lint tools, repository layout, build and deployment config), so the
// design follows an existing project instead of replacing it.
package scan

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, core.StateDir: true, "dist": true, "build": true,
	"vendor": true, ".next": true, "target": true, ".dart_tool": true, "Pods": true, ".venv": true,
}

type pkgJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      json.RawMessage   `json:"workspaces"`
}

// Result of scanning a directory.
type Result struct {
	Existing   bool
	Components []core.Component
}

// Component kinds that name a technology area directly (the stack reads
// them as decisions taken by the code).
const (
	KindUI            = core.TechUI
	KindAPI           = core.TechAPI
	KindCache         = core.TechCache
	KindMessaging     = core.TechMessaging
	KindWorkers       = core.TechWorkers
	KindStorage       = core.TechStorage
	KindSearch        = core.TechSearch
	KindRepository    = core.TechRepository
	KindDeployment    = core.TechDeployment
	KindObservability = core.TechObservability
	KindTesting       = core.TechTesting
	KindQuality       = core.TechQuality
)

// Scan walks dir up to depth 4.
func Scan(dir string) Result {
	r := Result{}
	seen := map[string]bool{}
	add := func(c core.Component) {
		key := c.Kind + "|" + c.Path + "|" + c.Tech
		if !seen[key] {
			seen[key] = true
			r.Components = append(r.Components, c)
		}
	}
	code := func(c core.Component) {
		r.Existing = true
		add(c)
	}
	goTests := false
	if fi, err := os.Stat(filepath.Join(dir, ".github", "workflows")); err == nil && fi.IsDir() {
		add(core.Component{Kind: "ci", Path: ".github/workflows", Tech: "GitHub Actions", Evidence: ".github/workflows/"})
	}
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			if path != dir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			if strings.Count(rel, string(filepath.Separator)) >= 4 {
				return filepath.SkipDir
			}
			switch d.Name() {
			case "migrations", "prisma":
				add(core.Component{Kind: "database", Path: rel, Evidence: d.Name() + "/"})
			case "packages", "libs", "shared":
				add(core.Component{Kind: "shared", Path: rel, Evidence: d.Name() + "/"})
			case "android":
				add(core.Component{Kind: "mobile", Path: rel, Tech: "Android", Evidence: "android/"})
			case "ios":
				add(core.Component{Kind: "mobile", Path: rel, Tech: "iOS", Evidence: "ios/"})
			case "k8s", "helm":
				add(core.Component{Kind: "infra", Path: rel, Evidence: d.Name() + "/"})
				add(core.Component{Kind: KindDeployment, Path: rel, Tech: "Kubernetes", Evidence: d.Name() + "/"})
			case "terraform", "deploy", "infra":
				add(core.Component{Kind: "infra", Path: rel, Evidence: d.Name() + "/"})
			}
			return nil
		}
		relDir := filepath.Dir(rel)
		switch name := d.Name(); {
		case name == "go.mod":
			r.Existing = true
			scanGo(relDir, read(path), add)
		case name == "go.work":
			add(core.Component{Kind: KindRepository, Path: relDir, Tech: core.RepoMonorepo, Evidence: "go.work"})
		case name == "package.json":
			r.Existing = true
			scanNode(dir, relDir, path, add)
		case name == "pnpm-workspace.yaml" || name == "turbo.json" || name == "nx.json" || name == "lerna.json":
			add(core.Component{Kind: KindRepository, Path: relDir, Tech: core.RepoMonorepo, Evidence: name})
		case name == "pyproject.toml" || name == "requirements.txt" || name == "Pipfile":
			scanPython(relDir, name, read(path), code, add)
		case name == "Cargo.toml":
			scanManifest(relDir, name, "Rust", read(path), code, add, []dep{{"axum", "backend", "Rust + Axum"}, {"actix-web", "backend", "Rust + Actix Web"}, {"clap", "cli", "Rust + clap"}})
		case name == "pom.xml" || name == "build.gradle" || name == "build.gradle.kts":
			body := read(path)
			lang := "Java"
			if strings.HasSuffix(name, ".kts") || strings.Contains(body, "kotlin") {
				lang = "Kotlin"
			}
			if strings.Contains(body, "com.android.application") {
				code(core.Component{Kind: "mobile", Path: relDir, Tech: "Android (" + lang + ")", Language: lang, Evidence: name})
				break
			}
			scanManifest(relDir, name, lang, body, code, add, []dep{{"spring-boot", "backend", "Spring Boot + " + lang}})
		case name == "composer.json":
			scanManifest(relDir, name, "PHP", read(path), code, add, []dep{{"laravel/framework", "backend", "Laravel + PHP"}, {"symfony/framework-bundle", "backend", "Symfony + PHP"}})
		case name == "Gemfile":
			scanManifest(relDir, name, "Ruby", read(path), code, add, []dep{{"rails", "backend", "Ruby on Rails"}})
		case strings.HasSuffix(name, ".csproj"):
			if body := read(path); strings.Contains(body, "Microsoft.NET.Sdk.Web") {
				code(core.Component{Kind: "backend", Path: relDir, Tech: "ASP.NET Core + C#", Language: "C#", Evidence: name})
			} else {
				code(core.Component{Kind: "library", Path: relDir, Tech: ".NET", Language: "C#", Evidence: name})
			}
		case name == "pubspec.yaml":
			r.Existing = true
			add(core.Component{Kind: "mobile", Path: relDir, Tech: "Flutter", Language: "Dart", Evidence: "pubspec.yaml"})
		case name == "schema.prisma":
			if db := prismaDB(read(path)); db != "" {
				add(core.Component{Kind: "database", Path: relDir, Tech: db, Evidence: "schema.prisma"})
			}
		case name == "components.json" && strings.Contains(read(path), "shadcn"):
			add(core.Component{Kind: KindUI, Path: relDir, Tech: "Tailwind CSS + shadcn/ui", Evidence: "components.json"})
		case name == ".golangci.yml" || name == ".golangci.yaml":
			add(core.Component{Kind: KindQuality, Path: relDir, Tech: "golangci-lint", Language: "Go", Evidence: name})
		case name == "Dockerfile":
			add(core.Component{Kind: "infra", Path: relDir, Tech: "Docker", Evidence: name})
		case name == "docker-compose.yml" || name == "docker-compose.yaml" || name == "compose.yaml" || name == "compose.yml":
			add(core.Component{Kind: "infra", Path: relDir, Tech: "Docker", Evidence: name})
			add(core.Component{Kind: KindDeployment, Path: relDir, Tech: "Docker Compose (satu server)", Evidence: name})
			scanCompose(relDir, name, read(path), add)
		case name == "vercel.json" || name == "netlify.toml" || name == "fly.toml" || name == "render.yaml" || name == "Procfile":
			add(core.Component{Kind: KindDeployment, Path: relDir, Tech: hosts[name], Evidence: name})
		case name == "Makefile":
			add(core.Component{Kind: "build", Path: relDir, Tech: "Make", Evidence: name})
		case name == ".gitlab-ci.yml":
			add(core.Component{Kind: "ci", Path: relDir, Tech: "GitLab CI", Evidence: name})
		case strings.HasSuffix(name, ".sql"):
			add(core.Component{Kind: "database", Path: relDir, Evidence: name})
		case strings.HasSuffix(name, "_test.go"):
			goTests = true
			add(core.Component{Kind: "testing", Path: relDir, Evidence: name})
		case strings.Contains(name, ".test.") || strings.Contains(name, ".spec."):
			add(core.Component{Kind: "testing", Path: relDir, Evidence: name})
		}
		return nil
	})
	if goTests {
		add(core.Component{Kind: KindTesting, Path: ".", Tech: "go test", Language: "Go", Evidence: "*_test.go"})
	}
	sort.SliceStable(r.Components, func(i, j int) bool { return r.Components[i].Kind < r.Components[j].Kind })
	return r
}

var hosts = map[string]string{"vercel.json": "Vercel", "netlify.toml": "Netlify", "fly.toml": "Fly.io", "render.yaml": "Render", "Procfile": "PaaS (Procfile)"}

func read(path string) string {
	b, err := os.ReadFile(path)
	if err != nil || len(b) > 1<<20 {
		return ""
	}
	return string(b)
}

// dep is a dependency name that shows a component: kind and tech.
type dep struct{ name, kind, tech string }

// mentions reports whether body names dependency as a whole token.
func mentions(body, name string) bool {
	re := regexp.MustCompile(`(?i)(^|[^A-Za-z0-9_./-])` + regexp.QuoteMeta(name) + `($|[^A-Za-z0-9_-])`)
	return re.MatchString(body)
}

// scanManifest reads a manifest of language lang: the first framework
// found decides the component; without one it is a library.
func scanManifest(relDir, file, lang, body string, code, add func(core.Component), deps []dep) {
	for _, d := range deps {
		if mentions(body, d.name) {
			code(core.Component{Kind: d.kind, Path: relDir, Tech: d.tech, Language: lang, Evidence: file})
			return
		}
	}
	code(core.Component{Kind: "library", Path: relDir, Tech: lang, Language: lang, Evidence: file})
}

// infra are the data, communication and operations dependencies a
// manifest or compose file can name, per language ecosystem.
var infraDeps = map[string][]dep{
	"go": {
		{"github.com/jackc/pgx", "database", "PostgreSQL"}, {"github.com/lib/pq", "database", "PostgreSQL"}, {"gorm.io/driver/postgres", "database", "PostgreSQL"},
		{"github.com/go-sql-driver/mysql", "database", "MySQL"}, {"go.mongodb.org/mongo-driver", "database", "MongoDB"},
		{"github.com/mattn/go-sqlite3", "database", "SQLite"}, {"modernc.org/sqlite", "database", "SQLite"},
		{"github.com/redis/go-redis", KindCache, "Redis"}, {"github.com/go-redis/redis", KindCache, "Redis"},
		{"github.com/nats-io/nats.go", KindMessaging, "NATS"}, {"github.com/segmentio/kafka-go", KindMessaging, "Kafka"},
		{"github.com/IBM/sarama", KindMessaging, "Kafka"}, {"github.com/rabbitmq/amqp091-go", KindMessaging, "RabbitMQ"},
		{"github.com/hibiken/asynq", KindWorkers, "Worker terpisah + antrean"}, {"google.golang.org/grpc", KindAPI, "gRPC"},
		{"go.opentelemetry.io/otel", KindObservability, "OpenTelemetry"}, {"github.com/getsentry/sentry-go", KindObservability, "Sentry"},
	},
	"node": {
		{"pg", "database", "PostgreSQL"}, {"postgres", "database", "PostgreSQL"}, {"mysql2", "database", "MySQL"},
		{"mongodb", "database", "MongoDB"}, {"mongoose", "database", "MongoDB"}, {"better-sqlite3", "database", "SQLite"},
		{"ioredis", KindCache, "Redis"}, {"redis", KindCache, "Redis"},
		{"kafkajs", KindMessaging, "Kafka"}, {"amqplib", KindMessaging, "RabbitMQ"}, {"nats", KindMessaging, "NATS"},
		{"bullmq", KindWorkers, "Worker terpisah + antrean"},
		{"@trpc/server", KindAPI, "tRPC"}, {"@apollo/server", KindAPI, "GraphQL"}, {"graphql", KindAPI, "GraphQL"}, {"@grpc/grpc-js", KindAPI, "gRPC"},
		{"@mui/material", KindUI, "MUI (Material UI)"}, {"antd", KindUI, "Ant Design"}, {"vuetify", KindUI, "Vuetify"},
		{"primevue", KindUI, "PrimeVue"}, {"tailwindcss", KindUI, "Tailwind CSS"},
		{"vitest", KindTesting, "Vitest"}, {"jest", KindTesting, "Jest"}, {"@playwright/test", KindTesting, "Playwright (E2E)"}, {"cypress", KindTesting, "Cypress (E2E)"},
		{"@biomejs/biome", KindQuality, "Biome"},
		{"@sentry/node", KindObservability, "Sentry"}, {"@sentry/react", KindObservability, "Sentry"}, {"@opentelemetry/api", KindObservability, "OpenTelemetry"},
		{"meilisearch", KindSearch, "Meilisearch"}, {"@elastic/elasticsearch", KindSearch, "Elasticsearch / OpenSearch"},
		{"@aws-sdk/client-s3", KindStorage, "S3-compatible (S3 / MinIO)"}, {"minio", KindStorage, "S3-compatible (S3 / MinIO)"},
	},
	"python": {
		{"psycopg", "database", "PostgreSQL"}, {"psycopg2", "database", "PostgreSQL"}, {"psycopg2-binary", "database", "PostgreSQL"}, {"asyncpg", "database", "PostgreSQL"},
		{"pymysql", "database", "MySQL"}, {"mysqlclient", "database", "MySQL"}, {"pymongo", "database", "MongoDB"},
		{"redis", KindCache, "Redis"}, {"celery", KindWorkers, "Worker terpisah + antrean"}, {"arq", KindWorkers, "Worker terpisah + antrean"},
		{"pytest", KindTesting, "pytest"}, {"ruff", KindQuality, "Ruff"}, {"sentry-sdk", KindObservability, "Sentry"},
		{"boto3", KindStorage, "S3-compatible (S3 / MinIO)"},
	},
}

func scanDeps(relDir, file, eco string, has func(string) bool, add func(core.Component)) {
	for _, d := range infraDeps[eco] {
		if has(d.name) {
			add(core.Component{Kind: d.kind, Path: relDir, Tech: d.tech, Evidence: file})
		}
	}
}

func scanGo(relDir, body string, add func(core.Component)) {
	has := func(m string) bool { return strings.Contains(body, m) }
	c := core.Component{Kind: "backend", Path: relDir, Tech: "Go", Language: "Go", Evidence: "go.mod"}
	switch {
	case has("github.com/gin-gonic/gin"):
		c.Tech = "Go + Gin"
	case has("github.com/gofiber/fiber"):
		c.Tech = "Go + Fiber"
	case has("github.com/labstack/echo"):
		c.Tech = "Go + Echo"
	case has("github.com/go-chi/chi"):
		c.Tech = "Go + chi"
	case has("github.com/spf13/cobra"):
		c.Kind, c.Tech = "cli", "Go + Cobra"
	}
	add(c)
	scanDeps(relDir, "go.mod", "go", has, add)
}

func scanPython(relDir, file, body string, code, add func(core.Component)) {
	has := func(n string) bool { return mentions(body, n) }
	switch {
	case has("fastapi"):
		code(core.Component{Kind: "backend", Path: relDir, Tech: "Python + FastAPI", Language: "Python", Evidence: file})
	case has("django"):
		code(core.Component{Kind: "backend", Path: relDir, Tech: "Django + Python", Language: "Python", Evidence: file})
	case has("flask"):
		code(core.Component{Kind: "backend", Path: relDir, Tech: "Flask + Python", Language: "Python", Evidence: file})
	case has("typer"):
		code(core.Component{Kind: "cli", Path: relDir, Tech: "Python + Typer", Language: "Python", Evidence: file})
	default:
		code(core.Component{Kind: "library", Path: relDir, Tech: "Python", Language: "Python", Evidence: file})
	}
	scanDeps(relDir, file, "python", has, add)
}

func scanNode(root, relDir, path string, add func(core.Component)) {
	var p pkgJSON
	if json.Unmarshal([]byte(read(path)), &p) != nil {
		return
	}
	has := func(dep string) bool {
		_, a := p.Dependencies[dep]
		_, b := p.DevDependencies[dep]
		return a || b
	}
	lang := "JavaScript"
	if _, err := os.Stat(filepath.Join(root, relDir, "tsconfig.json")); err == nil || has("typescript") {
		lang = "TypeScript"
	}
	switch {
	case has("expo") || has("react-native"):
		add(core.Component{Kind: "mobile", Path: relDir, Tech: "React Native", Language: lang, Evidence: "package.json"})
	case has("electron"):
		add(core.Component{Kind: "desktop", Path: relDir, Tech: "Electron", Language: lang, Evidence: "package.json"})
	case has("@tauri-apps/api"):
		add(core.Component{Kind: "desktop", Path: relDir, Tech: "Tauri", Language: lang, Evidence: "package.json"})
	case has("react") || has("vue") || has("svelte") || has("next") || has("vite"):
		add(core.Component{Kind: "web", Path: relDir, Tech: webTech(has), Language: lang, Evidence: "package.json"})
	case has("@nestjs/core"):
		add(core.Component{Kind: "backend", Path: relDir, Tech: "NestJS", Language: lang, Evidence: "package.json"})
	case has("express") || has("fastify") || has("hono"):
		add(core.Component{Kind: "backend", Path: relDir, Tech: nodeTech(has), Language: lang, Evidence: "package.json"})
	case len(p.Workspaces) > 0:
		add(core.Component{Kind: "shared", Path: relDir, Tech: "Workspace root", Evidence: "package.json workspaces"})
	}
	if len(p.Workspaces) > 0 {
		add(core.Component{Kind: KindRepository, Path: relDir, Tech: core.RepoMonorepo, Evidence: "package.json workspaces"})
	}
	if has("eslint") || has("prettier") {
		t := map[[2]bool]string{{true, true}: "ESLint + Prettier", {true, false}: "ESLint", {false, true}: "Prettier"}[[2]bool{has("eslint"), has("prettier")}]
		add(core.Component{Kind: KindQuality, Path: relDir, Tech: t, Language: lang, Evidence: "package.json"})
	}
	scanDeps(relDir, "package.json", "node", has, add)
}

func webTech(has func(string) bool) string {
	switch {
	case has("next"):
		return "Next.js"
	case has("vue"):
		return "Vue"
	case has("svelte"):
		return "Svelte"
	default:
		return "React"
	}
}

func nodeTech(has func(string) bool) string {
	switch {
	case has("fastify"):
		return "Fastify"
	case has("hono"):
		return "Hono"
	default:
		return "Express"
	}
}

var prismaProvider = regexp.MustCompile(`(?s)datasource\s+\w+\s*\{[^}]*provider\s*=\s*"(\w+)"`)

func prismaDB(body string) string {
	m := prismaProvider.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return map[string]string{"postgresql": "PostgreSQL", "mysql": "MySQL", "mongodb": "MongoDB", "sqlite": "SQLite"}[m[1]]
}

var composeImage = regexp.MustCompile(`(?m)^\s*image:\s*["']?([^\s"':@]+)`)

// scanCompose reads the services a compose file runs from their images.
func scanCompose(relDir, file, body string, add func(core.Component)) {
	images := map[string]dep{
		"postgres": {"", "database", "PostgreSQL"}, "postgis/postgis": {"", "database", "PostgreSQL"}, "mysql": {"", "database", "MySQL"},
		"mongo": {"", "database", "MongoDB"}, "redis": {"", KindCache, "Redis"}, "valkey/valkey": {"", KindCache, "Valkey"},
		"nats": {"", KindMessaging, "NATS"}, "rabbitmq": {"", KindMessaging, "RabbitMQ"}, "apache/kafka": {"", KindMessaging, "Kafka"},
		"bitnami/kafka": {"", KindMessaging, "Kafka"}, "confluentinc/cp-kafka": {"", KindMessaging, "Kafka"},
		"getmeili/meilisearch": {"", KindSearch, "Meilisearch"}, "minio/minio": {"", KindStorage, "S3-compatible (S3 / MinIO)"},
		"elasticsearch": {"", KindSearch, "Elasticsearch / OpenSearch"}, "opensearchproject/opensearch": {"", KindSearch, "Elasticsearch / OpenSearch"},
	}
	for _, m := range composeImage.FindAllStringSubmatch(body, -1) {
		img := strings.TrimPrefix(strings.TrimPrefix(m[1], "docker.io/"), "library/")
		if strings.Contains(img, "elasticsearch") {
			img = "elasticsearch"
		}
		if d, ok := images[img]; ok {
			add(core.Component{Kind: d.kind, Path: relDir, Tech: d.tech, Evidence: file})
		}
	}
}
