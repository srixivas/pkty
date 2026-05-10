//go:build ignore

// Generates a synthetic demo.pcap for use with the VHS tape.
// Run: go run demo/gen_pcap.go
// Output: demo/demo.pcap

package main

import (
	"encoding/binary"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

var (
	localMAC  = net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	routerMAC = net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	localIP   = net.IP{192, 168, 1, 42}

	hosts = []struct {
		name string
		ip   net.IP
	}{
		{"github.com", net.IP{140, 82, 121, 4}},
		{"api.github.com", net.IP{140, 82, 121, 6}},
		{"google.com", net.IP{142, 250, 80, 46}},
		{"googleapis.com", net.IP{172, 217, 14, 74}},
		{"cloudflare.com", net.IP{104, 16, 132, 229}},
		{"1.1.1.1", net.IP{1, 1, 1, 1}},
		{"fastly.net", net.IP{151, 101, 1, 57}},
		{"slack.com", net.IP{54, 192, 4, 141}},
	}

	dnsNames = []string{
		"github.com", "api.github.com", "google.com",
		"googleapis.com", "cloudflare.com", "slack.com", "fastly.net",
	}

	ciphers = []uint16{0xc02c, 0xc02b, 0x1301, 0x1302, 0x1303}
)

func main() {
	f, err := os.Create("demo/demo.pcap")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	w := pcapgo.NewWriter(f)
	w.WriteFileHeader(65535, layers.LinkTypeEthernet)

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	ts := time.Now().Add(-60 * time.Second)
	rng := rand.New(rand.NewSource(42))

	writePacket := func(ts time.Time, layers ...gopacket.SerializableLayer) {
		buf.Clear()
		gopacket.SerializeLayers(buf, opts, layers...)
		w.WritePacket(gopacket.CaptureInfo{
			Timestamp:     ts,
			CaptureLength: len(buf.Bytes()),
			Length:        len(buf.Bytes()),
		}, buf.Bytes())
	}

	// DNS query + response pairs
	for i, name := range dnsNames {
		host := hosts[i]
		qTs := ts.Add(time.Duration(i*800) * time.Millisecond)
		txID := uint16(rng.Intn(65535))

		// Query (client → 1.1.1.1)
		dnsQ := &layers.DNS{
			ID:      txID,
			QR:      false,
			OpCode:  layers.DNSOpCodeQuery,
			RD:      true,
			QDCount: 1,
			Questions: []layers.DNSQuestion{{
				Name:  []byte(name),
				Type:  layers.DNSTypeA,
				Class: layers.DNSClassIN,
			}},
		}
		writePacket(qTs,
			&layers.Ethernet{SrcMAC: localMAC, DstMAC: routerMAC, EthernetType: layers.EthernetTypeIPv4},
			&layers.IPv4{SrcIP: localIP, DstIP: net.IP{1, 1, 1, 1}, Protocol: layers.IPProtocolUDP, TTL: 64},
			&layers.UDP{SrcPort: layers.UDPPort(40000+i), DstPort: 53},
			dnsQ,
		)

		// Response (1.1.1.1 → client)
		dnsR := &layers.DNS{
			ID:      txID,
			QR:      true,
			OpCode:  layers.DNSOpCodeQuery,
			AA:      false,
			RD:      true,
			RA:      true,
			QDCount: 1,
			ANCount: 1,
			Questions: []layers.DNSQuestion{{
				Name:  []byte(name),
				Type:  layers.DNSTypeA,
				Class: layers.DNSClassIN,
			}},
			Answers: []layers.DNSResourceRecord{{
				Name:  []byte(name),
				Type:  layers.DNSTypeA,
				Class: layers.DNSClassIN,
				TTL:   300,
				IP:    host.ip,
			}},
		}
		writePacket(qTs.Add(20*time.Millisecond),
			&layers.Ethernet{SrcMAC: routerMAC, DstMAC: localMAC, EthernetType: layers.EthernetTypeIPv4},
			&layers.IPv4{SrcIP: net.IP{1, 1, 1, 1}, DstIP: localIP, Protocol: layers.IPProtocolUDP, TTL: 57},
			&layers.UDP{SrcPort: 53, DstPort: layers.UDPPort(40000 + i)},
			dnsR,
		)
		ts = qTs.Add(20 * time.Millisecond)
	}

	// TLS ClientHello packets (simulated as TCP payload)
	for i, host := range hosts {
		flowTS := ts.Add(time.Duration(i*600) * time.Millisecond)
		srcPort := uint16(50000 + i*7)

		// TCP SYN
		writePacket(flowTS,
			&layers.Ethernet{SrcMAC: localMAC, DstMAC: routerMAC, EthernetType: layers.EthernetTypeIPv4},
			&layers.IPv4{SrcIP: localIP, DstIP: host.ip, Protocol: layers.IPProtocolTCP, TTL: 64},
			&layers.TCP{SrcPort: layers.TCPPort(srcPort), DstPort: 443, SYN: true, Seq: rng.Uint32()},
			gopacket.Payload{},
		)

		// TLS ClientHello — hand-crafted bytes
		sni := []byte(host.name)
		if len(host.name) == 0 || host.name[0] >= '0' && host.name[0] <= '9' {
			// numeric IP — skip SNI
			continue
		}
		hello := buildClientHello(sni, ciphers)
		writePacket(flowTS.Add(5*time.Millisecond),
			&layers.Ethernet{SrcMAC: localMAC, DstMAC: routerMAC, EthernetType: layers.EthernetTypeIPv4},
			&layers.IPv4{SrcIP: localIP, DstIP: host.ip, Protocol: layers.IPProtocolTCP, TTL: 64},
			&layers.TCP{SrcPort: layers.TCPPort(srcPort), DstPort: 443, ACK: true, PSH: true, Seq: rng.Uint32()},
			gopacket.Payload(hello),
		)

		// HTTPS response data (ACK + data)
		payload := make([]byte, 256+rng.Intn(1024))
		rng.Read(payload)
		payload[0] = 0x17 // TLS application_data record type
		payload[1] = 0x03
		payload[2] = 0x03
		for j := 0; j < 3; j++ {
			writePacket(flowTS.Add(time.Duration(10+j*30)*time.Millisecond),
				&layers.Ethernet{SrcMAC: routerMAC, DstMAC: localMAC, EthernetType: layers.EthernetTypeIPv4},
				&layers.IPv4{SrcIP: host.ip, DstIP: localIP, Protocol: layers.IPProtocolTCP, TTL: 52},
				&layers.TCP{SrcPort: 443, DstPort: layers.TCPPort(srcPort), ACK: true, PSH: true, Seq: rng.Uint32()},
				gopacket.Payload(payload),
			)
		}
		ts = flowTS.Add(100 * time.Millisecond)
	}

	// Burst of HTTPS data packets to build up bandwidth graph
	for i := 0; i < 200; i++ {
		host := hosts[rng.Intn(len(hosts))]
		pktTS := ts.Add(time.Duration(i*15) * time.Millisecond)
		size := 512 + rng.Intn(1024)
		payload := make([]byte, size)
		rng.Read(payload)
		payload[0] = 0x17; payload[1] = 0x03; payload[2] = 0x03

		tx := rng.Intn(2) == 0
		srcIP, dstIP := localIP, host.ip
		srcMAC, dstMAC := localMAC, routerMAC
		if !tx {
			srcIP, dstIP = host.ip, localIP
			srcMAC, dstMAC = routerMAC, localMAC
		}
		writePacket(pktTS,
			&layers.Ethernet{SrcMAC: srcMAC, DstMAC: dstMAC, EthernetType: layers.EthernetTypeIPv4},
			&layers.IPv4{SrcIP: srcIP, DstIP: dstIP, Protocol: layers.IPProtocolTCP, TTL: 64},
			&layers.TCP{SrcPort: layers.TCPPort(50000 + rng.Intn(20)), DstPort: 443, ACK: true, PSH: true, Seq: rng.Uint32()},
			gopacket.Payload(payload),
		)
	}
}

// buildClientHello builds a minimal TLS 1.3-compatible ClientHello with SNI.
func buildClientHello(sni []byte, cipherSuites []uint16) []byte {
	// SNI extension
	sniExt := []byte{}
	sniExt = append(sniExt, 0x00, byte(len(sni)+3)) // server_name_list length
	sniExt = append(sniExt, 0x00)                    // name_type: host_name
	sniExt = append(sniExt, byte(len(sni)>>8), byte(len(sni)))
	sniExt = append(sniExt, sni...)

	ext := []byte{}
	ext = append(ext, 0x00, 0x00)                                    // extension type: server_name
	ext = append(ext, byte(len(sniExt)>>8), byte(len(sniExt)))
	ext = append(ext, sniExt...)
	// supported versions extension (TLS 1.3 + 1.2)
	ext = append(ext, 0x00, 0x2b, 0x00, 0x05, 0x04, 0x03, 0x04, 0x03, 0x03)

	hello := []byte{}
	hello = append(hello, 0x01)               // HandshakeType: client_hello
	hello = append(hello, 0x00, 0x00, 0x00)   // length placeholder
	hello = append(hello, 0x03, 0x03)         // legacy version TLS 1.2
	random := make([]byte, 32)
	hello = append(hello, random...)
	hello = append(hello, 0x00)               // session id length 0

	cs := []byte{}
	for _, c := range cipherSuites {
		cs = append(cs, byte(c>>8), byte(c))
	}
	csLen := make([]byte, 2)
	binary.BigEndian.PutUint16(csLen, uint16(len(cs)))
	hello = append(hello, csLen...)
	hello = append(hello, cs...)
	hello = append(hello, 0x01, 0x00) // compression methods

	extLenB := make([]byte, 2)
	binary.BigEndian.PutUint16(extLenB, uint16(len(ext)))
	hello = append(hello, extLenB...)
	hello = append(hello, ext...)

	// fix handshake length
	hLen := len(hello) - 4
	hello[1] = byte(hLen >> 16)
	hello[2] = byte(hLen >> 8)
	hello[3] = byte(hLen)

	// TLS record header
	recLen := make([]byte, 2)
	binary.BigEndian.PutUint16(recLen, uint16(len(hello)))
	record := []byte{0x16, 0x03, 0x01}
	record = append(record, recLen...)
	record = append(record, hello...)
	return record
}
