package capture

import (
	"fmt"
	"io"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// ErrEOF signals the pcap file has been fully read.
var ErrEOF = io.EOF

// PcapFileBackend reads packets from a .pcap or .pcapng file.
type PcapFileBackend struct {
	path   string
	handle *pcap.Handle
}

// NewPcapFileBackend creates a backend that reads from a pcap file.
func NewPcapFileBackend(path string) *PcapFileBackend {
	return &PcapFileBackend{path: path}
}

func (p *PcapFileBackend) Open() error {
	handle, err := pcap.OpenOffline(p.path)
	if err != nil {
		return fmt.Errorf("opening pcap file %q: %w", p.path, err)
	}
	p.handle = handle
	return nil
}

func (p *PcapFileBackend) ReadPacket() (*RawPacket, error) {
	data, ci, err := p.handle.ReadPacketData()
	if err != nil {
		return nil, fmt.Errorf("reading packet from file: %w", err)
	}
	if data == nil {
		return nil, ErrEOF
	}
	return &RawPacket{
		Timestamp:     ci.Timestamp,
		Data:          data,
		Length:        ci.Length,
		CaptureLength: ci.CaptureLength,
	}, nil
}

func (p *PcapFileBackend) Close() error {
	if p.handle != nil {
		p.handle.Close()
	}
	return nil
}

func (p *PcapFileBackend) ListInterfaces() ([]InterfaceInfo, error) {
	return nil, fmt.Errorf("ListInterfaces not supported for pcap file backend")
}

func (p *PcapFileBackend) SetBPFFilter(expr string) error {
	if p.handle == nil {
		return fmt.Errorf("cannot set filter: capture not open")
	}
	return p.handle.SetBPFFilter(expr)
}

func (p *PcapFileBackend) LinkType() layers.LinkType {
	if p.handle != nil {
		return p.handle.LinkType()
	}
	return layers.LinkTypeEthernet
}

func (p *PcapFileBackend) gopacketSource() *gopacket.PacketSource {
	return gopacket.NewPacketSource(p.handle, p.handle.LinkType())
}
