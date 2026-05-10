BINARY   := pkty
PKG      := github.com/srixivas/pkty
CMD      := ./cmd/pkty
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-s -w -X main.version=$(VERSION)"

# Packages that can be tested without CGO (no libpcap/sqlite linkage needed).
# parser imports capture which imports gopacket/pcap (CGO), so it is excluded.
TEST_PKGS := ./internal/widgets/... ./internal/events/... ./internal/resolve/...

# Colors (safe across macOS + Linux)
GREEN  := $(shell printf '\033[0;32m')
YELLOW := $(shell printf '\033[0;33m')
CYAN   := $(shell printf '\033[0;36m')
BOLD   := $(shell printf '\033[1m')
RESET  := $(shell printf '\033[0m')

.PHONY: build run install test test-race cover vet lint snapshot clean help

## 🦉 build: compile the binary
build:
	@printf '$(CYAN)→ building pkty $(VERSION)…$(RESET)\n'
	@go build $(LDFLAGS) -o $(BINARY) $(CMD)
	@printf '$(GREEN)✓ $(BINARY) ready$(RESET)\n'

## 🚀 run: build then run  (use ARGS="..." to pass flags, e.g. make run ARGS="-r capture.pcap")
run: build
	./$(BINARY) $(ARGS)

## 📦 install: install to $$GOPATH/bin (or ~/go/bin)
install:
	@printf '$(CYAN)→ installing pkty…$(RESET)\n'
	@go install $(LDFLAGS) $(CMD)
	@printf '$(GREEN)✓ installed$(RESET)\n'

## 🧪 test: run unit tests (CGO disabled — no libpcap/sqlite headers required)
test:
	@printf '$(CYAN)→ running tests…$(RESET)\n'
	CGO_ENABLED=0 go test $(TEST_PKGS) -count=1 -timeout 60s
	@printf '$(GREEN)✓ all tests passed$(RESET)\n'

## 🏁 test-race: run tests with race detector
test-race:
	@printf '$(CYAN)→ running tests with race detector…$(RESET)\n'
	CGO_ENABLED=0 go test $(TEST_PKGS) -count=1 -race -timeout 60s
	@printf '$(GREEN)✓ race check passed$(RESET)\n'

## 📊 cover: generate HTML coverage report and open it
cover:
	@printf '$(CYAN)→ generating coverage…$(RESET)\n'
	CGO_ENABLED=0 go test $(TEST_PKGS) -coverprofile=coverage.out -count=1 -timeout 60s
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html 2>/dev/null || xdg-open coverage.html
	@printf '$(GREEN)✓ coverage.html opened$(RESET)\n'

## 🔍 vet: run go vet
vet:
	@printf '$(CYAN)→ vetting…$(RESET)\n'
	@go vet ./...
	@printf '$(GREEN)✓ clean$(RESET)\n'

## 🔧 lint: run golangci-lint  (install: https://golangci-lint.run/usage/install/)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { printf '$(YELLOW)⚠  install golangci-lint: https://golangci-lint.run/usage/install/$(RESET)\n'; exit 1; }
	golangci-lint run ./...

## 📸 snapshot: dry-run GoReleaser build  (no publish, no git tag required)
snapshot:
	@command -v goreleaser >/dev/null 2>&1 || { printf '$(YELLOW)⚠  install goreleaser: https://goreleaser.com/install/$(RESET)\n'; exit 1; }
	goreleaser release --snapshot --clean

## 🧹 clean: remove build artifacts
clean:
	@printf '$(CYAN)→ cleaning…$(RESET)\n'
	@rm -f $(BINARY) coverage.out coverage.html
	@rm -rf dist/
	@printf '$(GREEN)✓ clean$(RESET)\n'

## ❓ help: list available targets
help:
	@printf '$(BOLD)pkty Makefile targets:$(RESET)\n\n'
	@grep -E '^## ' Makefile | sed 's/^## /  /'
	@printf '\n'
