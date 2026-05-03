package capture

import (
	"fmt"
	"time"

	"github.com/google/gopacket/layers"
)

// RawPacket is a captured packet before protocol parsing.
type RawPacket struct {
	Timestamp time.Time
	Data      []byte
	Length    int
	CaptureLength int
}

// InterfaceInfo describes a network interface available for capture.
type InterfaceInfo struct {
	Name        string
	Description string
	Addresses   []string
}

// Backend is the capture interface. All capture sources (libpcap, eBPF,
// pcap files) implement this interface so the rest of the application
// never depends on a specific capture library.
type Backend interface {
	// Open initialises the capture source. For live capture this opens
	// the network interface; for offline mode it opens a pcap file.
	Open() error

	// ReadPacket blocks until the next packet is available and returns it.
	// Returns an error when the source is exhausted or closed.
	ReadPacket() (*RawPacket, error)

	// Close releases all resources held by the capture source.
	Close() error

	// ListInterfaces returns available network interfaces.
	// Only meaningful for live capture backends.
	ListInterfaces() ([]InterfaceInfo, error)

	// SetBPFFilter applies a BPF filter expression to the capture.
	SetBPFFilter(expr string) error

	// LinkType returns the link layer type for packet decoding.
	LinkType() layers.LinkType
}

// ErrPrivilege is returned when the process lacks permissions for capture.
type ErrPrivilege struct {
	Reason string
}

func (e *ErrPrivilege) Error() string {
	return fmt.Sprintf("insufficient privileges for packet capture: %s\n\n"+
		"To fix this:\n"+
		"  Linux:  sudo setcap cap_net_raw+ep ./pkty   OR   sudo ./pkty\n"+
		"  macOS:  sudo ./pkty   (requires /dev/bpf access)\n"+
		"  Windows: install Npcap (https://npcap.com) and run as Administrator",
		e.Reason)
}
