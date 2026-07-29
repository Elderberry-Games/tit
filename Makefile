BINARY := tit
PKG := github.com/Elderberry-Games/tit
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT)

.PHONY: all build test cover lint vuln fmt clean

all: fmt lint vuln test

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/tit

test:
	go test -race ./...

cover:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	./scripts/check-coverage.sh coverage.out 85

lint:
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || (echo "gofmt found issues"; exit 1)
	go vet ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

fmt:
	gofmt -w .

clean:
	rm -f $(BINARY) coverage.out coverage.html
