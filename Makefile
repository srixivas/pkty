BINARY   := netdash
PKG      := github.com/c0d343v3r/netdash
CMD      := ./cmd/netdash
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-X main.version=$(VERSION)"

.PHONY: build run test lint clean

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

test:
	go test ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Install golangci-lint: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

clean:
	rm -f $(BINARY)
