# pkty — Developer Guide

> Quick start: `sudo ./pkty -i en0` · Build: `go build ./cmd/pkty` · Test: `CGO_ENABLED=0 go test ./internal/widgets/... ./internal/events/... ./internal/resolve/...`

---

## 📁 Repository Structure

```
pkty/
├── cmd/pkty/            main — flags, config wiring, bubbletea start
├── internal/
│   ├── capture/         Backend interface + libpcap + pcapfile adapters
│   ├── parser/          Protocol dissectors (DNS, TLS, HTTP/1.x)
│   ├── events/          Typed event structs + EventBus
│   ├── widgets/         All TUI widget implementations + display filter types
│   ├── layout/          bubbletea Model — orchestrates all widgets
│   ├── config/          TOML loader + defaults
│   ├── resolve/         Async reverse DNS PTR cache
│   ├── session/         On-demand pcap export
│   └── store/           Async SQLite event logger
├── testdata/            Sample pcap files
├── docs/                Architecture diagrams
├── config.example.toml  Annotated config reference
├── .goreleaser.yml      Cross-platform release config
└── .github/workflows/   CI (ci.yml) + release (release.yml)
```

---

## 🛠️ Dev Setup

### System dependencies

| Platform | Required |
|----------|----------|
| macOS | Xcode Command Line Tools (includes libpcap) |
| Linux | `sudo apt install libpcap-dev gcc` (Debian/Ubuntu) · `sudo dnf install libpcap-devel gcc` (Fedora) |

### Go version

1.19+ required. CGO must be enabled for capture and SQLite packages.

### Build & run

```bash
go build -o pkty ./cmd/pkty
sudo ./pkty -i en0

# or from source directly
sudo go run ./cmd/pkty -i en0

# offline replay (no root needed)
pkty -r capture.pcap
```

### Tests

CGO-free packages (widgets, events, resolve) can be tested without libpcap headers:

```bash
CGO_ENABLED=0 go test ./internal/widgets/... ./internal/events/... ./internal/resolve/... -race -count=1
```

`internal/parser/` and `internal/store/` require CGO and are excluded from CI's test step. Run them locally with:

```bash
go test ./internal/parser/... ./internal/store/...
```

### Vet & lint

```bash
go vet ./...
```

---

## 🏗️ Architecture

### Data flow

```
Network interface / pcap file
        │
        ▼
capture.Backend              ← RawPacket{bytes, timestamp, linkType}
(LibpcapBackend / PcapFileBackend)
        │  goroutine: ReadPacket loop
        ▼
parser.Parser                ← decodes gopacket layers, fires sub-parsers
        │
        ▼
events.EventBus              ← typed buffered channels (cap 8192)
  Packets · DNS · TLS · HTTP · Connections · Bandwidth
        │  bubbletea Cmd listeners (one per channel)
        ▼
layout.Model                 ← bubbletea Model; dispatches to widgets
        │
        ├── PacketInspector   (centre — list / detail tree / hex dump)
        ├── NetGraphWidget    (centre alternate — toggled with n)
        ├── ConnectionsWidget (left)
        ├── DNSWidget         (right-top)
        ├── BandwidthWidget   (right-bottom)
        ├── ProtocolDistWidget (bottom-left)
        ├── RemoteHostsWidget  (bottom-centre)
        └── TLSInspectorWidget (bottom-right)
        │
        ├── store.SQLiteStore  ← async batched writes (optional, --sqlite)
        └── session.SavePcap   ← on-demand pcap snapshot (S key)
```

### Key interfaces

**`capture.Backend`** — swap capture implementations without touching upstream:
```go
type Backend interface {
    Open() error
    ReadPacket() (*RawPacket, error)
    Close() error
    ListInterfaces() ([]InterfaceInfo, error)
    SetBPFFilter(expr string) error
    LinkType() layers.LinkType
}
```

**`widgets.Widget`** — every panel implements this; layout routes keys through it:
```go
type Widget interface {
    Name() string
    Init() tea.Cmd
    Update(tea.Msg) (Widget, tea.Cmd)
    View() string
    SetSize(w, h int)
    SetFocused(bool)
    Focused() bool
}
```

### EventBus

Typed buffered channels (cap 8192 each). `layout.Model` registers one bubbletea `Cmd` listener per channel — each blocks on its channel and returns the event as a `tea.Msg`. This is the standard bubbletea pattern for bridging external goroutines into the update loop.

```go
type EventBus struct {
    Packets     chan PacketEvent
    DNS         chan DNSEvent
    TLS         chan TLSEvent
    HTTP        chan HTTPEvent
    Connections chan ConnectionEvent
    Bandwidth   chan BandwidthEvent
}
```

Sub-parsers publish with a non-blocking `select { case ch <- event: default: }` so a slow UI never stalls the capture goroutine.

### Focus & key routing

`layout.Model` tracks `focusTarget` (`FocusCentre=1 / FocusLeft=2 / FocusRight=3 / FocusBottom=4`) and `bottomFocus` (0=ProtoDist / 1=RemoteHosts / 2=TLS). Keys are routed exclusively to the focused widget via `routeKey()`. All other widgets gate on `!focused` at the top of their Update switch and silently ignore keys.

### Reverse DNS (`internal/resolve/resolver.go`)

- `Seed(ip, name)` — called from DNS event handler; prevents redundant PTR lookups for observed traffic
- `Resolve(ip) tea.Cmd` — fires async `net.LookupAddr` (3s timeout, semaphore cap=20); returns nil if already cached/pending
- `Name(ip) string` — synchronous cache read
- NXDOMAIN and failures cached as `""` — never retried
- Results delivered as `resolve.Msg`; layout calls `remoteHosts.AddIPName` + `netGraph.AddIPName`
- Cache is transient (resets on exit by design)

### Display filter system (`internal/widgets/displayfilter.go`)

```
DisplayFilterSet
  ├── filters []DisplayFilter     — active predicates (AND logic)
  ├── dnsNameToIPs map[string][]string  — domain → []IP (DNS= filters)
  └── sniToIPs     map[string][]string  — SNI   → []IP (SNI= filters)
```

Side widgets emit `DisplayFilterToggleMsg` on `Enter`. `layout.Model` handles it → calls `DisplayFilterSet.Toggle()` + `PacketList.RebuildFiltered()`. `PacketList` maintains a `filteredRows` slice rebuilt from all rows on filter change; new packets are checked incrementally in `addPacket()`.

Filter kinds: `FilterIP`, `FilterProtocol`, `FilterDNS`, `FilterTLSSNI`, `FilterPort`.

### Parser sub-parsers

`parser.Parser.ProcessPacket` walks gopacket layers (Ethernet → IP → TCP/UDP → application) and calls:

- `parseDNS` — gopacket DNS layer → `DNSEvent` for queries and responses
- `parseTLS` — byte-level TLS record inspection → `TLSEvent` from ClientHello SNI + version
- `parseHTTP` — HTTP/1.x text parsing → `HTTPEvent`

### NetGraph widget (`internal/widgets/netgraph.go`)

ASCII digraph: `[iface · source]` → `ngHost` nodes (per remote IP) → `ngEdge` rows (per proto:port). Direction via RFC 1918: `srcPriv && !dstPriv` = TX (green `──▶`), `!srcPriv && dstPriv` = RX (cyan `◀──`). `maxEdgeBytes` scales TX and RX bars on a shared global axis.

### SQLite store (`internal/store/sqlite.go`)

Async batched writes: packets arrive on a buffered channel (cap 8192); a background goroutine drains it in transactions of up to 500 rows. WAL mode enabled for concurrent reads while pkty runs.

Tables: `packets`, `dns_events`, `tls_events`.

---

## 🧑‍💻 Go Conventions in This Codebase

- Widget border color: focused = `Color("226")` (bright yellow); each widget has its own unfocused color
- `truncStr(s, n)` lives in `packetlist.go` — accessible to all widgets in the package
- `formatBytes(n)` lives in `connections.go` — same
- No global state outside `privateBlocks` in netgraph (init-time parsed, read-only after)
- bubbletea `Cmd` is the only way to communicate from outside the update loop into it — no shared mutable state between goroutines

---

## 📦 Dependencies

### Go modules

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/charmbracelet/bubbletea` | v0.25.0 | TUI event loop (Elm architecture) |
| `github.com/charmbracelet/lipgloss` | v0.9.1 | Terminal styling and layout |
| `github.com/google/gopacket` | v1.1.19 | Packet capture (wraps libpcap) and L2–L4 decoding |
| `github.com/BurntSushi/toml` | v1.3.2 | TOML config parsing |
| `github.com/mattn/go-sqlite3` | v1.14.34 | SQLite driver (CGO; WAL-mode async logging) |

### System libraries

| | Linux | macOS |
|--|-------|-------|
| Capture | `libpcap-dev` | included in SDK |
| SQLite | included in go-sqlite3 (statically linked) | same |
| Cross-compile arm64 | `gcc-aarch64-linux-gnu` (CI only) | native |

---

## 🚢 CI / Release

### CI (`ci.yml`)

Runs on every push and PR to any branch:
- `go vet ./...`
- `go build ./...`
- `CGO_ENABLED=0 go test ./internal/widgets/... ./internal/events/... ./internal/resolve/... -race`

### Release (`release.yml`)

Triggered by any `v*` tag. Parallel builds on `ubuntu-latest` and `macos-latest` via goreleaser `--split`, then merged into a single GitHub Release.

Targets: `linux/amd64`, `linux/arm64` (cross-compiled), `darwin/amd64`, `darwin/arm64`.

**To cut a release:**
```bash
git tag v0.2.0
git push origin v0.2.0
```

Goreleaser reads `.goreleaser.yml`:
- Strips debug info (`-s -w`) and injects `main.version`
- Archives as `pkty_<os>_<arch>.tar.gz` with `checksums.txt`
- Changelog auto-generated from GitHub; `chore`, `docs`, `test` commits excluded

### Required GitHub settings

- Actions → General → Workflow permissions: **Read and write** (for `GITHUB_TOKEN` to create releases)
- No extra secrets needed beyond the default `GITHUB_TOKEN`
