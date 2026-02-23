# netdash — Terminal Network Dashboard
### Project Plan & Architecture Decisions · v0.1 Draft

---

## 1. Vision & Goals

netdash is a modular, real-time terminal network dashboard. Unlike tools such as termshark that mirror Wireshark's single-focus UI, netdash presents a full-screen terminal environment with a central packet inspector surrounded by configurable widget panes — each giving a distinct view into live network activity.

### Core Goals

- Real-time packet capture and live traffic analysis at interactive frame rates
- Deep packet inspection in a termshark-style centre pane (packet list → detail tree → hex dump)
- Surrounding modular widgets: DNS queries, TLS/SNI, connections, geo/IP, bandwidth, protocol distribution
- Read and analyse existing `.pcap` / `.pcapng` files alongside live capture
- User-configurable layout — default widgets ship out-of-the-box; additional widgets are opt-in
- Cross-platform: macOS, Linux, Windows (via npcap)

### Non-Goals (v1)

- Full Wireshark-level protocol dissection (1000+ protocols)
- GUI or web-based interface — terminal only
- Network injection or active scanning — passive capture only
- Cloud/remote agent architecture — local machine first

---

## 2. Architecture Overview

The system is structured in three independent layers separated by well-defined interfaces. The capture backend can be swapped (libpcap → eBPF) without touching the UI, and widgets can be added without touching the capture pipeline.

```
┌─────────────────────────────────────────────────────────┐
│  LAYER 1 — Capture Backend  (swappable interface)        │
│    libpcap adapter   (default · cross-platform)          │
│    eBPF adapter      (optional · Linux power mode)       │
│                          ▼                               │
│  LAYER 2 — Protocol Parser  (pure Go, no external deps)  │
│    gopacket (Ethernet/IP/TCP/UDP)                        │
│    → custom dissectors (DNS · HTTP · TLS SNI)            │
│    → emits typed events into per-protocol channels       │
│                          ▼                               │
│  LAYER 3 — TUI Render Loop  (bubbletea + lipgloss)       │
│    Widget goroutines subscribe to relevant channels      │
│    → update local state → render                         │
└─────────────────────────────────────────────────────────┘
```

---

## 3. Architecture Decisions

| Decision | Choice | Rationale |
|---|---|---|
| **Language** | `Go` | Goroutine-per-widget maps perfectly to the concurrent architecture. GC pauses are sub-millisecond in modern Go — fine for an interactive tool. Significantly easier to maintain and contribute to than Rust. Near-identical performance for this workload. |
| **TUI Framework** | `bubbletea + lipgloss` | Charmbracelet's bubbletea is the most mature Go TUI framework with an Elm-like update model. lipgloss handles layout and styling. bubbles provides pre-built components (tables, viewports, spinners). |
| **Capture Backend** | `gopacket / libpcap` | Industry standard, used by Wireshark/tcpdump/tshark. Cross-platform (Linux, macOS, Windows via npcap). Mature Go bindings via gopacket. Sufficient throughput for interactive monitoring on workstations. |
| **eBPF Strategy** | Optional / future | eBPF is Linux-only and adds significant complexity. Abstract the capture layer behind an interface from day one so an eBPF backend can be swapped in later — enables process attribution and pre-encryption TLS capture without refactoring. |
| **Protocol Parsing** | `gopacket + custom` | gopacket handles L2–L4 decoding. Custom pure-Go dissectors own HTTP/1.1, DNS response parsing, and TLS SNI extraction. No tshark subprocess dependency — keeps the binary self-contained. |
| **Widget System** | Goroutine-per-widget | Each widget runs in its own goroutine, subscribed to a typed channel for its relevant events. Widget state updates are decoupled from the capture pipeline. New widgets are added by implementing a `Widget` interface and registering a channel subscription. |
| **Config Format** | `TOML` | Human-readable, widely understood, first-class Go support via BurntSushi/toml. Users define which widgets are active and their layout positions in `~/.config/netdash/config.toml`. |
| **pcap Read Mode** | `gopacket OpenOfflineFile` | Same gopacket API for both live capture and offline pcap reading. The same parser and widget pipeline handles both modes transparently. |
| **Privilege Model** | `CAP_NET_RAW / sudo` | Packet capture requires elevated privileges. On Linux use `setcap cap_net_raw`. On macOS `/dev/bpf` access. Document clearly. No daemon or setuid binary. |

---

## 4. Default Widget Set

### 4.1 Centre Panel — Packet Inspector
> Primary focus of the UI, styled after termshark. Always visible, not configurable.

- **Packet list** — scrollable table with columns: No. / Time / Source / Destination / Protocol / Length / Info
- **Packet detail tree** — collapsible layer-by-layer breakdown of selected packet (Frame → Ethernet → IP → TCP/UDP → application layer)
- **Hex dump** — raw bytes with ASCII representation; highlights bytes corresponding to selected tree node

### 4.2 Left Panel — Active Connections

- Live list of all active TCP/UDP connections with state indicator (`ESTABLISHED` / `TIME_WAIT` / `CLOSE_WAIT`)
- Shows: remote host (resolved), process name, local port, bytes transferred
- Select to filter the centre packet view to that connection

### 4.3 Right Panel — DNS Queries

- Live stream of DNS queries and responses as they are captured
- Shows: queried hostname, record type (A / AAAA / CNAME / MX / SRV), resolved IP, TTL
- `NXDOMAIN` responses highlighted in amber as potentially suspicious

### 4.4 Right Panel — Bandwidth

- Per-interface TX and RX rates with horizontal bar indicators
- Rolling totals for the current session

### 4.5 Bottom Strip — Protocol Distribution

- Horizontal bar chart of protocol breakdown by packet count
- Sparkline showing packets/sec over the last 60 seconds (TX and RX)
- Protocols: TLS, TCP, UDP, DNS, HTTP, ARP, ICMP, Other

### 4.6 Bottom Strip — Remote Hosts / Geo

- Table of unique remote IPs sorted by traffic volume
- Columns: country flag, hostname/org (reverse DNS + ASN lookup), raw IP, bytes, relative traffic bar
- Unresolved IPs flagged in amber

### 4.7 Bottom Strip — TLS Inspector

- Version breakdown bar at top (TLS 1.3 / 1.2 / 1.0 proportions)
- Per-handshake table: SNI hostname, TLS version, cipher suite
- TLS 1.2 rows highlighted amber, TLS 1.0/SSL rows highlighted red

---

## 5. Optional / Future Widgets

- **HTTP Request Log** — decoded HTTP/1.1 method, URL, status code, response time, content type
- **WebSocket Inspector** — WS frame stream with opcode, payload length, decoded text frames
- **ICMP / Ping Monitor** — ICMP echo request/reply pairs with round-trip time
- **Port Scanner View** — passive detection of open/listening ports observed in traffic
- **ARP Watch** — ARP table changes, potential ARP spoofing detection
- **Process Tree** — connection-to-process mapping (Linux: `/proc/net`; macOS: `lsof`/PKTAP; full fidelity requires eBPF backend)
- **Packet Rate Alerting** — threshold-based alerts when pps or bytes/s exceed configured limits
- **Custom Filter Bookmarks** — saved BPF filter expressions with labels

---

## 6. Build Phases

| Phase | Name | Deliverables | Tag |
|---|---|---|---|
| 0 | **Scaffold** | Repo structure, Go module init, bubbletea shell with empty panes, config loader (TOML), Makefile | Foundation |
| 1 | **Capture Core** | libpcap integration via gopacket, capture goroutine, packet channel, pcap file reading, privilege docs | Foundation |
| 2 | **Parser Layer** | gopacket L2–L4 decoding, DNS dissector, TLS SNI extractor, HTTP/1.1 basic parser, typed event structs | Foundation |
| 3 | **Centre Panel** | Packet list table widget, packet detail tree widget, hex dump widget, keyboard navigation, BPF filter input | Core |
| 4 | **Side Widgets** | Connections widget, DNS widget, bandwidth widget — all wired to live event channels | Core |
| 5 | **Bottom Strip** | Protocol distribution widget with sparkline, remote hosts/geo widget, TLS inspector widget | Core |
| 6 | **Widget Registry** | Widget interface definition, config-driven enable/disable, layout positioning in TOML, hot-reload config | Extension |
| 7 | **pcap Mode** | Full offline pcap playback, playback speed control, seek/jump, export filtered pcap | Extension |
| 8 | **Polish** | Colour themes, keyboard shortcut help overlay, status bar, mouse support, man page, README | Polish |
| 9 | **eBPF Backend** | Linux eBPF capture adapter, process attribution, optional pre-encryption TLS capture (cilium/ebpf) | Advanced |

---

## 7. Repository Structure

```
netdash/
├── cmd/netdash/         # binary entrypoint
├── internal/
│   ├── capture/         # Capture interface + libpcap adapter
│   ├── capture/ebpf/    # eBPF adapter (Linux, optional build tag)
│   ├── parser/          # Protocol dissectors (DNS, TLS, HTTP)
│   ├── events/          # Typed event structs + channel definitions
│   ├── widgets/         # Widget interface + all default widget implementations
│   ├── layout/          # Pane layout engine (bubbletea model)
│   └── config/          # TOML config loader + defaults
├── pkg/                 # Public packages (widget SDK for community widgets)
├── testdata/            # Sample .pcap files for tests
├── docs/                # Architecture diagrams, this document
└── config.example.toml # Annotated example config
```

---

## 8. References & Dependencies

### 8.1 Core Go Dependencies

| Package | URL | Purpose |
|---|---|---|
| `bubbletea` | github.com/charmbracelet/bubbletea | TUI event loop and component model (Elm architecture) |
| `lipgloss` | github.com/charmbracelet/lipgloss | Terminal styling, colours, layout borders |
| `bubbles` | github.com/charmbracelet/bubbles | Pre-built TUI components: table, viewport, spinner, textinput |
| `gopacket` | github.com/google/gopacket | Packet capture (wraps libpcap) and L2–L4 decoding |
| `BurntSushi/toml` | github.com/BurntSushi/toml | TOML config file parsing |
| `cilium/ebpf` | github.com/cilium/ebpf | eBPF program loading for Linux advanced capture backend (Phase 9) |

### 8.2 System Dependencies

| Package | URL | Purpose |
|---|---|---|
| `libpcap` | https://www.tcpdump.org | Packet capture on Linux and macOS — same engine as Wireshark |
| `npcap` | https://npcap.com | Windows packet capture (Wireshark-compatible) |
| `tshark` | https://www.wireshark.org/docs/man-pages/tshark.html | Reference only — not a runtime dependency; used for comparison testing |

### 8.3 Prior Art & Inspiration

| Project | URL | Notes |
|---|---|---|
| `termshark` | github.com/gcla/termshark | Primary inspiration for centre panel UX. Wireshark in the terminal. Uses tshark as backend. |
| `btop` | github.com/aristocratos/btop | Inspiration for modular widget dashboard layout and design quality. |
| `rustnet` | github.com/RustNet/rustnet | Closest existing network TUI; single-purpose, no modular widget system. |
| `WTF Terminal` | github.com/wtfutil/wtf | Modular dashboard concept (DevOps info) — widget registry pattern reference. |
| `ratatui` | github.com/ratatui-org/ratatui | Leading Rust TUI framework — reference for widget architecture patterns. |
| `Pixie` | px.dev/pixie | Inspiration for eBPF-based pre-encryption TLS capture approach (Phase 9). |
| `awesome-tuis` | github.com/rothgar/awesome-tuis | Comprehensive list of TUI projects used for prior art research. |

### 8.4 Learning Resources

| Resource | URL | Notes |
|---|---|---|
| gopacket docs | pkg.go.dev/github.com/google/gopacket | API reference for packet capture and decoding |
| bubbletea examples | github.com/charmbracelet/bubbletea/tree/main/examples | Tutorial programs for TUI patterns |
| Go packet capture walkthrough | https://www.devdungeon.com/content/packet-capture-injection-and-analysis-gopacket | Practical Go + gopacket article |
| BPF filter syntax | https://biot.com/capstats/bpf.html | Berkeley Packet Filter expression syntax reference |
| eBPF intro | https://ebpf.io/what-is-ebpf | Conceptual overview of eBPF for Phase 9 |
| TLS 1.3 — RFC 8446 | https://www.rfc-editor.org/rfc/rfc8446 | TLS 1.3 spec — reference for SNI and handshake parsing |
| DNS — RFC 1035 | https://www.rfc-editor.org/rfc/rfc1035 | DNS wire format spec for custom DNS dissector |

---

## 9. Claude Code Bootstrap

### Initial Prompt

```
# Bootstrap netdash — Phase 0 + Phase 1

Build the scaffold and capture core for netdash, a modular terminal
network dashboard written in Go.

Stack:
- Language:  Go 1.22+
- TUI:       charmbracelet/bubbletea + lipgloss + bubbles
- Capture:   google/gopacket (libpcap)
- Config:    BurntSushi/toml

Repo layout (create this structure):
  cmd/netdash/         binary entrypoint
  internal/capture/    Capture interface + libpcap adapter
  internal/parser/     Protocol dissectors
  internal/events/     Typed event structs + channels
  internal/widgets/    Widget interface + implementations
  internal/layout/     Bubbletea model / layout engine
  internal/config/     TOML config loader

Phase 0 deliverables:
  - go.mod with all dependencies
  - bubbletea shell that renders empty placeholder panes
    (centre + left + right + bottom strip layout)
  - TOML config loader with sane defaults
  - Makefile with: build, run, test, lint targets

Phase 1 deliverables:
  - Capture interface:  type Backend interface { Open() / ReadPacket() / Close() }
  - libpcap adapter implementing the interface via gopacket
  - Capture goroutine feeding raw packets into a channel
  - pcap file reader using the same interface (gopacket.OpenOfflineFile)
  - Privilege check on startup with helpful error if CAP_NET_RAW missing

Key rules:
  - Never call gopacket directly from widget code — only through the interface
  - Use typed event structs (DNSEvent, TLSEvent, PacketEvent) in internal/events/
  - Each widget implements:  Update(event Event)  and  View() string
  - No shared mutable state between goroutines — communicate via channels only
  - Write table-driven tests for all dissectors using testdata/*.pcap files
```

### Key Guardrails for All Claude Code Sessions

- Abstract the capture backend behind a Go interface from day one
- Each widget must implement a `Widget` interface — no ad-hoc rendering
- Use typed event structs — never pass raw `gopacket` objects to widgets
- The bubbletea model receives widget state updates via a single `Msg` channel
- No shared mutable state — goroutines communicate through channels only
- Table-driven tests for all protocol dissectors using `testdata/*.pcap` samples
- Document `CAP_NET_RAW` / privilege requirements in README and in runtime error messages
