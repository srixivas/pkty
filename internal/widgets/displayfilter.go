package widgets

import (
	"fmt"
	"net"
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
		return srcIP == f.Value || dstIP == f.Value

	case FilterProtocol:
		return strings.EqualFold(row.Protocol, f.Value)

	case FilterDNS:
		ips := d.dnsNameToIPs[f.Value]
		if len(ips) == 0 {
			return false
		}
		srcIP := extractIP(row.SrcAddr)
		dstIP := extractIP(row.DstAddr)
		for _, ip := range ips {
			if srcIP == ip || dstIP == ip {
				return true
			}
		}
		return false

	case FilterTLSSNI:
		ips := d.sniToIPs[f.Value]
		if len(ips) == 0 {
			return false
		}
		srcIP := extractIP(row.SrcAddr)
		dstIP := extractIP(row.DstAddr)
		for _, ip := range ips {
			if srcIP == ip || dstIP == ip {
				return true
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
