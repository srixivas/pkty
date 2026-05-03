BINARY   := pkty
PKG      := github.com/c0d343v3r/pkty
CMD      := ./cmd/pkty
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-s -w -X main.version=$(VERSION)"

# Packages that can be tested without CGO (no libpcap/sqlite linkage needed).
# parser imports capture which imports gopacket/pcap (CGO), so it is excluded.
TEST_PKGS := ./internal/widgets/... ./internal/events/... ./internal/resolve/...

.PHONY: build run install test test-race cover vet lint snapshot clean

## build: compile the binary
build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

## run: build then run (use ARGS="..." to pass flags, e.g. make run ARGS="-r capture.pcap")
run: build
	./$(BINARY) $(ARGS)

## install: install to $GOPATH/bin (or ~/go/bin)
install:
	go install $(LDFLAGS) $(CMD)

## test: run unit tests (CGO disabled — no libpcap/sqlite headers required)
test:
	CGO_ENABLED=0 go test $(TEST_PKGS) -count=1 -timeout 60s

## test-race: run tests with race detector
test-race:
	CGO_ENABLED=0 go test $(TEST_PKGS) -count=1 -race -timeout 60s

## cover: generate HTML coverage report and open it
cover:
	CGO_ENABLED=0 go test $(TEST_PKGS) -coverprofile=coverage.out -count=1 -timeout 60s
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html 2>/dev/null || xdg-open coverage.html

## vet: run go vet
vet:
	go vet ./...

## lint: run golangci-lint (install: https://golangci-lint.run/usage/install/)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Install golangci-lint: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

## snapshot: dry-run GoReleaser build (no publish, no git tag required)
snapshot:
	@command -v goreleaser >/dev/null 2>&1 || { echo "Install goreleaser: https://goreleaser.com/install/"; exit 1; }
	goreleaser release --snapshot --clean

## clean: remove build artifacts
clean:
	rm -f $(BINARY) coverage.out coverage.html
	rm -rf dist/

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/^## //'
