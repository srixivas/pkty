package parser

import (
	"encoding/binary"
	"fmt"

	"github.com/c0d343v3r/netdash/internal/events"
)

// parseTLS extracts TLS handshake info (SNI, version, cipher) from ClientHello.
func (p *Parser) parseTLS(payload []byte, evt *events.PacketEvent) {
	evt.Protocol = "TLS"

	// TLS record header: ContentType(1) + Version(2) + Length(2)
	if len(payload) < 5 {
		return
	}

	recordVersion := tlsVersionString(payload[1], payload[2])
	recordLen := int(binary.BigEndian.Uint16(payload[3:5]))

	if len(payload) < 5+recordLen {
		return
	}

	// Handshake header: HandshakeType(1) + Length(3)
	hs := payload[5:]
	if len(hs) < 4 {
		return
	}

	hsType := hs[0]
	if hsType != 1 { // Not ClientHello
		evt.Info = fmt.Sprintf("TLS %s Handshake", recordVersion)
		return
	}

	// ClientHello: Version(2) + Random(32) + SessionIDLen(1) + ...
	ch := hs[4:]
	if len(ch) < 34 {
		return
	}

	clientVersion := tlsVersionString(ch[0], ch[1])
	// Skip Random (32 bytes)
	pos := 34

	// Session ID
	if pos >= len(ch) {
		return
	}
	sessionIDLen := int(ch[pos])
	pos += 1 + sessionIDLen

	// Cipher suites
	if pos+2 > len(ch) {
		return
	}
	cipherLen := int(binary.BigEndian.Uint16(ch[pos : pos+2]))
	pos += 2

	var firstCipher uint16
	if cipherLen >= 2 && pos+2 <= len(ch) {
		firstCipher = binary.BigEndian.Uint16(ch[pos : pos+2])
	}
	pos += cipherLen

	// Compression methods
	if pos >= len(ch) {
		return
	}
	compLen := int(ch[pos])
	pos += 1 + compLen

	// Extensions
	sni := ""
	negotiatedVersion := clientVersion

	if pos+2 <= len(ch) {
		extLen := int(binary.BigEndian.Uint16(ch[pos : pos+2]))
		pos += 2
		extEnd := pos + extLen
		if extEnd > len(ch) {
			extEnd = len(ch)
		}

		for pos+4 <= extEnd {
			extType := binary.BigEndian.Uint16(ch[pos : pos+2])
			extDataLen := int(binary.BigEndian.Uint16(ch[pos+2 : pos+4]))
			pos += 4

			if pos+extDataLen > extEnd {
				break
			}

			switch extType {
			case 0x0000: // SNI
				sni = parseSNI(ch[pos : pos+extDataLen])
			case 0x002b: // supported_versions
				if extDataLen > 1 {
					v := parseSupportedVersions(ch[pos : pos+extDataLen])
					if v != "" {
						negotiatedVersion = v
					}
				}
			}

			pos += extDataLen
		}
	}

	cipherStr := fmt.Sprintf("0x%04x", firstCipher)

	if sni != "" {
		evt.Info = fmt.Sprintf("ClientHello %s SNI=%s", negotiatedVersion, sni)
	} else {
		evt.Info = fmt.Sprintf("ClientHello %s", negotiatedVersion)
	}

	evt.LayerData = append(evt.LayerData, events.LayerInfo{
		Name: "TLS ClientHello",
		Fields: []events.FieldInfo{
			{Key: "Version", Value: negotiatedVersion},
			{Key: "SNI", Value: sni},
			{Key: "Cipher Suite", Value: cipherStr},
			{Key: "Record Version", Value: recordVersion},
		},
	})

	// Emit TLS event
	tlsEvt := events.TLSEvent{
		Timestamp:   evt.Timestamp,
		SrcIP:       evt.SrcIP,
		DstIP:       evt.DstIP,
		SrcPort:     evt.SrcPort,
		DstPort:     evt.DstPort,
		SNI:         sni,
		Version:     negotiatedVersion,
		CipherSuite: cipherStr,
	}
	select {
	case p.bus.TLS <- tlsEvt:
	default:
	}
}

func parseSNI(data []byte) string {
	// SNI extension: ListLength(2) + Type(1) + NameLength(2) + Name
	if len(data) < 5 {
		return ""
	}
	nameLen := int(binary.BigEndian.Uint16(data[3:5]))
	if len(data) < 5+nameLen {
		return ""
	}
	return string(data[5 : 5+nameLen])
}

func parseSupportedVersions(data []byte) string {
	// First byte is length of the version list
	if len(data) < 3 {
		return ""
	}
	// Return the first (highest priority) version
	return tlsVersionString(data[1], data[2])
}

func tlsVersionString(major, minor byte) string {
	v := uint16(major)<<8 | uint16(minor)
	switch v {
	case 0x0304:
		return "TLS 1.3"
	case 0x0303:
		return "TLS 1.2"
	case 0x0302:
		return "TLS 1.1"
	case 0x0301:
		return "TLS 1.0"
	case 0x0300:
		return "SSL 3.0"
	default:
		return fmt.Sprintf("TLS 0x%04x", v)
	}
}
