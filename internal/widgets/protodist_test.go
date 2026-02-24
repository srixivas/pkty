package widgets

import "testing"

// ── servicePortName ───────────────────────────────────────────────────────────

func TestServicePortName(t *testing.T) {
	t.Run("KnownPorts", func(t *testing.T) {
		cases := []struct {
			port uint16
			want string
		}{
			{80, "HTTP"},
			{8080, "HTTP"},
			{443, "HTTPS"},
			{8443, "HTTPS"},
			{22, "SSH"},
			{53, "DNS"},
			{21, "FTP"},
			{25, "SMTP"},
			{587, "SMTP"},
			{465, "SMTPS"},
			{110, "POP3"},
			{143, "IMAP"},
			{993, "IMAPS"},
			{3306, "MySQL"},
			{5432, "Postgres"},
			{6379, "Redis"},
			{27017, "MongoDB"},
			{5353, "mDNS"},
			{67, "DHCP"},
			{68, "DHCP"},
			{123, "NTP"},
			{3389, "RDP"},
			{5900, "VNC"},
		}
		for _, c := range cases {
			got := servicePortName(c.port)
			if got != c.want {
				t.Errorf("servicePortName(%d) = %q, want %q", c.port, got, c.want)
			}
		}
	})

	t.Run("UnknownPorts", func(t *testing.T) {
		for _, port := range []uint16{0, 1, 9999, 12345, 65535} {
			if got := servicePortName(port); got != "" {
				t.Errorf("servicePortName(%d) = %q, want empty string", port, got)
			}
		}
	})
}

// ── effectiveProto ────────────────────────────────────────────────────────────

func TestEffectiveProto(t *testing.T) {
	cases := []struct {
		name      string
		proto     string
		src, dst  uint16
		wantProto string
	}{
		// Non-TCP/UDP pass through unchanged
		{"ICMP pass-through", "ICMP", 0, 0, "ICMP"},
		{"ARP pass-through", "ARP", 0, 0, "ARP"},
		{"HTTP pass-through", "HTTP", 0, 0, "HTTP"},
		{"TLS pass-through", "TLS", 0, 0, "TLS"},
		{"DNS pass-through", "DNS", 0, 0, "DNS"},
		{"empty pass-through", "", 0, 0, ""},

		// TCP with unknown ports stays TCP
		{"TCP unknown ports", "TCP", 9999, 9998, "TCP"},

		// Dst port takes priority over src port
		{"TCP dst=443 wins", "TCP", 12345, 443, "HTTPS"},
		{"TCP dst=80 wins", "TCP", 12345, 80, "HTTP"},

		// Src port used as fallback when dst is unknown
		{"TCP src=22 fallback", "TCP", 22, 54321, "SSH"},
		{"TCP src=3306 fallback", "TCP", 3306, 54321, "MySQL"},

		// UDP with known ports
		{"UDP dst=53", "UDP", 0, 53, "DNS"},
		{"UDP dst=5353", "UDP", 0, 5353, "mDNS"},
		{"UDP dst=123", "UDP", 0, 123, "NTP"},
		{"UDP dst=67", "UDP", 0, 67, "DHCP"},
		{"UDP unknown ports", "UDP", 9999, 9998, "UDP"},

		// Common TCP service ports
		{"TCP dst=22", "TCP", 54321, 22, "SSH"},
		{"TCP dst=3306", "TCP", 54321, 3306, "MySQL"},
		{"TCP dst=5432", "TCP", 54321, 5432, "Postgres"},
		{"TCP dst=6379", "TCP", 54321, 6379, "Redis"},
		{"TCP dst=27017", "TCP", 54321, 27017, "MongoDB"},
		{"TCP dst=3389", "TCP", 54321, 3389, "RDP"},
		{"TCP dst=5900", "TCP", 54321, 5900, "VNC"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := effectiveProto(c.proto, c.src, c.dst)
			if got != c.wantProto {
				t.Errorf("effectiveProto(%q, %d, %d) = %q, want %q",
					c.proto, c.src, c.dst, got, c.wantProto)
			}
		})
	}
}

// ── ProtocolDistWidget.SelectedFilter ─────────────────────────────────────────

func TestProtoDist_SelectedFilter(t *testing.T) {
	t.Run("PortInferredServices", func(t *testing.T) {
		// Services in l7PrimaryPort should emit FilterPort
		portServices := []struct {
			name string
			port string
		}{
			{"HTTPS", "443"},
			{"SSH", "22"},
			{"FTP", "21"},
			{"MySQL", "3306"},
			{"Postgres", "5432"},
			{"Redis", "6379"},
			{"RDP", "3389"},
		}
		for _, c := range portServices {
			t.Run(c.name, func(t *testing.T) {
				p := NewProtocolDistWidget()
				p.counts[c.name] = 100
				p.rebuildSorted()
				p.cursor = 0

				f := p.SelectedFilter()
				if f == nil {
					t.Fatal("SelectedFilter should not be nil")
				}
				if f.Kind != FilterPort {
					t.Errorf("%s: expected FilterPort, got kind=%d", c.name, f.Kind)
				}
				if f.Value != c.port {
					t.Errorf("%s: expected port %s, got %s", c.name, c.port, f.Value)
				}
			})
		}
	})

	t.Run("ProtocolPassThrough", func(t *testing.T) {
		// Protocols NOT in l7PrimaryPort should emit FilterProtocol
		protos := []string{"TCP", "UDP", "ICMP", "ARP", "TLS", "HTTP", "DNS"}
		for _, proto := range protos {
			t.Run(proto, func(t *testing.T) {
				p := NewProtocolDistWidget()
				p.counts[proto] = 50
				p.rebuildSorted()
				p.cursor = 0

				f := p.SelectedFilter()
				if f == nil {
					t.Fatal("SelectedFilter should not be nil")
				}
				if f.Kind != FilterProtocol {
					t.Errorf("%s: expected FilterProtocol, got kind=%d", proto, f.Kind)
				}
				if f.Value != proto {
					t.Errorf("%s: value mismatch, got %s", proto, f.Value)
				}
			})
		}
	})

	t.Run("Empty", func(t *testing.T) {
		p := NewProtocolDistWidget()
		if f := p.SelectedFilter(); f != nil {
			t.Error("SelectedFilter on empty widget should return nil")
		}
	})

	t.Run("CursorOutOfBounds", func(t *testing.T) {
		p := NewProtocolDistWidget()
		p.counts["TCP"] = 10
		p.rebuildSorted()
		p.cursor = 99 // way out of bounds
		if f := p.SelectedFilter(); f != nil {
			t.Error("out-of-bounds cursor should return nil")
		}
	})
}

// ── l7PrimaryPort consistency with servicePortName ────────────────────────────

func TestL7PrimaryPort_ConsistentWithServicePortName(t *testing.T) {
	for name, portStr := range l7PrimaryPort {
		t.Run(name, func(t *testing.T) {
			// Parse the port string manually (all entries are 1-5 digit integers)
			var portNum uint16
			n := 0
			for _, ch := range portStr {
				n = n*10 + int(ch-'0')
			}
			portNum = uint16(n)

			got := servicePortName(portNum)
			if got != name {
				t.Errorf("l7PrimaryPort[%q]=%q but servicePortName(%d)=%q",
					name, portStr, portNum, got)
			}
		})
	}
}
