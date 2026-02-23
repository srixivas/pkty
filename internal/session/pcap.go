package session

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"

	"github.com/c0d343v3r/netdash/internal/events"
)

// DefaultSaveDir returns the default directory for saved pcap files.
func DefaultSaveDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "netdash-saves"
	}
	return filepath.Join(home, ".local", "share", "netdash", "saves")
}

// SavePcap writes packets to a timestamped pcap file in dir.
// Returns the path written to and the number of packets written.
func SavePcap(dir string, linkType layers.LinkType, packets []events.PacketEvent) (string, int, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	name := fmt.Sprintf("netdash-%s.pcap", time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, name)

	f, err := os.Create(path)
	if err != nil {
		return "", 0, fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := pcapgo.NewWriter(f)
	if err := w.WriteFileHeader(65535, linkType); err != nil {
		return "", 0, fmt.Errorf("write pcap header: %w", err)
	}

	count := 0
	for _, pkt := range packets {
		if len(pkt.RawBytes) == 0 {
			continue
		}
		ci := gopacket.CaptureInfo{
			Timestamp:     pkt.Timestamp,
			CaptureLength: len(pkt.RawBytes),
			Length:        len(pkt.RawBytes),
		}
		if err := w.WritePacket(ci, pkt.RawBytes); err != nil {
			return path, count, fmt.Errorf("write packet %d: %w", count+1, err)
		}
		count++
	}

	return path, count, nil
}
