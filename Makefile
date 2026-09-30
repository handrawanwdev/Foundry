WEB := apps/web
# Binary suffix follows target OS (".exe" on Windows, empty elsewhere; respects GOOS).
EXE := $(shell go env GOEXE)
BIN := bin/foundry$(EXE)

ifeq ($(OS),Windows_NT)
# Force cmd.exe so recipes behave the same whether or not sh.exe is on PATH.
SHELL := cmd.exe
WEB_WIN := $(subst /,\,$(WEB))
endif

.PHONY: all web build install test check dev-api dev-web clean

all: build

web:
ifeq ($(OS),Windows_NT)
	cd $(WEB) && npm ci && npm run build && copy /y nul dist\.gitkeep >nul
else
	cd $(WEB) && npm ci && npm run build && touch dist/.gitkeep
endif

build: web
	go build -o $(BIN) ./cmd/foundry

# Installs foundry to $GOBIN (default $GOPATH/bin) so it runs from any folder.
install: web
	go install ./cmd/foundry

test:
	go vet ./...
	go test ./...
	cd $(WEB) && npm run typecheck && npm test

check: test build

# Development: API on :8080, Vite on :5173 (proxies /api).
dev-api:
	go run ./cmd/foundry serve

dev-web:
	cd $(WEB) && npm run dev

clean:
ifeq ($(OS),Windows_NT)
	if exist bin rmdir /s /q bin
	if exist $(WEB_WIN)\dist\assets rmdir /s /q $(WEB_WIN)\dist\assets
	if exist $(WEB_WIN)\dist\index.html del /q $(WEB_WIN)\dist\index.html
else
	rm -rf bin $(WEB)/dist/assets $(WEB)/dist/index.html
endif
