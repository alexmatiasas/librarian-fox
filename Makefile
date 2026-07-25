.PHONY: build test lint format clean schema

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build: schema
	CGO_ENABLED=1 go build -tags "sqlite_fts5" -ldflags "$(LDFLAGS)" -o lfx ./cmd/lfx

test:
	CGO_ENABLED=1 go test -tags "sqlite_fts5" -race -count=1 ./...

lint:
	golangci-lint run

format:
	gofumpt -w -extra .

clean:
	rm -f lfx
	rm -rf dist/

# Regenerate JSON schema and add empty required arrays for VS Code compatibility
schema:
	go generate ./internal/config/
