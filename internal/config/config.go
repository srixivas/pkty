package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all netdash configuration.
type Config struct {
	Capture  CaptureConfig  `toml:"capture"`
	Layout   LayoutConfig   `toml:"layout"`
	Widgets  WidgetsConfig  `toml:"widgets"`
	Theme    ThemeConfig    `toml:"theme"`
}

// CaptureConfig controls the capture backend.
type CaptureConfig struct {
	Interface  string `toml:"interface"`
	PcapFile   string `toml:"pcap_file"`
	BPFFilter  string `toml:"bpf_filter"`
	SnapLen    int    `toml:"snap_len"`
	Promiscuous bool  `toml:"promiscuous"`
}

// LayoutConfig controls the pane layout.
type LayoutConfig struct {
	LeftPanelWidth   int `toml:"left_panel_width"`
	RightPanelWidth  int `toml:"right_panel_width"`
	BottomPanelHeight int `toml:"bottom_panel_height"`
}

// WidgetsConfig controls which widgets are enabled.
type WidgetsConfig struct {
	PacketInspector    bool `toml:"packet_inspector"`
	Connections        bool `toml:"connections"`
	DNSQueries         bool `toml:"dns_queries"`
	Bandwidth          bool `toml:"bandwidth"`
	ProtocolDist       bool `toml:"protocol_distribution"`
	RemoteHosts        bool `toml:"remote_hosts"`
	TLSInspector       bool `toml:"tls_inspector"`
}

// ThemeConfig controls colours and styles.
type ThemeConfig struct {
	Name string `toml:"name"`
}

// Default returns a Config with sane defaults.
func Default() *Config {
	return &Config{
		Capture: CaptureConfig{
			SnapLen:     65535,
			Promiscuous: true,
		},
		Layout: LayoutConfig{
			LeftPanelWidth:    30,
			RightPanelWidth:   35,
			BottomPanelHeight: 12,
		},
		Widgets: WidgetsConfig{
			PacketInspector:  true,
			Connections:      true,
			DNSQueries:       true,
			Bandwidth:        true,
			ProtocolDist:     true,
			RemoteHosts:      true,
			TLSInspector:     true,
		},
		Theme: ThemeConfig{
			Name: "default",
		},
	}
}

// Load reads a TOML config file and merges it over defaults.
// If the file does not exist, defaults are returned.
func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		path = defaultPath()
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}
	return cfg, nil
}

func defaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.toml"
	}
	return filepath.Join(home, ".config", "netdash", "config.toml")
}
