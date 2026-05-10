package widgets

import (
	"net"
	"testing"

	"github.com/srixivas/pkty/internal/events"
)

// ── isPrivateIP ───────────────────────────────────────────────────────────────

func TestIsPrivateIP(t *testing.T) {
	t.Run("Private", func(t *testing.T) {
		cases := []struct {
			name string
			ip   string
		}{
			{"10.x", "10.0.0.1"},
			{"10.x max", "10.255.255.255"},
			{"172.16.x", "172.16.0.1"},
			{"172.31.x", "172.31.255.255"},
			{"192.168.x", "192.168.0.1"},
			{"192.168.x max", "192.168.255.255"},
			{"loopback", "127.0.0.1"},
			{"loopback range", "127.0.0.255"},
			{"link-local", "169.254.1.1"},
			{"IPv6 loopback", "::1"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ip := net.ParseIP(c.ip)
				if ip == nil {
					t.Fatalf("invalid IP: %s", c.ip)
				}
				if !isPrivateIP(ip) {
					t.Errorf("isPrivateIP(%s) = false, want true", c.ip)
				}
			})
		}
	})

	t.Run("Public", func(t *testing.T) {
		cases := []struct {
			name string
			ip   string
		}{
			{"Google DNS", "8.8.8.8"},
			{"Cloudflare", "1.1.1.1"},
			{"example.com", "93.184.216.34"},
			{"Google", "142.250.80.46"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ip := net.ParseIP(c.ip)
				if ip == nil {
					t.Fatalf("invalid IP: %s", c.ip)
				}
				if isPrivateIP(ip) {
					t.Errorf("isPrivateIP(%s) = true, want false", c.ip)
				}
			})
		}
	})

	t.Run("Nil", func(t *testing.T) {
		if isPrivateIP(nil) {
			t.Error("isPrivateIP(nil) should return false")
		}
	})
}

// ── protoLabel ────────────────────────────────────────────────────────────────

func TestProtoLabel(t *testing.T) {
	t.Run("KnownPorts", func(t *testing.T) {
		cases := []struct {
			proto string
			port  uint16
			want  string
		}{
			{"TCP", 80, "HTTP"},
			{"TCP", 8080, "HTTP"},
			{"TCP", 443, "HTTPS"},
			{"TCP", 8443, "HTTPS"},
			{"TCP", 22, "SSH"},
			{"UDP", 53, "DNS"},
			{"TCP", 21, "FTP"},
			{"TCP", 25, "SMTP"},
			{"TCP", 587, "SMTP"},
			{"TCP", 3306, "MySQL"},
			{"TCP", 5432, "Postgres"},
			{"TCP", 6379, "Redis"},
			{"TCP", 27017, "MongoDB"},
			{"UDP", 5353, "mDNS"},
			{"UDP", 67, "DHCP"},
			{"UDP", 123, "NTP"},
			{"TCP", 3389, "RDP"},
			{"TCP", 5900, "VNC"},
		}
		for _, c := range cases {
			got := protoLabel(c.proto, c.port)
			if got != c.want {
				t.Errorf("protoLabel(%q, %d) = %q, want %q", c.proto, c.port, got, c.want)
			}
		}
	})

	t.Run("UnknownPort", func(t *testing.T) {
		got := protoLabel("TCP", 9999)
		if got != "TCP:9999" {
			t.Errorf("protoLabel(TCP, 9999) = %q, want TCP:9999", got)
		}
	})

	t.Run("ZeroPort", func(t *testing.T) {
		got := protoLabel("", 0)
		if got != "???" {
			t.Errorf("protoLabel('', 0) = %q, want ???", got)
		}
	})
}

// ── ngBarFill ─────────────────────────────────────────────────────────────────

func TestNgBarFill(t *testing.T) {
	cases := []struct {
		name       string
		val, max   uint64
		width      int
		want       int
	}{
		{"zero value", 0, 100, 10, 0},
		{"full value", 100, 100, 10, 10},
		{"half value", 50, 100, 10, 5},
		{"zero max", 0, 0, 10, 0},
		{"zero width", 5, 10, 0, 0},
		{"value exceeds max clamped", 200, 100, 10, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ngBarFill(c.val, c.max, c.width)
			if got != c.want {
				t.Errorf("ngBarFill(%d, %d, %d) = %d, want %d",
					c.val, c.max, c.width, got, c.want)
			}
		})
	}
}

// ── RebuildVisible ────────────────────────────────────────────────────────────

// injectHost adds an ngHost directly into the widget for test setup.
func injectHost(g *NetGraphWidget, ip string, port uint16, proto string, tx, rx uint64) {
	ek := ngEdgeKey{Proto: proto, Port: port}
	h, ok := g.hosts[ip]
	if !ok {
		h = &ngHost{IP: ip, Edges: make(map[ngEdgeKey]*ngEdge), Tag: "remote"}
		g.hosts[ip] = h
	}
	h.Edges[ek] = &ngEdge{Proto: proto, Port: port, TXBytes: tx, RXBytes: rx}
	h.TXTotal += tx
	h.RXTotal += rx
}

func TestRebuildVisible(t *testing.T) {
	t.Run("NoFilter_AllVisible", func(t *testing.T) {
		g := NewNetGraphWidget()
		injectHost(g, "1.1.1.1", 443, "TCP", 1000, 0)
		injectHost(g, "8.8.8.8", 53, "UDP", 500, 0)
		g.rebuildSorted()

		if len(g.visible) != 2 {
			t.Errorf("no filter: visible should contain 2 hosts, got %d", len(g.visible))
		}
		if len(g.visible) != len(g.sorted) {
			t.Error("no filter: visible and sorted should have same length")
		}
	})

	t.Run("FilterIP_ExactMatch", func(t *testing.T) {
		g := NewNetGraphWidget()
		ds := NewDisplayFilterSet()
		g.SetDisplayFilter(ds)

		injectHost(g, "1.1.1.1", 443, "TCP", 1000, 0)
		injectHost(g, "8.8.8.8", 53, "UDP", 500, 0)
		injectHost(g, "9.9.9.9", 80, "TCP", 200, 0)

		ds.Toggle(MakeFilter(FilterIP, "1.1.1.1"))
		g.rebuildSorted()

		if len(g.visible) != 1 {
			t.Errorf("expected 1 visible host, got %d", len(g.visible))
		}
		if g.visible[0].IP != "1.1.1.1" {
			t.Errorf("wrong visible host: got %s, want 1.1.1.1", g.visible[0].IP)
		}
	})

	t.Run("FilterIP_Glob", func(t *testing.T) {
		g := NewNetGraphWidget()
		ds := NewDisplayFilterSet()
		g.SetDisplayFilter(ds)

		injectHost(g, "1.1.1.1", 443, "TCP", 1000, 0)
		injectHost(g, "1.1.1.2", 443, "TCP", 800, 0)
		injectHost(g, "8.8.8.8", 53, "UDP", 500, 0)

		ds.Toggle(MakeFilter(FilterIP, "1.1.1.*"))
		g.rebuildSorted()

		if len(g.visible) != 2 {
			t.Errorf("glob filter: expected 2 visible hosts, got %d", len(g.visible))
		}
	})

	t.Run("FilterPort", func(t *testing.T) {
		g := NewNetGraphWidget()
		ds := NewDisplayFilterSet()
		g.SetDisplayFilter(ds)

		injectHost(g, "1.1.1.1", 443, "TCP", 1000, 0)
		injectHost(g, "8.8.8.8", 53, "UDP", 500, 0)
		injectHost(g, "9.9.9.9", 80, "TCP", 200, 0)

		ds.Toggle(MakeFilter(FilterPort, "443"))
		g.rebuildSorted()

		if len(g.visible) != 1 {
			t.Fatalf("expected 1 visible host, got %d", len(g.visible))
		}
		if g.visible[0].IP != "1.1.1.1" {
			t.Errorf("wrong visible host: got %s, want 1.1.1.1", g.visible[0].IP)
		}
	})

	t.Run("FilterDNS", func(t *testing.T) {
		g := NewNetGraphWidget()
		ds := NewDisplayFilterSet()
		g.SetDisplayFilter(ds)

		ds.AddDNSMapping("example.com", net.ParseIP("93.184.216.34"))
		injectHost(g, "93.184.216.34", 443, "TCP", 5000, 0)
		injectHost(g, "8.8.8.8", 53, "UDP", 500, 0)

		ds.Toggle(MakeFilter(FilterDNS, "example.com"))
		g.rebuildSorted()

		if len(g.visible) != 1 {
			t.Fatalf("expected 1 visible host, got %d", len(g.visible))
		}
		if g.visible[0].IP != "93.184.216.34" {
			t.Errorf("wrong visible host: got %s, want 93.184.216.34", g.visible[0].IP)
		}
	})

	t.Run("AllFiltered", func(t *testing.T) {
		g := NewNetGraphWidget()
		ds := NewDisplayFilterSet()
		g.SetDisplayFilter(ds)

		injectHost(g, "1.1.1.1", 443, "TCP", 1000, 0)
		injectHost(g, "8.8.8.8", 53, "UDP", 500, 0)

		ds.Toggle(MakeFilter(FilterIP, "5.5.5.5"))
		g.rebuildSorted()

		if len(g.visible) != 0 {
			t.Errorf("all-filtered: expected 0 visible hosts, got %d", len(g.visible))
		}
	})

	t.Run("ClearRestoresAll", func(t *testing.T) {
		g := NewNetGraphWidget()
		ds := NewDisplayFilterSet()
		g.SetDisplayFilter(ds)

		injectHost(g, "1.1.1.1", 443, "TCP", 1000, 0)
		injectHost(g, "8.8.8.8", 53, "UDP", 500, 0)

		ds.Toggle(MakeFilter(FilterIP, "1.1.1.1"))
		g.rebuildSorted()
		if len(g.visible) != 1 {
			t.Fatalf("pre-condition: expected 1 visible host, got %d", len(g.visible))
		}

		ds.Clear()
		g.RebuildVisible()

		if len(g.visible) != 2 {
			t.Errorf("after clear: expected 2 visible hosts, got %d", len(g.visible))
		}
	})

	t.Run("CursorClampedOnFilter", func(t *testing.T) {
		g := NewNetGraphWidget()
		ds := NewDisplayFilterSet()
		g.SetDisplayFilter(ds)

		injectHost(g, "1.1.1.1", 443, "TCP", 3000, 0)
		injectHost(g, "8.8.8.8", 53, "UDP", 2000, 0)
		injectHost(g, "9.9.9.9", 80, "TCP", 1000, 0)
		g.rebuildSorted()
		g.cursor = 2 // at last host

		ds.Toggle(MakeFilter(FilterIP, "1.1.1.1"))
		g.RebuildVisible()

		if g.cursor != 0 {
			t.Errorf("cursor should clamp to 0 when visible shrinks, got %d", g.cursor)
		}
	})
}

// ── NetGraphWidget.Update with PacketEvent ────────────────────────────────────

func makePacketEvent(srcIP, dstIP string, dstPort uint16, length int) events.PacketEvent {
	return events.PacketEvent{
		SrcIP:    net.ParseIP(srcIP),
		DstIP:    net.ParseIP(dstIP),
		Protocol: "TCP",
		DstPort:  dstPort,
		Length:   length,
	}
}

func TestNetGraph_Update(t *testing.T) {
	t.Run("AddsRemoteHost", func(t *testing.T) {
		g := NewNetGraphWidget()
		g.Update(makePacketEvent("192.168.1.1", "8.8.8.8", 443, 100))
		if len(g.hosts) != 1 {
			t.Errorf("expected 1 host, got %d", len(g.hosts))
		}
		if _, ok := g.hosts["8.8.8.8"]; !ok {
			t.Error("remote host 8.8.8.8 not found in hosts map")
		}
	})

	t.Run("TrafficDirection_TX_RX", func(t *testing.T) {
		cases := []struct {
			name       string
			src, dst   string
			wantTX     uint64
			wantRX     uint64
		}{
			{"outgoing (local→remote) is TX", "192.168.1.1", "8.8.8.8", 200, 0},
			{"incoming (remote→local) is RX", "8.8.8.8", "192.168.1.1", 0, 150},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				g := NewNetGraphWidget()
				size := int(c.wantTX)
				if c.wantRX > 0 {
					size = int(c.wantRX)
				}
				g.Update(makePacketEvent(c.src, c.dst, 443, size))
				h := g.hosts["8.8.8.8"]
				if h == nil {
					t.Fatal("host 8.8.8.8 not found")
				}
				if h.TXTotal != c.wantTX {
					t.Errorf("TXTotal = %d, want %d", h.TXTotal, c.wantTX)
				}
				if h.RXTotal != c.wantRX {
					t.Errorf("RXTotal = %d, want %d", h.RXTotal, c.wantRX)
				}
			})
		}
	})
}

// ── NetGraphWidget.SelectedFilter ────────────────────────────────────────────

func TestNetGraph_SelectedFilter(t *testing.T) {
	t.Run("WithDNSName_EmitsDNSFilter", func(t *testing.T) {
		g := NewNetGraphWidget()
		g.AddIPName("8.8.8.8", "dns.google.com")
		injectHost(g, "8.8.8.8", 53, "UDP", 100, 0)
		g.rebuildSorted()
		g.cursor = 0

		f := g.SelectedFilter()
		if f == nil {
			t.Fatal("SelectedFilter should not be nil")
		}
		if f.Kind != FilterDNS {
			t.Errorf("host with DNS name: expected FilterDNS, got kind=%d", f.Kind)
		}
		if f.Value != "dns.google.com" {
			t.Errorf("filter value = %q, want dns.google.com", f.Value)
		}
	})

	t.Run("WithoutDNSName_EmitsIPFilter", func(t *testing.T) {
		g := NewNetGraphWidget()
		injectHost(g, "1.2.3.4", 443, "TCP", 100, 0)
		g.rebuildSorted()
		g.cursor = 0

		f := g.SelectedFilter()
		if f == nil {
			t.Fatal("SelectedFilter should not be nil")
		}
		if f.Kind != FilterIP {
			t.Errorf("host without DNS name: expected FilterIP, got kind=%d", f.Kind)
		}
		if f.Value != "1.2.3.4" {
			t.Errorf("filter value = %q, want 1.2.3.4", f.Value)
		}
	})

	t.Run("EmptyGraph_ReturnsNil", func(t *testing.T) {
		g := NewNetGraphWidget()
		if f := g.SelectedFilter(); f != nil {
			t.Error("SelectedFilter on empty graph should return nil")
		}
	})

	t.Run("OutOfBoundsCursor_ReturnsNil", func(t *testing.T) {
		g := NewNetGraphWidget()
		injectHost(g, "1.2.3.4", 443, "TCP", 100, 0)
		g.rebuildSorted()
		g.cursor = 99

		if f := g.SelectedFilter(); f != nil {
			t.Error("out-of-bounds cursor should return nil")
		}
	})
}
