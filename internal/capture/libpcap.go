package capture

import (
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const (
	defaultSnapLen int32         = 65535
	defaultTimeout time.Duration = pcap.BlockForever
	defaultPromisc bool          = true
)

// LibpcapBackend captures packets using libpcap via gopacket.
type LibpcapBackend struct {
	iface   string
	snapLen int32
	promisc bool
	timeout time.Duration
	handle  *pcap.Handle
}

// NewLibpcapBackend creates a new live capture backend for the given interface.
func NewLibpcapBackend(iface string) *LibpcapBackend {
	return &LibpcapBackend{
		iface:   iface,
		snapLen: defaultSnapLen,
		promisc: defaultPromisc,
		timeout: defaultTimeout,
	}
}

func (l *LibpcapBackend) Open() error {
	handle, err := pcap.OpenLive(l.iface, l.snapLen, l.promisc, l.timeout)
	if err != nil {
		return &ErrPrivilege{Reason: err.Error()}
	}
	l.handle = handle
	return nil
}

func (l *LibpcapBackend) ReadPacket() (*RawPacket, error) {
	for {
		data, ci, err := l.handle.ReadPacketData()
		if err != nil {
			// pcap returns NextErrorTimeoutExpired as an error string on
			// macOS when the buffer timeout fires with no packets. Retry.
			if err == pcap.NextErrorTimeoutExpired {
				continue
			}
			return nil, fmt.Errorf("reading packet: %w", err)
		}
		if data == nil {
			continue
		}
		return &RawPacket{
			Timestamp:     ci.Timestamp,
			Data:          data,
			Length:        ci.Length,
			CaptureLength: ci.CaptureLength,
		}, nil
	}
}

func (l *LibpcapBackend) Close() error {
	if l.handle != nil {
		l.handle.Close()
	}
	return nil
}

func (l *LibpcapBackend) ListInterfaces() ([]InterfaceInfo, error) {
	devs, err := pcap.FindAllDevs()
	if err != nil {
		return nil, fmt.Errorf("listing interfaces: %w", err)
	}
	ifaces := make([]InterfaceInfo, 0, len(devs))
	for _, d := range devs {
		info := InterfaceInfo{
			Name:        d.Name,
			Description: d.Description,
		}
		for _, addr := range d.Addresses {
			info.Addresses = append(info.Addresses, addr.IP.String())
		}
		ifaces = append(ifaces, info)
	}
	return ifaces, nil
}

func (l *LibpcapBackend) SetBPFFilter(expr string) error {
	if l.handle == nil {
		return fmt.Errorf("cannot set filter: capture not open")
	}
	return l.handle.SetBPFFilter(expr)
}

func (l *LibpcapBackend) LinkType() layers.LinkType {
	if l.handle != nil {
		return l.handle.LinkType()
	}
	return layers.LinkTypeEthernet
}

// gopacketSource returns a gopacket.PacketSource for this handle.
// Used only if callers need gopacket-level decoding externally.
func (l *LibpcapBackend) gopacketSource() *gopacket.PacketSource {
	return gopacket.NewPacketSource(l.handle, l.handle.LinkType())
}
