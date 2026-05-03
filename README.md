# pkty

A terminal-based network monitor written in Go. Captures live traffic via libpcap (or reads pcap files offline) and displays a real-time multi-panel dashboard using [bubbletea](https://github.com/charmbracelet/bubbletea) + [lipgloss](https://github.com/charmbracelet/lipgloss).

```
sudo ./pkty -i en0
```

Press `S` at any time to save everything captured so far to a pcap file. Pass `--sqlite` to continuously log all packets, DNS, and TLS events to a SQLite database.

---

## Installation

### Download binary (recommended)

Download a pre-built binary from the [Releases](../../releases) page, unpack it, and move it somewhere on your `$PATH`:

```bash
# macOS (Apple Silicon)
curl -L https://github.com/c0d343v3r/pkty/releases/latest/download/pkty_darwin_arm64.tar.gz | tar xz
sudo mv pkty /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/c0d343v3r/pkty/releases/latest/download/pkty_darwin_amd64.tar.gz | tar xz
sudo mv pkty /usr/local/bin/

# Linux (x86-64)
curl -L https://github.com/c0d343v3r/pkty/releases/latest/download/pkty_linux_amd64.tar.gz | tar xz
sudo mv pkty /usr/local/bin/
```

### Build from source

Requires Go 1.19+ with CGO enabled, and libpcap headers (`libpcap-dev` on Linux — included on macOS).

```bash
git clone https://github.com/c0d343v3r/pkty
cd pkty
make build       # produces ./pkty
```

Or via `go install` (CGO + libpcap headers required):

```bash
go install github.com/c0d343v3r/pkty/cmd/pkty@latest
```

### Prerequisites

| Platform | Requirement |
|----------|-------------|
| macOS | Nothing extra — libpcap ships with the OS |
| Linux | `sudo apt install libpcap-dev` (Debian/Ubuntu) or `sudo dnf install libpcap-devel` (Fedora) |
| Both | Run as root or grant `cap_net_raw` (see below) |

**Linux: run without sudo**
```bash
sudo setcap cap_net_raw+ep ./pkty
./pkty -i eth0
```

---

## Layout

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ Display: ◆ all traffic                                       [top filter bar]│
├──────────────────┬──────────────────────────────┬──────────────────────────┤
│  Connections [2] │   Packet Inspector [1]        │   DNS Queries      [3]   │
│                  │   ├── Packet List (scroll)    │                          │
│  IP:port  bytes  │   ├── Detail Tree (layers)    │   Name   Type  Resolved  │
│  ...             │   └── Hex Dump                │   ...                    │
│                  │                               ├──────────────────────────┤
│                  │                               │   Bandwidth              │
│                  │                               │   ▁▃▅▇ TX/RX            │
├──────────────────┴──────────────────────────────┴──────────────────────────┤
│  Proto Dist [4]        │  Remote Hosts [4+Tab]     │  TLS Inspector [4+Tab] │
│  TCP ████████  60.2%   │  google.com     10.5M ███ │  SNI      Ver  Cipher  │
│  UDP ████     20.1%    │  1.2.3.4         2.1M ██  │  ...                   │
├──────────────────────────────────────────────────────────────────────────────┤
│ [status bar]  pkty | Packets: 1234 | [Packets] | Focus: Centre | ...     │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## Usage

```bash
# Live capture (requires root / cap_net_raw)
sudo pkty -i en0

# Read from pcap file (no root needed)
pkty -r capture.pcap

# With initial BPF filter
sudo pkty -i en0 -f "tcp port 443"

# Enable SQLite logging (default path: ~/.local/share/pkty/pkty.db)
sudo pkty -i en0 --sqlite

# Enable SQLite logging to a specific path
sudo pkty -i en0 --sqlite-db ~/captures/live.db

# Print version
pkty -version
```

---

## Key Bindings

| Key | Action |
|-----|--------|
| `1` | Focus centre (Packet Inspector) |
| `2` | Focus left (Connections) |
| `3` | Focus right (DNS Queries) |
| `4` | Focus bottom panel (cycles with Tab) |
| `Tab` | Cycle bottom sub-panels: Proto Dist → Remote Hosts → TLS |
| `Tab` | In centre: cycle inspector sub-panes (Packets / Detail / Hex) |
| `n` | Toggle centre panel: Packet Inspector ↔ Network Diagram |
| `j` / `↓` | Move cursor down in focused widget |
| `k` / `↑` | Move cursor up in focused widget |
| `Enter` | Toggle display filter from focused row |
| `D` | Clear all display filters (back to wildcard) |
| `S` | Save all captured packets to a pcap file |
| `/` | Open BPF filter bar |
| `q` / `Ctrl+C` | Quit |

---

## Display Filters

The top bar always shows the current filter state:

- **`Display: ◆ all traffic`** — no filter, all packets visible
- **`Display: DNS=google.com  [D: clear]`** — filtered; packet roll shows only matching traffic

Filters are set by navigating any side widget and pressing `Enter` on a row:

| Widget | Filter kind | Example label |
|--------|-------------|---------------|
| Connections | `IP=` | `IP=142.250.80.46` |
| DNS Queries | `DNS=` | `DNS=google.com` |
| Remote Hosts | `DNS=` (if resolved) or `IP=` | `DNS=api.github.com` |
| TLS Inspector | `SNI=` | `SNI=example.com` |
| Proto Dist | `Proto=` | `Proto=TCP` |

Multiple filters combine with AND logic. Press `Enter` on an already-active filter to toggle it off. Press `D` to clear all.

`DNS=` and `SNI=` filters resolve domain names to all known IPs automatically, so traffic to any IP that domain has resolved to is included.

The `/` BPF filter is separate — it narrows what the capture backend hands to the parser (kernel-level), while display filters work on already-captured packets inside the UI.

---

## Saving Captures

### Save to pcap (`S` key)

Press `S` at any point during a live capture to snapshot everything in the current packet list to a pcap file:

```
~/.local/share/pkty/saves/pkty-20260222-143000.pcap
```

The file is fully compatible with Wireshark and `tcpdump -r`. The save runs in the background so the UI stays responsive; a confirmation appears in the status bar when done.

### SQLite logging (`--sqlite`)

Pass `--sqlite` (or `--sqlite-db <path>`) to continuously write all events to a SQLite database as they arrive:

```bash
sudo ./pkty -i en0 --sqlite
# or
sudo ./pkty -i en0 --sqlite-db ~/captures/live.db
```

Default path: `~/.local/share/pkty/pkty.db`

Three tables are written:

| Table | Contents |
|-------|----------|
| `packets` | Every captured packet — timestamp, IPs, ports, protocol, length, info, raw bytes |
| `dns_events` | DNS queries and responses — query name, record type, resolved IP, TTL |
| `tls_events` | TLS handshakes — SNI, version, cipher suite |

Writes are batched in transactions (up to 500 rows each) via an async goroutine so high packet rates never stall the TUI. The database uses WAL mode for concurrent read access while pkty is running.

Query example:
```sql
SELECT ts, src_ip, dst_ip, protocol, length, info FROM packets ORDER BY ts DESC LIMIT 50;
SELECT query_name, resolved_ip, ttl FROM dns_events WHERE is_response = 1;
```

---

## Configuration

Config file location: `~/.config/pkty/config.toml`
See `config.example.toml` for all options.

```toml
[capture]
interface    = "en0"
bpf_filter   = ""
snap_len     = 65535
promiscuous  = true

[layout]
left_panel_width    = 30
right_panel_width   = 35
bottom_panel_height = 12
```

CLI flags override the config file.

---

## Architecture

### Data Flow

```
Network interface / pcap file
         │
         ▼
  capture.Backend          ← RawPacket (bytes + timestamp)
  (libpcap / pcapfile)
         │  goroutine: ReadPacket loop
         ▼
  parser.Parser             ← decodes layers, emits typed events
         │
         ▼
  events.EventBus           ← buffered channels per event type
  (Packets / DNS / TLS / HTTP)
         │  bubbletea Cmd listeners (one per channel)
         ▼
  layout.Model              ← bubbletea Model; dispatches to widgets
         │
         ├── PacketInspector   (centre)
         ├── ConnectionsWidget (left)
         ├── DNSWidget         (right-top)
         ├── BandwidthWidget   (right-bottom)
         ├── ProtocolDistWidget (bottom-left)
         ├── RemoteHostsWidget  (bottom-centre)
         └── TLSInspectorWidget (bottom-right)
         │
         ├── store.SQLiteStore  ← async SQLite writes (optional)
         └── session.SavePcap   ← on-demand pcap export (S key)
         │
         ▼
  bubbletea renders terminal
```

### Packages

| Package | Path | Responsibility |
|---------|------|----------------|
| `main` | `cmd/pkty/` | Parses flags, loads config, wires all components, runs bubbletea |
| `config` | `internal/config/` | TOML config loading with defaults |
| `events` | `internal/events/` | Typed event structs and `EventBus` |
| `capture` | `internal/capture/` | `Backend` interface + libpcap and pcapfile implementations |
| `parser` | `internal/parser/` | Decodes `RawPacket` → typed events; sub-parsers for DNS, TLS, HTTP |
| `layout` | `internal/layout/` | bubbletea `Model`; owns all widgets, display filter set, and persistence wiring |
| `widgets` | `internal/widgets/` | Individual TUI widget implementations + display filter types |
| `resolve` | `internal/resolve/` | Transient reverse-DNS PTR cache with async lookups |
| `session` | `internal/session/` | On-demand pcap file export (`SavePcap`) |
| `store` | `internal/store/` | Async SQLite event logger (`SQLiteStore`) |

### Key Interfaces

#### `capture.Backend`
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
Implemented by `LibpcapBackend` (live capture) and `PcapFileBackend` (offline). Swap implementations without touching anything upstream.

#### `widgets.Widget`
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
Every panel implements this. `layout.Model` holds concrete types (for extra methods like `SetDisplayFilter`) but routes keyboard messages through the interface.

### `events.EventBus`

A struct of typed buffered channels:
```
Packets     chan PacketEvent
DNS         chan DNSEvent
TLS         chan TLSEvent
HTTP        chan HTTPEvent
Connections chan ConnectionEvent
Bandwidth   chan BandwidthEvent
```

`layout.Model` registers one bubbletea `Cmd` listener per channel. Each listener blocks on its channel and returns the event as a bubbletea `Msg` — this is the standard bubbletea pattern for bridging external goroutines into the update loop.

### Focus & Key Routing

`layout.Model` tracks `focusTarget` (Centre/Left/Right/Bottom) and `bottomFocus` (Proto Dist / Remote Hosts / TLS). Keys are routed exclusively to the focused widget via `routeKey()`. All other widgets silently ignore key messages (they gate on `!focused` at the top of their Update switch).

Focus key shortcuts: `1`/`2`/`3`/`4` set `focusTarget`; `Tab` on FocusBottom cycles `bottomFocus`; `Tab` on FocusCentre cycles inspector sub-panes.

### Reverse DNS (`internal/resolve/resolver.go`)

Raw IPs — especially IPv6 — are resolved to hostnames automatically using async PTR lookups. The `Resolver` keeps a transient in-memory cache for the session:

- **Observed DNS traffic** (forward responses in the capture) seeds the cache immediately via `Seed()` — no extra lookup fired.
- **Everything else** triggers an async `net.LookupAddr` PTR query (3 s timeout, max 20 concurrent goroutines).
- NXDOMAIN and failed lookups are cached as empty so they're never retried.
- Results are delivered back to the bubbletea update loop as `resolve.Msg` and immediately propagated to Remote Hosts and Network Diagram widgets.

The cache resets on exit — it's intentionally transient.

### Display Filter System (`internal/widgets/displayfilter.go`)

`DisplayFilterSet` holds a slice of active `DisplayFilter` values and DNS/SNI resolution caches:

```
DisplayFilterSet
  ├── filters []DisplayFilter     — active predicates (AND logic)
  ├── dnsNameToIPs map[string][]string  — domain → []IP (for DNS= filters)
  └── sniToIPs     map[string][]string  — SNI   → []IP (for SNI= filters)
```

Filter kinds: `FilterIP`, `FilterProtocol`, `FilterDNS`, `FilterTLSSNI`, `FilterPort`.

Side widgets emit `DisplayFilterToggleMsg` on `Enter`. `layout.Model` handles it by calling `DisplayFilterSet.Toggle()` and `PacketList.RebuildFiltered()`. The packet list maintains a `filteredRows` slice rebuilt from all rows whenever the filter changes; new packets are checked incrementally via `addPacket()`.

### Parser Sub-parsers

`parser.Parser.ProcessPacket` walks gopacket layers in order (Ethernet → IP → TCP/UDP → application). It calls sub-parsers for application payloads:

- `parseDNS` — gopacket DNS layer; emits `DNSEvent` for queries and responses
- `parseTLS` — byte-level TLS record inspection; emits `TLSEvent` from ClientHello SNI + version
- `parseHTTP` — HTTP/1.x text parsing; emits `HTTPEvent`

Each sub-parser publishes to the relevant `EventBus` channel non-blocking (`select` with `default`) to avoid stalling the capture goroutine under load.

---

## Dependencies

| Library | Purpose |
|---------|---------|
| `github.com/google/gopacket` | Packet capture, protocol decoding, and pcap file writing |
| `github.com/charmbracelet/bubbletea` | TUI event loop (Elm architecture) |
| `github.com/charmbracelet/lipgloss` | Terminal styling and layout |
| `github.com/BurntSushi/toml` | Config file parsing |
| `github.com/mattn/go-sqlite3` | SQLite driver (CGo; WAL-mode async packet logging) |
