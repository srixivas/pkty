# pkty — Terminal Network Dashboard
### Project Plan & Architecture Decisions · v0.2

---

## 1. Vision & Goals

pkty is a modular, real-time terminal network dashboard. Unlike tools such as termshark that mirror Wireshark's single-focus UI, pkty presents a full-screen terminal environment with a central packet inspector surrounded by widget panes — each giving a distinct view into live network activity.

### Core Goals

- Real-time packet capture and live traffic analysis at interactive frame rates
- Deep packet inspection in a termshark-style centre pane (packet list → detail tree → hex dump)
- Surrounding modular widgets: DNS queries, TLS/SNI, connections, geo/IP, bandwidth, protocol distribution
- Read and analyse existing `.pcap` / `.pcapng` files alongside live capture
- Cross-platform: macOS, Linux, Windows (via npcap)

### Non-Goals (v1)

- Full Wireshark-level protocol dissection (1000+ protocols)
- GUI or web-based interface — terminal only
- Network injection or active scanning — passive capture only
- Cloud/remote agent architecture — local machine first

---

## 2. Architecture Overview

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
| **Language** | `Go` | Goroutines map perfectly to the concurrent widget architecture. Sub-millisecond GC pauses, easier to maintain than Rust. |
| **TUI Framework** | `bubbletea + lipgloss` | Most mature Go TUI framework; Elm-like update model. lipgloss handles layout and styling. |
| **Capture Backend** | `gopacket / libpcap` | Industry standard (Wireshark/tcpdump). Cross-platform. Mature Go bindings. |
| **eBPF Strategy** | Optional / future | Linux-only, significant complexity. Capture layer is abstracted behind an interface for future swap-in. |
| **Protocol Parsing** | `gopacket + custom` | gopacket for L2–L4; custom pure-Go dissectors for HTTP/1.1, DNS, TLS SNI. No tshark subprocess. |
| **Config Format** | `TOML` | Human-readable; `~/.config/pkty/config.toml` |
| **Privilege Model** | `CAP_NET_RAW / sudo` | No daemon or setuid binary. Linux: `setcap cap_net_raw+ep`. macOS: `/dev/bpf` access. |

---

## 4. Build Phases

| Phase | Name | Status | Notes |
|---|---|---|---|
| 0 | **Scaffold** | ✅ Done | Repo structure, bubbletea shell, TOML config, Makefile |
| 1 | **Capture Core** | ✅ Done | libpcap + pcap file backends, capture goroutine, privilege check |
| 2 | **Parser Layer** | ✅ Done | DNS, TLS SNI, HTTP/1.1 dissectors, typed event structs |
| 3 | **Centre Panel** | ✅ Done | PacketList, DetailTree, HexDump, BPF filter bar, search |
| 4 | **Side Widgets** | ✅ Done | ConnectionsWidget, DNSWidget, BandwidthWidget (with sparkline) |
| 5 | **Bottom Strip** | ✅ Done | ProtocolDistWidget, RemoteHostsWidget, TLSInspectorWidget |
| 5b | **NetGraph** | ✅ Done | ASCII network digraph (not in original plan) — TX/RX per host/protocol |
| 5c | **Display Filters** | ✅ Done | Per-widget Enter-to-filter, AND logic, DNS/SNI IP resolution, `D` to clear |
| 5d | **Reverse DNS** | ✅ Done | Async PTR cache, seeded from observed DNS traffic |
| 5e | **Persistence** | ✅ Done | `S` key → timestamped pcap; `--sqlite` → async SQLite logging |
| 5f | **Release infra** | ✅ Done | GoReleaser + GitHub Actions (linux/darwin × amd64/arm64) |
| 6 | **Widget Registry** | ❌ Not started | Config-driven enable/disable, layout positioning in TOML, hot-reload |
| 7 | **pcap Mode** | ⚠️ Partial | Offline reading ✅ — playback speed control, seek/jump, export filtered pcap ❌ |
| 8 | **Polish** | ⚠️ Partial | Status bar ✅, mouse ✅, keybindings ✅ — colour themes ❌, help overlay ❌, man page ❌ |
| 9 | **eBPF Backend** | ❌ Not started | Linux eBPF adapter, process attribution, pre-encryption TLS capture |

---

## 5. Widget Status

### Implemented (all in `internal/widgets/`)

| Widget | File | Notes |
|---|---|---|
| PacketList | `packetlist.go` | Scrollable table, display filter, search |
| PacketDetailTree | `detailtree.go` | Collapsible layer breakdown |
| HexDump | `hexdump.go` | Raw bytes + ASCII |
| ConnectionsWidget | `connections.go` | No process name or TCP state yet |
| DNSWidget | `dns.go` | Queries + responses, NXDOMAIN highlight |
| BandwidthWidget | `bandwidth.go` | TX/RX rates + sparkline |
| ProtocolDistWidget | `protodist.go` | Bar chart by protocol |
| RemoteHostsWidget | `remotehosts.go` | By traffic volume; no geo/ASN |
| TLSInspectorWidget | `tlsinspector.go` | SNI, version, cipher; TLS 1.2/1.0 coloured |
| NetGraphWidget | `netgraph.go` | ASCII digraph per host/protocol; TX/RX bars |

### Not yet implemented

| Widget | Phase | Notes |
|---|---|---|
| HTTP Request Log | 5 optional | `HTTPEvent` is parsed and on the bus — needs a widget |
| Help overlay | 8 | Keyboard shortcut reference popup |
| Colour theme picker | 8 | Theme support in config |
| Process Tree | 9 | Needs eBPF or `/proc/net` + `lsof` |
| WebSocket Inspector | future | WS frame stream |
| ARP Watch | future | ARP table changes, spoofing detection |

---

## 6. Gaps vs Original Plan

| Feature | Plan said | Reality |
|---|---|---|
| TCP connection state | ESTABLISHED/TIME_WAIT/CLOSE_WAIT in connections widget | Not tracked — no TCP state machine |
| Process name in connections | Show process per connection | Not implemented — needs eBPF or platform-specific APIs |
| Geo / country flags in remote hosts | Country flag + ASN lookup | Not implemented — no geo database wired |
| Sparkline in proto dist | packets/sec sparkline in bottom strip | Sparkline is in BandwidthWidget; ProtoDist has bars only |
| Config-driven widget enable/disable | Phase 6 | Not started — all widgets are always shown |
| pcap playback controls | Speed, seek, export filtered | Only basic offline read; no playback controls |

---

## 7. Repository Structure

```
pkty/
├── cmd/pkty/            binary entrypoint
├── internal/
│   ├── capture/         Capture interface + libpcap + pcapfile adapters
│   ├── parser/          Protocol dissectors (DNS, TLS, HTTP)
│   ├── events/          Typed event structs + EventBus
│   ├── widgets/         All TUI widget implementations + display filter
│   ├── layout/          bubbletea Model — orchestrates all widgets
│   ├── config/          TOML config loader + defaults
│   ├── resolve/         Async reverse DNS PTR cache
│   ├── session/         On-demand pcap export
│   └── store/           Async SQLite event logger
├── pkg/                 Public packages (widget SDK — currently empty)
├── testdata/            Sample .pcap files for tests
├── docs/                Architecture diagrams
└── config.example.toml  Annotated example config
```

---

## 8. Dependencies

| Package | Purpose |
|---|---|
| `github.com/charmbracelet/bubbletea` | TUI event loop (Elm architecture) |
| `github.com/charmbracelet/lipgloss` | Terminal styling and layout |
| `github.com/google/gopacket` | Packet capture (wraps libpcap) and L2–L4 decoding |
| `github.com/BurntSushi/toml` | TOML config parsing |
| `github.com/mattn/go-sqlite3` | SQLite driver (CGO; WAL-mode async logging) |

### System dependencies

| | Linux | macOS |
|---|---|---|
| Capture | `libpcap-dev` | included in SDK |
| Privilege | `sudo` or `setcap cap_net_raw+ep` | `sudo` |
