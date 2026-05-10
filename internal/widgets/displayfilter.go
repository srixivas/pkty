package widgets

import (
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
)

// FilterKind identifies what kind of display filter is being applied.
type FilterKind int

const (
	FilterIP       FilterKind = iota // match src or dst IP
	FilterProtocol                   // match protocol name
	FilterDNS                        // match domain name → resolved IPs
	FilterTLSSNI                     // match TLS SNI → resolved IPs
	FilterPort                       // match src or dst port
)

// DisplayFilter is a single filter predicate.
type DisplayFilter struct {
	Kind  FilterKind
	Value string // e.g. "1.2.3.4", "TCP", "example.com"
	Label string // human-readable display
}

// DisplayFilterToggleMsg is sent by side widgets when Enter is pressed on a row.
type DisplayFilterToggleMsg struct {
	Filter DisplayFilter
}

// DisplayFilterSet holds the active set of display filters plus lookup caches
// for resolving DNS/TLS domain names to IPs.
type DisplayFilterSet struct {
	filters      []DisplayFilter
	dnsNameToIPs map[string][]string // domain → []IP strings
	sniToIPs     map[string][]string // SNI → []IP strings
}

func NewDisplayFilterSet() *DisplayFilterSet {
	return &DisplayFilterSet{
		dnsNameToIPs: make(map[string][]string),
		sniToIPs:     make(map[string][]string),
	}
}

// AddDNSMapping records that a domain name resolved to an IP address.
func (d *DisplayFilterSet) AddDNSMapping(name string, ip net.IP) {
	if ip == nil {
		return
	}
	ipStr := ip.String()
	for _, existing := range d.dnsNameToIPs[name] {
		if existing == ipStr {
			return
		}
	}
	d.dnsNameToIPs[name] = append(d.dnsNameToIPs[name], ipStr)
}

// AddSNIMapping records that a TLS SNI was seen communicating with a given IP.
func (d *DisplayFilterSet) AddSNIMapping(sni string, ip net.IP) {
	if ip == nil {
		return
	}
	ipStr := ip.String()
	for _, existing := range d.sniToIPs[sni] {
		if existing == ipStr {
			return
		}
	}
	d.sniToIPs[sni] = append(d.sniToIPs[sni], ipStr)
}

// Toggle adds the filter if not present; removes it if already present.
func (d *DisplayFilterSet) Toggle(f DisplayFilter) {
	for i, existing := range d.filters {
		if existing.Kind == f.Kind && existing.Value == f.Value {
			d.filters = append(d.filters[:i], d.filters[i+1:]...)
			return
		}
	}
	d.filters = append(d.filters, f)
}

// Clear removes all active display filters.
func (d *DisplayFilterSet) Clear() {
	d.filters = d.filters[:0]
}

// Active returns true if any display filter is set.
func (d *DisplayFilterSet) Active() bool {
	return len(d.filters) > 0
}

// IsActive returns whether a specific filter is currently toggled on.
func (d *DisplayFilterSet) IsActive(kind FilterKind, value string) bool {
	for _, f := range d.filters {
		if f.Kind == kind && f.Value == value {
			return true
		}
	}
	return false
}

// Match returns true if the PacketRow satisfies ALL active filters (AND logic).
// When no filters are active it always returns true.
func (d *DisplayFilterSet) Match(row PacketRow) bool {
	for _, f := range d.filters {
		if !d.matchOne(f, row) {
			return false
		}
	}
	return true
}

func (d *DisplayFilterSet) matchOne(f DisplayFilter, row PacketRow) bool {
	switch f.Kind {
	case FilterIP:
		srcIP := extractIP(row.SrcAddr)
		dstIP := extractIP(row.DstAddr)
		srcOK, _ := filepath.Match(f.Value, srcIP)
		dstOK, _ := filepath.Match(f.Value, dstIP)
		return srcOK || dstOK

	case FilterProtocol:
		matched, _ := filepath.Match(strings.ToLower(f.Value), strings.ToLower(row.Protocol))
		return matched

	case FilterDNS:
		srcIP := extractIP(row.SrcAddr)
		dstIP := extractIP(row.DstAddr)
		for domain, ips := range d.dnsNameToIPs {
			ok, _ := filepath.Match(f.Value, domain)
			if !ok {
				continue
			}
			for _, ip := range ips {
				if ip == srcIP || ip == dstIP {
					return true
				}
			}
		}
		return false

	case FilterTLSSNI:
		srcIP := extractIP(row.SrcAddr)
		dstIP := extractIP(row.DstAddr)
		for sni, ips := range d.sniToIPs {
			ok, _ := filepath.Match(f.Value, sni)
			if !ok {
				continue
			}
			for _, ip := range ips {
				if ip == srcIP || ip == dstIP {
					return true
				}
			}
		}
		return false

	case FilterPort:
		srcPort := extractPort(row.SrcAddr)
		dstPort := extractPort(row.DstAddr)
		return srcPort == f.Value || dstPort == f.Value
	}
	return true
}

// MatchHost returns true if a remote host (identified by IP, its edge ports,
// and its edge proto names) passes ALL active display filters.
// Always true when no filters are set.
func (d *DisplayFilterSet) MatchHost(ip string, ports []uint16, protos []string) bool {
	if len(d.filters) == 0 {
		return true
	}
	for _, f := range d.filters {
		if !d.matchHostOne(f, ip, ports, protos) {
			return false
		}
	}
	return true
}

func (d *DisplayFilterSet) matchHostOne(f DisplayFilter, ip string, ports []uint16, protos []string) bool {
	switch f.Kind {
	case FilterIP:
		ok, _ := filepath.Match(f.Value, ip)
		return ok
	case FilterDNS:
		for domain, ips := range d.dnsNameToIPs {
			if ok, _ := filepath.Match(f.Value, domain); !ok {
				continue
			}
			for _, resolved := range ips {
				if resolved == ip {
					return true
				}
			}
		}
		return false
	case FilterTLSSNI:
		for sni, ips := range d.sniToIPs {
			if ok, _ := filepath.Match(f.Value, sni); !ok {
				continue
			}
			for _, resolved := range ips {
				if resolved == ip {
					return true
				}
			}
		}
		return false
	case FilterPort:
		for _, p := range ports {
			if strconv.Itoa(int(p)) == f.Value {
				return true
			}
		}
		return false
	case FilterProtocol:
		for _, proto := range protos {
			if matched, _ := filepath.Match(strings.ToLower(f.Value), strings.ToLower(proto)); matched {
				return true
			}
		}
		return false
	}
	return true
}

// Summary returns a human-readable description of active filters.
func (d *DisplayFilterSet) Summary() string {
	if len(d.filters) == 0 {
		return ""
	}
	parts := make([]string, 0, len(d.filters))
	for _, f := range d.filters {
		parts = append(parts, f.Label)
	}
	return strings.Join(parts, " AND ")
}

// MakeFilter builds a DisplayFilter with a formatted Label.
func MakeFilter(kind FilterKind, value string) DisplayFilter {
	return DisplayFilter{
		Kind:  kind,
		Value: value,
		Label: fmt.Sprintf("%s=%s", kindStr(kind), value),
	}
}

func kindStr(k FilterKind) string {
	switch k {
	case FilterIP:
		return "IP"
	case FilterProtocol:
		return "Proto"
	case FilterDNS:
		return "DNS"
	case FilterTLSSNI:
		return "SNI"
	case FilterPort:
		return "Port"
	}
	return "Filter"
}

// extractIP strips the port from "ip:port" to return just the IP.
func extractIP(addr string) string {
	if addr == "" {
		return ""
	}
	// IPv6 bracket notation [::1]:port
	if addr[0] == '[' {
		if end := strings.Index(addr, "]"); end >= 0 {
			return addr[1:end]
		}
	}
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		before := addr[:idx]
		// bare IPv6 has multiple colons in the 'before' segment
		if strings.Count(before, ":") > 0 {
			return addr
		}
		return before
	}
	return addr
}

// extractPort returns just the port from "ip:port".
func extractPort(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		return addr[idx+1:]
	}
	return ""
}

// knownProtocols is the allow-list for protocol name auto-detection.
var knownProtocols = map[string]bool{
	"tcp": true, "udp": true, "http": true, "https": true,
	"tls": true, "dns": true, "ssh": true, "ftp": true,
	"quic": true, "icmp": true, "arp": true,
}

// ParseSearchPattern auto-detects the filter kind from a typed search string
// and returns the appropriate DisplayFilter. Returns (DisplayFilter{}, false)
// for empty input.
//
// Detection order:
//  1. IP pattern    — starts with a digit and contains only [0-9.*]
//  2. Port pattern  — starts with ":" OR is a pure integer ≤ 65535
//  3. Protocol name — matches knownProtocols allow-list
//  4. Default       — treat as DNS/domain wildcard (FilterDNS)
func ParseSearchPattern(s string) (DisplayFilter, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DisplayFilter{}, false
	}

	// 1. IP pattern
	if isIPPattern(s) {
		return MakeFilter(FilterIP, s), true
	}

	// 2. Port pattern — ":443" or "443"
	portStr := s
	if strings.HasPrefix(s, ":") {
		portStr = s[1:]
	}
	if n, err := strconv.Atoi(portStr); err == nil && n >= 0 && n <= 65535 {
		return MakeFilter(FilterPort, portStr), true
	}

	// 3. Protocol allow-list
	if knownProtocols[strings.ToLower(s)] {
		return MakeFilter(FilterProtocol, strings.ToLower(s)), true
	}

	// 4. Default: domain/SNI wildcard
	return MakeFilter(FilterDNS, s), true
}

// isIPPattern returns true when s looks like an IP address or IP glob pattern
// (starts with a digit, contains only digits, dots, and asterisks).
func isIPPattern(s string) bool {
	if len(s) == 0 || s[0] < '0' || s[0] > '9' {
		return false
	}
	for _, ch := range s {
		if ch != '.' && ch != '*' && (ch < '0' || ch > '9') {
			return false
		}
	}
	return true
}
