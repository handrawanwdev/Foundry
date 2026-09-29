WEB := apps/web
BIN := bin/forge

.PHONY: all web build test check dev-api dev-web clean

all: build

web:
	cd $(WEB) && npm ci && npm run build && touch dist/.gitkeep

build: web
	go build -o $(BIN) ./cmd/forge

test:
	go vet ./...
	go test ./...
	cd $(WEB) && npm run typecheck && npm test

check: test build

# Development: API on :8080, Vite on :5173 (proxies /api).
dev-api:
	go run ./cmd/forge serve

dev-web:
	cd $(WEB) && npm run dev

clean:
	rm -rf bin $(WEB)/dist/assets $(WEB)/dist/index.html
