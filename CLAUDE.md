# pkty — Claude Guide

## Project snapshot

Go terminal network monitor (`github.com/srixivas/pkty`). Entry: `cmd/pkty/main.go`. Run with `sudo ./pkty -i en0`.

## Architecture at a glance

```
capture.Backend → parser.Parser → events.EventBus → layout.Model → widgets
```

Three layers: capture (libpcap/pcapfile), parse (DNS/TLS/HTTP dissectors), TUI (bubbletea + lipgloss). The EventBus is typed buffered channels; layout registers one bubbletea `Cmd` listener per channel. No shared mutable state between goroutines — bubbletea `Cmd` is the bridge.

Key packages:
- `internal/capture/` — `Backend` interface + implementations
- `internal/events/` — typed event structs + `EventBus`
- `internal/parser/` — dissectors
- `internal/widgets/` — all TUI widgets + display filter system
- `internal/layout/` — bubbletea `Model`, orchestrates everything
- `internal/resolve/` — async reverse DNS PTR cache
- `internal/store/` — async SQLite logger
- `internal/session/` — on-demand pcap export

## Go practices for this codebase

- CGO is required for `capture/` and `store/` (libpcap + go-sqlite3). All other packages are CGO-free.
- Tests run with `CGO_ENABLED=0` for the CGO-free packages. Don't add CGO dependencies to `widgets/`, `events/`, or `resolve/`.
- bubbletea pattern: external goroutines communicate via `tea.Cmd` returning a `tea.Msg` — never touch widget state directly from outside the update loop.
- Sub-parsers publish to EventBus channels non-blocking (`select { case ch <- e: default: }`) — never block the capture goroutine.
- Widget interface: `Update(tea.Msg) (Widget, tea.Cmd)`. Side widgets gate on `!focused` at top of Update and ignore keys when unfocused.
- Focused widget border: `lipgloss.Color("226")` (bright yellow). Each widget has its own unfocused border color.
- `truncStr(s, n)` is in `packetlist.go`; `formatBytes(n)` is in `connections.go` — both accessible to all widgets in the package.
- No global mutable state. `privateBlocks` in netgraph is init-time parsed, read-only after.

## Build & test

```bash
go build ./...
go vet ./...
CGO_ENABLED=0 go test ./internal/widgets/... ./internal/events/... ./internal/resolve/... -race -count=1
```

## Release process

Cut a release by tagging and pushing — GitHub Actions does the rest:

```bash
git tag v0.2.0
git push origin v0.2.0
```

- `release.yml` triggers on `v*` tags
- Parallel builds on `ubuntu-latest` (linux/amd64 + linux/arm64 via cross-compile) and `macos-latest` (darwin/amd64 + darwin/arm64)
- goreleaser `--split` → `continue --merge` pattern for multi-runner builds
- Archives: `pkty_<os>_<arch>.tar.gz` + `checksums.txt`
- Changelog auto-generated from GitHub; commits prefixed `chore:`, `docs:`, `test:` are excluded
- No secrets needed beyond default `GITHUB_TOKEN`; repo must have Actions write permission enabled

## CI

`ci.yml` runs on every push/PR:
1. Install libpcap (`apt-get install libpcap-dev`)
2. `go vet ./...`
3. `go build ./...`
4. `CGO_ENABLED=0 go test ./internal/widgets/... ./internal/events/... ./internal/resolve/... -race -timeout 60s`

`internal/parser/` and `internal/store/` are intentionally excluded from CI tests (require CGO + sqlite headers in a specific environment).

## Known TODOs (from pkty-plan.md)

- `netgraph.go` top: wire `HTTPEvent.ContentType` for edge content-type annotation
- TCP state machine in ConnectionsWidget (ESTABLISHED/TIME_WAIT/CLOSE_WAIT not tracked)
- Process name per connection (needs eBPF or platform `/proc/net` + `lsof`)
- Geo/ASN in RemoteHostsWidget (no database wired)
- Config-driven widget enable/disable (Phase 6 — not started)
- pcap playback controls — speed, seek, export filtered (only basic offline read now)
- Help overlay (`?` key), colour theme picker
- eBPF backend (Linux-only, Phase 9)

## Display filter wiring (when adding a new widget)

1. Widget emits `DisplayFilterToggleMsg{Kind, Value, Label}` on `Enter`
2. `layout.Model.Update` handles it → `displayFilters.Toggle()` + `inspector.List().RebuildFiltered()`
3. Add a row in the Widget → Filter Mapping table in memory/MEMORY.md

## Module path

`github.com/srixivas/pkty` — all internal imports use this prefix.
