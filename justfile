# justfile for librarian_fox (lfx) — Go CLI

VERSION := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
COMMIT  := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
LDFLAGS := "-s -w -X main.version=" + VERSION + " -X main.commit=" + COMMIT

default: build

build: schema
    CGO_ENABLED=1 go build -tags "sqlite_fts5" -ldflags "{{ LDFLAGS }}" -o lfx ./cmd/lfx

test:
    CGO_ENABLED=1 go test -tags "sqlite_fts5" -race -count=1 ./...

lint:
    golangci-lint run

format:
    gofumpt -w -extra .

clean:
    rm -f lfx
    rm -rf dist/

schema:
    go generate ./internal/config/

check:
    golangci-lint run
    CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...
    CGO_ENABLED=1 go test -tags "sqlite_fts5" -race -count=1 ./...
