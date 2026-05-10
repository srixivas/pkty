package events

import (
	"net"
	"time"
)

// PacketEvent represents a captured network packet.
type PacketEvent struct {
	Timestamp   time.Time
	Number      uint64
	SrcIP       net.IP
	DstIP       net.IP
	SrcPort     uint16
	DstPort     uint16
	Protocol    string
	Length      int
	Info        string
	LayerData   []LayerInfo
	RawBytes    []byte
}

// LayerInfo holds decoded information for a single protocol layer.
type LayerInfo struct {
	Name   string
	Fields []FieldInfo
	Start  int
	End    int
}

// FieldInfo is a single key/value pair within a protocol layer.
type FieldInfo struct {
	Key   string
	Value string
}

// DNSEvent represents a captured DNS query or response.
type DNSEvent struct {
	Timestamp  time.Time
	SrcIP      net.IP
	DstIP      net.IP
	QueryName  string
	RecordType string // A, AAAA, CNAME, MX, SRV, etc.
	ResolvedIP net.IP
	TTL        uint32
	IsResponse bool
	IsNXDomain bool
}

// TLSEvent represents a captured TLS handshake.
type TLSEvent struct {
	Timestamp   time.Time
	SrcIP       net.IP
	DstIP       net.IP
	SrcPort     uint16
	DstPort     uint16
	SNI         string
	Version     string // "TLS 1.3", "TLS 1.2", etc.
	CipherSuite string
}

// HTTPEvent represents a captured HTTP request/response pair.
type HTTPEvent struct {
	Timestamp   time.Time
	SrcIP       net.IP
	DstIP       net.IP
	Method      string
	URL         string
	StatusCode  int
	ContentType string
	Duration    time.Duration
}

// ConnectionEvent represents an active network connection.
type ConnectionEvent struct {
	Timestamp    time.Time
	SrcIP        net.IP
	DstIP        net.IP
	SrcPort      uint16
	DstPort      uint16
	Protocol     string // "TCP" or "UDP"
	State        string // ESTABLISHED, TIME_WAIT, CLOSE_WAIT, etc.
	BytesSent    uint64
	BytesRecv    uint64
	ProcessName  string
}

// BandwidthEvent represents a periodic bandwidth measurement.
type BandwidthEvent struct {
	Timestamp  time.Time
	Interface  string
	TXBytesPS  uint64 // transmit bytes per second
	RXBytesPS  uint64 // receive bytes per second
	TXTotal    uint64
	RXTotal    uint64
}

// ProtocolCount tracks packet counts per protocol.
type ProtocolCount struct {
	Protocol string
	Count    uint64
}

// EventBus distributes typed events to subscriber channels.
type EventBus struct {
	Packets     chan PacketEvent
	DNS         chan DNSEvent
	TLS         chan TLSEvent
	HTTP        chan HTTPEvent
	Connections chan ConnectionEvent
	Bandwidth   chan BandwidthEvent
}

// NewEventBus creates an EventBus with buffered channels.
func NewEventBus(bufSize int) *EventBus {
	return &EventBus{
		Packets:     make(chan PacketEvent, bufSize),
		DNS:         make(chan DNSEvent, bufSize),
		TLS:         make(chan TLSEvent, bufSize),
		HTTP:        make(chan HTTPEvent, bufSize),
		Connections: make(chan ConnectionEvent, bufSize),
		Bandwidth:   make(chan BandwidthEvent, bufSize),
	}
}

// Close closes all channels on the event bus.
func (eb *EventBus) Close() {
	close(eb.Packets)
	close(eb.DNS)
	close(eb.TLS)
	close(eb.HTTP)
	close(eb.Connections)
	close(eb.Bandwidth)
}
