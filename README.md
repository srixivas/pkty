# netdash

A terminal-based network monitor written in Go. Captures live traffic via libpcap (or reads pcap files offline) and displays a real-time multi-panel dashboard using [bubbletea](https://github.com/charmbracelet/bubbletea) + [lipgloss](https://github.com/charmbracelet/lipgloss).

```
sudo ./netdash -i en0
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
│ [status bar]  netdash | Packets: 1234 | [Packets] | Focus: Centre | ...     │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## Building

Requires Go 1.19+ and libpcap headers (`libpcap-dev` on Linux, included on macOS).

```bash
# Build
make build          # or: go build -o netdash ./cmd/netdash

# Live capture (requires root / cap_net_raw)
sudo ./netdash -i en0

# Read from pcap file
./netdash -r capture.pcap

# With initial BPF filter
sudo ./netdash -i en0 -f "tcp port 443"

# Print version
./netdash -version
```

### Linux capability (no sudo)
```bash
sudo setcap cap_net_raw+ep ./netdash
./netdash -i eth0
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
| `j` / `↓` | Move cursor down in focused widget |
| `k` / `↑` | Move cursor up in focused widget |
| `Enter` | Toggle display filter from focused row |
| `D` | Clear all display filters (back to wildcard) |
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

## Configuration

Config file location: `~/.config/netdash/config.toml`
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
         ▼
  bubbletea renders terminal
```

### Packages

| Package | Path | Responsibility |
|---------|------|----------------|
| `main` | `cmd/netdash/` | Parses flags, loads config, wires all components, runs bubbletea |
| `config` | `internal/config/` | TOML config loading with defaults |
| `events` | `internal/events/` | Typed event structs and `EventBus` |
| `capture` | `internal/capture/` | `Backend` interface + libpcap and pcapfile implementations |
| `parser` | `internal/parser/` | Decodes `RawPacket` → typed events; sub-parsers for DNS, TLS, HTTP |
| `layout` | `internal/layout/` | bubbletea `Model`; owns all widgets and the display filter set |
| `widgets` | `internal/widgets/` | Individual TUI widget implementations + display filter types |

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
| `github.com/google/gopacket` | Packet capture and protocol decoding |
| `github.com/charmbracelet/bubbletea` | TUI event loop (Elm architecture) |
| `github.com/charmbracelet/lipgloss` | Terminal styling and layout |
| `github.com/BurntSushi/toml` | Config file parsing |
