package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/srixivas/pkty/internal/capture"
	"github.com/srixivas/pkty/internal/config"
	"github.com/srixivas/pkty/internal/events"
	"github.com/srixivas/pkty/internal/layout"
	"github.com/srixivas/pkty/internal/parser"
	"github.com/srixivas/pkty/internal/store"
)

var (
	version       = "dev"
	configPath    = flag.String("config", "", "path to config file (default: ~/.config/pkty/config.toml)")
	iface         = flag.String("i", "", "network interface to capture on")
	pcapFile      = flag.String("r", "", "read from pcap file instead of live capture")
	bpfFilter     = flag.String("f", "", "BPF filter expression")
	showVer       = flag.Bool("version", false, "print version and exit")
	sqliteEnable  = flag.Bool("sqlite", false, "enable SQLite packet logging to default path (~/.local/share/pkty/pkty.db)")
	sqliteDB      = flag.String("sqlite-db", "", "SQLite database path (enables SQLite logging, overrides default path)")
	uiRefresh     = flag.Int("ui-refresh", 0, "UI redraw interval in ms (default: 100); increase on high-traffic servers")
	showEncrypted = flag.Bool("show-encrypted", false, "show TLS/SSH/QUIC packets in the inspector list (default: stats only)")
)

func main() {
	flag.Parse()

	if *showVer {
		fmt.Printf("\n")
		fmt.Printf("     /\\_/\\     \n")
		fmt.Printf("    ((@v@))    \n")
		fmt.Printf("    ():::()    \n")
		fmt.Printf("     (   )     \n")
		fmt.Printf("    /|   |\\    \n")
		fmt.Printf("     v   v     \n")
		fmt.Printf("\n")
		fmt.Printf("  pkty %s\n", version)
		fmt.Printf("  terminal network inspector\n")
		fmt.Printf("\n")
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// CLI flags override config
	if *iface != "" {
		cfg.Capture.Interface = *iface
	}
	if *pcapFile != "" {
		cfg.Capture.PcapFile = *pcapFile
	}
	if *bpfFilter != "" {
		cfg.Capture.BPFFilter = *bpfFilter
	}
	if *uiRefresh > 0 {
		cfg.Performance.UIRefreshMs = *uiRefresh
	}
	if *showEncrypted {
		cfg.Performance.HideEncrypted = false
	}

	bus := events.NewEventBus(4096)

	// Select capture backend
	var backend capture.Backend
	if cfg.Capture.PcapFile != "" {
		backend = capture.NewPcapFileBackend(cfg.Capture.PcapFile)
	} else if cfg.Capture.Interface != "" {
		backend = capture.NewLibpcapBackend(cfg.Capture.Interface)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if backend != nil {
		if err := backend.Open(); err != nil {
			log.Fatalf("capture: %v", err)
		}

		if cfg.Capture.BPFFilter != "" {
			if err := backend.SetBPFFilter(cfg.Capture.BPFFilter); err != nil {
				backend.Close()
				log.Fatalf("bpf filter: %v", err)
			}
		}

		rawPackets := make(chan capture.RawPacket, 4096)

		// Capture goroutine: reads raw packets from the backend.
		// ReadPacket now handles timeouts internally (retries on
		// pcap.NextErrorTimeoutExpired) so this only exits on real
		// errors or context cancellation.
		go func() {
			defer backend.Close()
			defer close(rawPackets)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				pkt, err := backend.ReadPacket()
				if err != nil {
					log.Printf("capture: %v", err)
					return
				}
				select {
				case rawPackets <- *pkt:
				case <-ctx.Done():
					return
				}
			}
		}()

		// Parser goroutine: decodes raw packets → typed events on the bus.
		psr := parser.New(bus, backend.LinkType())
		go func() {
			var count uint64
			for raw := range rawPackets {
				count++
				psr.ProcessPacket(raw, count)
			}
		}()
	}

	// Optionally open a SQLite packet log.
	var sqliteStore *store.SQLiteStore
	sqliteDBPath := *sqliteDB
	if sqliteDBPath == "" && *sqliteEnable {
		sqliteDBPath = store.DefaultPath()
	}
	if sqliteDBPath != "" {
		s, err := store.NewSQLiteStore(sqliteDBPath)
		if err != nil {
			log.Fatalf("sqlite: %v", err)
		}
		sqliteStore = s
		log.Printf("SQLite logging to %s", sqliteDBPath)
	}

	model := layout.New(cfg, bus)
	if backend != nil {
		model.OnFilterApply = func(expr string) error {
			return backend.SetBPFFilter(expr)
		}
		model.SetLinkType(backend.LinkType())
		model.SetCapturing(true)
	}
	if sqliteStore != nil {
		model.SetSQLiteStore(sqliteStore)
	}

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		log.Fatalf("error running pkty: %v", err)
	}
	cancel()

	if sqliteStore != nil {
		if err := sqliteStore.Close(); err != nil {
			log.Printf("sqlite close: %v", err)
		}
	}
}
