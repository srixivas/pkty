package parser

import (
	"fmt"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/srixivas/pkty/internal/capture"
	"github.com/srixivas/pkty/internal/events"
)

// Parser decodes raw packets and emits typed events to the EventBus.
type Parser struct {
	bus     *events.EventBus
	linkType layers.LinkType
}

// New creates a Parser wired to the given EventBus.
func New(bus *events.EventBus, linkType layers.LinkType) *Parser {
	return &Parser{bus: bus, linkType: linkType}
}

// ProcessPacket decodes a raw packet and publishes typed events.
func (p *Parser) ProcessPacket(raw capture.RawPacket, number uint64) {
	packet := gopacket.NewPacket(raw.Data, p.linkType, gopacket.Default)

	evt := events.PacketEvent{
		Timestamp: raw.Timestamp,
		Number:    number,
		Length:    raw.Length,
		RawBytes:  raw.Data,
	}

	// --- L2: Ethernet ---
	if ethLayer := packet.Layer(layers.LayerTypeEthernet); ethLayer != nil {
		eth := ethLayer.(*layers.Ethernet)
		evt.LayerData = append(evt.LayerData, events.LayerInfo{
			Name:  "Ethernet",
			Start: 0,
			End:   14,
			Fields: []events.FieldInfo{
				{Key: "Src MAC", Value: eth.SrcMAC.String()},
				{Key: "Dst MAC", Value: eth.DstMAC.String()},
				{Key: "Type", Value: eth.EthernetType.String()},
			},
		})
	}

	// --- L3: IPv4 ---
	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		ip := ipLayer.(*layers.IPv4)
		evt.SrcIP = ip.SrcIP
		evt.DstIP = ip.DstIP
		evt.LayerData = append(evt.LayerData, events.LayerInfo{
			Name:  "IPv4",
			Start: len(ip.BaseLayer.Contents),
			End:   len(ip.BaseLayer.Contents) + len(ip.BaseLayer.Payload),
			Fields: []events.FieldInfo{
				{Key: "Src", Value: ip.SrcIP.String()},
				{Key: "Dst", Value: ip.DstIP.String()},
				{Key: "TTL", Value: fmt.Sprintf("%d", ip.TTL)},
				{Key: "Protocol", Value: ip.Protocol.String()},
				{Key: "Length", Value: fmt.Sprintf("%d", ip.Length)},
			},
		})
	}

	// --- L3: IPv6 ---
	if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
		ip := ipLayer.(*layers.IPv6)
		evt.SrcIP = ip.SrcIP
		evt.DstIP = ip.DstIP
		evt.LayerData = append(evt.LayerData, events.LayerInfo{
			Name: "IPv6",
			Fields: []events.FieldInfo{
				{Key: "Src", Value: ip.SrcIP.String()},
				{Key: "Dst", Value: ip.DstIP.String()},
				{Key: "Next Header", Value: ip.NextHeader.String()},
				{Key: "Hop Limit", Value: fmt.Sprintf("%d", ip.HopLimit)},
			},
		})
	}

	// --- L3: ARP ---
	if arpLayer := packet.Layer(layers.LayerTypeARP); arpLayer != nil {
		arp := arpLayer.(*layers.ARP)
		evt.Protocol = "ARP"
		srcIP := net.IP(arp.SourceProtAddress)
		dstIP := net.IP(arp.DstProtAddress)
		evt.SrcIP = srcIP
		evt.DstIP = dstIP
		op := "Request"
		if arp.Operation == 2 {
			op = "Reply"
		}
		evt.Info = fmt.Sprintf("ARP %s: Who has %s? Tell %s", op, dstIP, srcIP)
		evt.LayerData = append(evt.LayerData, events.LayerInfo{
			Name: "ARP",
			Fields: []events.FieldInfo{
				{Key: "Operation", Value: op},
				{Key: "Sender MAC", Value: net.HardwareAddr(arp.SourceHwAddress).String()},
				{Key: "Sender IP", Value: srcIP.String()},
				{Key: "Target MAC", Value: net.HardwareAddr(arp.DstHwAddress).String()},
				{Key: "Target IP", Value: dstIP.String()},
			},
		})
	}

	// --- L4: TCP ---
	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp := tcpLayer.(*layers.TCP)
		evt.SrcPort = uint16(tcp.SrcPort)
		evt.DstPort = uint16(tcp.DstPort)
		if evt.Protocol == "" {
			evt.Protocol = "TCP"
		}
		flags := tcpFlags(tcp)
		evt.Info = fmt.Sprintf("%d → %d [%s] Seq=%d Ack=%d Win=%d Len=%d",
			tcp.SrcPort, tcp.DstPort, flags, tcp.Seq, tcp.Ack, tcp.Window, len(tcp.Payload))
		evt.LayerData = append(evt.LayerData, events.LayerInfo{
			Name: "TCP",
			Fields: []events.FieldInfo{
				{Key: "Src Port", Value: fmt.Sprintf("%d", tcp.SrcPort)},
				{Key: "Dst Port", Value: fmt.Sprintf("%d", tcp.DstPort)},
				{Key: "Flags", Value: flags},
				{Key: "Seq", Value: fmt.Sprintf("%d", tcp.Seq)},
				{Key: "Ack", Value: fmt.Sprintf("%d", tcp.Ack)},
				{Key: "Window", Value: fmt.Sprintf("%d", tcp.Window)},
			},
		})

		// Check for application-layer protocols on TCP payload
		if len(tcp.Payload) > 0 {
			p.parseTCPPayload(tcp.Payload, &evt)
		}
	}

	// --- L4: UDP ---
	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp := udpLayer.(*layers.UDP)
		evt.SrcPort = uint16(udp.SrcPort)
		evt.DstPort = uint16(udp.DstPort)
		if evt.Protocol == "" {
			evt.Protocol = "UDP"
		}
		evt.Info = fmt.Sprintf("%d → %d Len=%d", udp.SrcPort, udp.DstPort, udp.Length)
		evt.LayerData = append(evt.LayerData, events.LayerInfo{
			Name: "UDP",
			Fields: []events.FieldInfo{
				{Key: "Src Port", Value: fmt.Sprintf("%d", udp.SrcPort)},
				{Key: "Dst Port", Value: fmt.Sprintf("%d", udp.DstPort)},
				{Key: "Length", Value: fmt.Sprintf("%d", udp.Length)},
			},
		})
	}

	// --- L4: ICMP ---
	if icmpLayer := packet.Layer(layers.LayerTypeICMPv4); icmpLayer != nil {
		icmp := icmpLayer.(*layers.ICMPv4)
		evt.Protocol = "ICMP"
		evt.Info = fmt.Sprintf("Type=%d Code=%d", icmp.TypeCode.Type(), icmp.TypeCode.Code())
		evt.LayerData = append(evt.LayerData, events.LayerInfo{
			Name: "ICMPv4",
			Fields: []events.FieldInfo{
				{Key: "Type", Value: fmt.Sprintf("%d", icmp.TypeCode.Type())},
				{Key: "Code", Value: fmt.Sprintf("%d", icmp.TypeCode.Code())},
				{Key: "Checksum", Value: fmt.Sprintf("0x%04x", icmp.Checksum)},
			},
		})
	}

	// --- DNS (gopacket has a DNS layer) ---
	if dnsLayer := packet.Layer(layers.LayerTypeDNS); dnsLayer != nil {
		p.parseDNS(dnsLayer.(*layers.DNS), &evt)
	}

	// Default protocol from packet layers if still empty
	if evt.Protocol == "" {
		evt.Protocol = "Unknown"
	}

	// Send non-blocking to avoid stalling capture
	select {
	case p.bus.Packets <- evt:
	default:
	}
}

// parseTCPPayload inspects TCP payload for TLS and HTTP.
func (p *Parser) parseTCPPayload(payload []byte, evt *events.PacketEvent) {
	// TLS detection: ContentType 0x16 (Handshake) with version bytes
	if len(payload) > 5 && payload[0] == 0x16 {
		p.parseTLS(payload, evt)
		return
	}

	// HTTP detection
	if isHTTP(payload) {
		p.parseHTTP(payload, evt)
	}
}

func tcpFlags(tcp *layers.TCP) string {
	var flags []byte
	if tcp.SYN {
		flags = append(flags, 'S')
	}
	if tcp.ACK {
		flags = append(flags, 'A')
	}
	if tcp.FIN {
		flags = append(flags, 'F')
	}
	if tcp.RST {
		flags = append(flags, 'R')
	}
	if tcp.PSH {
		flags = append(flags, 'P')
	}
	if tcp.URG {
		flags = append(flags, 'U')
	}
	if len(flags) == 0 {
		return "none"
	}
	return string(flags)
}
