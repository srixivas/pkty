package widgets

import (
	"net"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func makeRow(src, dst, proto string) PacketRow {
	return PacketRow{SrcAddr: src, DstAddr: dst, Protocol: proto}
}

func newDS() *DisplayFilterSet { return NewDisplayFilterSet() }

// ── Match (packet-row filtering) ──────────────────────────────────────────────

func TestMatch(t *testing.T) {
	t.Run("NoFilters", func(t *testing.T) {
		ds := newDS()
		if !ds.Match(makeRow("1.2.3.4:1000", "5.6.7.8:443", "TCP")) {
			t.Error("empty filter set should match everything")
		}
	})

	t.Run("FilterIP", func(t *testing.T) {
		cases := []struct {
			name    string
			filter  string
			src     string
			dst     string
			wantHit bool
		}{
			{"exact src match", "1.2.3.4", "1.2.3.4:1000", "5.6.7.8:80", true},
			{"exact dst match", "1.2.3.4", "9.9.9.9:1000", "1.2.3.4:80", true},
			{"no match", "1.2.3.4", "9.9.9.9:1000", "5.6.7.8:80", false},
			{"glob match src", "192.168.*", "192.168.1.1:1000", "8.8.8.8:53", true},
			{"glob no match", "192.168.*", "10.0.0.1:1000", "8.8.8.8:53", false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ds := newDS()
				ds.Toggle(MakeFilter(FilterIP, c.filter))
				got := ds.Match(makeRow(c.src, c.dst, "TCP"))
				if got != c.wantHit {
					t.Errorf("Match(%q→%q) = %v, want %v", c.src, c.dst, got, c.wantHit)
				}
			})
		}
	})

	t.Run("FilterProtocol", func(t *testing.T) {
		cases := []struct {
			name    string
			filter  string
			proto   string
			wantHit bool
		}{
			{"case-insensitive match", "tcp", "TCP", true},
			{"exact upper match", "UDP", "UDP", true},
			{"no match", "tcp", "UDP", false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ds := newDS()
				ds.Toggle(MakeFilter(FilterProtocol, c.filter))
				got := ds.Match(makeRow("1.2.3.4:1000", "5.6.7.8:80", c.proto))
				if got != c.wantHit {
					t.Errorf("proto=%q filter=%q: got %v, want %v", c.proto, c.filter, got, c.wantHit)
				}
			})
		}
	})

	t.Run("FilterPort", func(t *testing.T) {
		cases := []struct {
			name    string
			src     string
			dst     string
			wantHit bool
		}{
			{"dst port match", "1.2.3.4:1000", "5.6.7.8:443", true},
			{"src port match", "1.2.3.4:443", "5.6.7.8:80", true},
			{"no match", "1.2.3.4:1000", "5.6.7.8:80", false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ds := newDS()
				ds.Toggle(MakeFilter(FilterPort, "443"))
				got := ds.Match(makeRow(c.src, c.dst, "TCP"))
				if got != c.wantHit {
					t.Errorf("src=%q dst=%q: got %v, want %v", c.src, c.dst, got, c.wantHit)
				}
			})
		}
	})

	t.Run("FilterDNS", func(t *testing.T) {
		cases := []struct {
			name    string
			domain  string
			filter  string
			dstIP   string
			wantHit bool
		}{
			{"exact domain", "example.com", "example.com", "93.184.216.34", true},
			{"glob domain", "api.example.com", "*.example.com", "93.184.216.34", true},
			{"unrelated IP", "example.com", "example.com", "1.2.3.4", false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ds := newDS()
				ds.AddDNSMapping(c.domain, net.ParseIP("93.184.216.34"))
				ds.Toggle(MakeFilter(FilterDNS, c.filter))
				got := ds.Match(makeRow("10.0.0.1:1000", c.dstIP+":443", "TCP"))
				if got != c.wantHit {
					t.Errorf("domain=%q filter=%q dstIP=%q: got %v, want %v",
						c.domain, c.filter, c.dstIP, got, c.wantHit)
				}
			})
		}
	})

	t.Run("FilterTLSSNI", func(t *testing.T) {
		ds := newDS()
		ds.AddSNIMapping("secure.example.com", net.ParseIP("93.184.216.34"))
		ds.Toggle(MakeFilter(FilterTLSSNI, "secure.example.com"))

		if !ds.Match(makeRow("10.0.0.1:1000", "93.184.216.34:443", "TLS")) {
			t.Error("SNI-resolved IP should match")
		}
		if ds.Match(makeRow("10.0.0.1:1000", "1.2.3.4:443", "TLS")) {
			t.Error("unrelated IP should not match")
		}
	})

	t.Run("MultipleFilters_AND", func(t *testing.T) {
		cases := []struct {
			name    string
			proto   string
			dst     string
			wantHit bool
		}{
			{"both match", "TCP", "5.6.7.8:443", true},
			{"proto fails", "UDP", "5.6.7.8:443", false},
			{"port fails", "TCP", "5.6.7.8:80", false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ds := newDS()
				ds.Toggle(MakeFilter(FilterProtocol, "tcp"))
				ds.Toggle(MakeFilter(FilterPort, "443"))
				got := ds.Match(makeRow("1.2.3.4:1000", c.dst, c.proto))
				if got != c.wantHit {
					t.Errorf("proto=%q dst=%q: got %v, want %v", c.proto, c.dst, got, c.wantHit)
				}
			})
		}
	})
}

// ── Toggle / Clear / IsActive ─────────────────────────────────────────────────

func TestToggle(t *testing.T) {
	t.Run("AddThenRemove", func(t *testing.T) {
		ds := newDS()
		f := MakeFilter(FilterIP, "1.2.3.4")
		ds.Toggle(f)
		if !ds.Active() {
			t.Error("should be active after first toggle")
		}
		ds.Toggle(f)
		if ds.Active() {
			t.Error("should be inactive after second toggle (removal)")
		}
	})
}

func TestClear(t *testing.T) {
	ds := newDS()
	ds.Toggle(MakeFilter(FilterIP, "1.2.3.4"))
	ds.Toggle(MakeFilter(FilterProtocol, "tcp"))
	ds.Clear()
	if ds.Active() {
		t.Error("should be inactive after Clear()")
	}
}

func TestIsActive(t *testing.T) {
	cases := []struct {
		name      string
		kind      FilterKind
		value     string
		checkKind FilterKind
		checkVal  string
		want      bool
	}{
		{"toggled on", FilterPort, "443", FilterPort, "443", true},
		{"wrong value", FilterPort, "443", FilterPort, "80", false},
		{"wrong kind", FilterPort, "443", FilterProtocol, "443", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ds := newDS()
			ds.Toggle(MakeFilter(c.kind, c.value))
			got := ds.IsActive(c.checkKind, c.checkVal)
			if got != c.want {
				t.Errorf("IsActive(%d, %q) = %v, want %v", c.checkKind, c.checkVal, got, c.want)
			}
		})
	}
}

// ── MatchHost ─────────────────────────────────────────────────────────────────

func TestMatchHost(t *testing.T) {
	t.Run("NoFilters", func(t *testing.T) {
		ds := newDS()
		if !ds.MatchHost("1.2.3.4", []uint16{443}, []string{"TCP"}) {
			t.Error("no filters: MatchHost should always return true")
		}
	})

	t.Run("FilterIP", func(t *testing.T) {
		cases := []struct {
			name    string
			filter  string
			ip      string
			wantHit bool
		}{
			{"exact match", "1.2.3.4", "1.2.3.4", true},
			{"glob match", "192.168.*", "192.168.1.100", true},
			{"glob no match", "192.168.*", "10.0.0.1", false},
			{"no match", "5.5.5.5", "1.2.3.4", false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ds := newDS()
				ds.Toggle(MakeFilter(FilterIP, c.filter))
				got := ds.MatchHost(c.ip, []uint16{443}, []string{"TCP"})
				if got != c.wantHit {
					t.Errorf("filter=%q ip=%q: got %v, want %v", c.filter, c.ip, got, c.wantHit)
				}
			})
		}
	})

	t.Run("FilterDNS", func(t *testing.T) {
		// resolvedIP is what gets mapped in DNS; checkIP is the host we're testing.
		cases := []struct {
			name        string
			domain      string
			filter      string
			resolvedIP  string // IP added to the DNS mapping
			checkIP     string // IP passed to MatchHost
			wantHit     bool
		}{
			{"exact domain IP matches", "example.com", "example.com", "93.184.216.34", "93.184.216.34", true},
			{"glob domain matches", "api.google.com", "*.google.com", "142.250.80.46", "142.250.80.46", true},
			{"unrelated IP does not match", "example.com", "example.com", "93.184.216.34", "1.1.1.1", false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ds := newDS()
				ds.AddDNSMapping(c.domain, net.ParseIP(c.resolvedIP))
				ds.Toggle(MakeFilter(FilterDNS, c.filter))
				got := ds.MatchHost(c.checkIP, []uint16{443}, []string{"TCP"})
				if got != c.wantHit {
					t.Errorf("domain=%q filter=%q check=%q: got %v, want %v",
						c.domain, c.filter, c.checkIP, got, c.wantHit)
				}
			})
		}
	})

	t.Run("FilterPort", func(t *testing.T) {
		cases := []struct {
			name    string
			ports   []uint16
			filter  string
			wantHit bool
		}{
			{"port present", []uint16{22, 80}, "22", true},
			{"port absent", []uint16{80, 443}, "22", false},
			{"single port match", []uint16{443}, "443", true},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ds := newDS()
				ds.Toggle(MakeFilter(FilterPort, c.filter))
				got := ds.MatchHost("1.2.3.4", c.ports, []string{"TCP"})
				if got != c.wantHit {
					t.Errorf("ports=%v filter=%q: got %v, want %v", c.ports, c.filter, got, c.wantHit)
				}
			})
		}
	})

	t.Run("FilterProtocol", func(t *testing.T) {
		cases := []struct {
			name    string
			protos  []string
			filter  string
			wantHit bool
		}{
			{"proto present", []string{"TCP", "UDP"}, "UDP", true},
			{"proto absent", []string{"TCP"}, "UDP", false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ds := newDS()
				ds.Toggle(MakeFilter(FilterProtocol, c.filter))
				got := ds.MatchHost("1.2.3.4", []uint16{80}, c.protos)
				if got != c.wantHit {
					t.Errorf("protos=%v filter=%q: got %v, want %v", c.protos, c.filter, got, c.wantHit)
				}
			})
		}
	})
}

// ── ParseSearchPattern ────────────────────────────────────────────────────────

func TestParseSearchPattern(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantKind FilterKind
		wantVal  string
		wantOK   bool
	}{
		// IP patterns (starts with digit, only digits/dots/asterisks)
		{"exact IP", "192.168.1.1", FilterIP, "192.168.1.1", true},
		{"IP glob", "10.*", FilterIP, "10.*", true},
		// Bare integer: isIPPattern matches it first (all digits, starts with digit)
		{"bare number is IP pattern", "443", FilterIP, "443", true},
		// Port with colon prefix
		{"colon port", ":443", FilterPort, "443", true},
		{"colon port 22", ":22", FilterPort, "22", true},
		// Protocol names
		{"tcp", "tcp", FilterProtocol, "tcp", true},
		{"UDP", "udp", FilterProtocol, "udp", true},
		{"https", "https", FilterProtocol, "https", true},
		{"dns", "dns", FilterProtocol, "dns", true},
		{"ssh", "ssh", FilterProtocol, "ssh", true},
		// DNS/domain fallback
		{"domain", "example.com", FilterDNS, "example.com", true},
		{"wildcard domain", "*.google.com", FilterDNS, "*.google.com", true},
		// Empty
		{"empty", "", FilterIP, "", false},
		{"whitespace", "   ", FilterIP, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, ok := ParseSearchPattern(c.input)
			if ok != c.wantOK {
				t.Errorf("ParseSearchPattern(%q): ok=%v, want %v", c.input, ok, c.wantOK)
				return
			}
			if !ok {
				return
			}
			if f.Kind != c.wantKind {
				t.Errorf("ParseSearchPattern(%q): kind=%d, want %d", c.input, f.Kind, c.wantKind)
			}
			if c.wantVal != "" && f.Value != c.wantVal {
				t.Errorf("ParseSearchPattern(%q): value=%q, want %q", c.input, f.Value, c.wantVal)
			}
		})
	}
}

// ── extractIP / extractPort ───────────────────────────────────────────────────

func TestExtractIP(t *testing.T) {
	cases := []struct {
		addr string
		want string
	}{
		{"1.2.3.4:443", "1.2.3.4"},
		{"1.2.3.4:0", "1.2.3.4"},
		{"[::1]:443", "::1"},
		{"192.168.1.1:22", "192.168.1.1"},
		{"", ""},
	}
	for _, c := range cases {
		t.Run(c.addr, func(t *testing.T) {
			got := extractIP(c.addr)
			if got != c.want {
				t.Errorf("extractIP(%q) = %q, want %q", c.addr, got, c.want)
			}
		})
	}
}

func TestExtractPort(t *testing.T) {
	cases := []struct {
		addr string
		want string
	}{
		{"1.2.3.4:443", "443"},
		{"1.2.3.4:22", "22"},
		{"noport", ""},
	}
	for _, c := range cases {
		t.Run(c.addr, func(t *testing.T) {
			got := extractPort(c.addr)
			if got != c.want {
				t.Errorf("extractPort(%q) = %q, want %q", c.addr, got, c.want)
			}
		})
	}
}

// ── MakeFilter / Summary ──────────────────────────────────────────────────────

func TestMakeFilter(t *testing.T) {
	cases := []struct {
		kind      FilterKind
		value     string
		wantLabel string
	}{
		{FilterIP, "1.2.3.4", "IP=1.2.3.4"},
		{FilterPort, "443", "Port=443"},
		{FilterProtocol, "TCP", "Proto=TCP"},
		{FilterDNS, "example.com", "DNS=example.com"},
		{FilterTLSSNI, "example.com", "SNI=example.com"},
	}
	for _, c := range cases {
		t.Run(c.wantLabel, func(t *testing.T) {
			f := MakeFilter(c.kind, c.value)
			if f.Label != c.wantLabel {
				t.Errorf("MakeFilter label = %q, want %q", f.Label, c.wantLabel)
			}
			if f.Kind != c.kind {
				t.Errorf("MakeFilter kind = %d, want %d", f.Kind, c.kind)
			}
			if f.Value != c.value {
				t.Errorf("MakeFilter value = %q, want %q", f.Value, c.value)
			}
		})
	}
}

func TestSummary(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		ds := newDS()
		if ds.Summary() != "" {
			t.Error("empty filter set should have empty summary")
		}
	})
	t.Run("NonEmpty", func(t *testing.T) {
		ds := newDS()
		ds.Toggle(MakeFilter(FilterIP, "1.2.3.4"))
		ds.Toggle(MakeFilter(FilterPort, "443"))
		if ds.Summary() == "" {
			t.Error("active filters should produce non-empty summary")
		}
	})
}

// ── AddDNSMapping / AddSNIMapping deduplication ───────────────────────────────

func TestAddDNSMapping_Dedup(t *testing.T) {
	ds := newDS()
	ip := net.ParseIP("1.2.3.4")
	ds.AddDNSMapping("example.com", ip)
	ds.AddDNSMapping("example.com", ip)
	if n := len(ds.dnsNameToIPs["example.com"]); n != 1 {
		t.Errorf("duplicate IP should not be added: got %d entries, want 1", n)
	}
}

func TestAddSNIMapping_Dedup(t *testing.T) {
	ds := newDS()
	ip := net.ParseIP("1.2.3.4")
	ds.AddSNIMapping("example.com", ip)
	ds.AddSNIMapping("example.com", ip)
	if n := len(ds.sniToIPs["example.com"]); n != 1 {
		t.Errorf("duplicate SNI IP should not be added: got %d entries, want 1", n)
	}
}
