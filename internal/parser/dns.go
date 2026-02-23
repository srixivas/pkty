package parser

import (
	"fmt"
	"strings"

	"github.com/google/gopacket/layers"

	"github.com/c0d343v3r/netdash/internal/events"
)

// parseDNS extracts DNS query/response data and emits DNSEvents.
func (p *Parser) parseDNS(dns *layers.DNS, evt *events.PacketEvent) {
	evt.Protocol = "DNS"

	if dns.QR { // Response
		p.parseDNSResponse(dns, evt)
	} else { // Query
		p.parseDNSQuery(dns, evt)
	}
}

func (p *Parser) parseDNSQuery(dns *layers.DNS, evt *events.PacketEvent) {
	var names []string
	for _, q := range dns.Questions {
		qName := string(q.Name)
		qType := dnsTypeString(q.Type)
		names = append(names, fmt.Sprintf("%s %s", qType, qName))

		dnsEvt := events.DNSEvent{
			Timestamp:  evt.Timestamp,
			SrcIP:      evt.SrcIP,
			DstIP:      evt.DstIP,
			QueryName:  qName,
			RecordType: qType,
			IsResponse: false,
		}
		select {
		case p.bus.DNS <- dnsEvt:
		default:
		}
	}
	evt.Info = fmt.Sprintf("Query %s", strings.Join(names, ", "))

	evt.LayerData = append(evt.LayerData, events.LayerInfo{
		Name: "DNS Query",
		Fields: dnsQuestionFields(dns),
	})
}

func (p *Parser) parseDNSResponse(dns *layers.DNS, evt *events.PacketEvent) {
	isNX := dns.ResponseCode == layers.DNSResponseCodeNXDomain

	var parts []string
	for _, ans := range dns.Answers {
		qType := dnsTypeString(ans.Type)
		val := string(ans.Name)

		if ans.IP != nil {
			val = fmt.Sprintf("%s → %s", ans.Name, ans.IP)
		} else if ans.CNAME != nil {
			val = fmt.Sprintf("%s → %s", ans.Name, string(ans.CNAME))
		}

		parts = append(parts, fmt.Sprintf("%s %s", qType, val))

		dnsEvt := events.DNSEvent{
			Timestamp:  evt.Timestamp,
			SrcIP:      evt.SrcIP,
			DstIP:      evt.DstIP,
			QueryName:  string(ans.Name),
			RecordType: qType,
			ResolvedIP: ans.IP,
			TTL:        ans.TTL,
			IsResponse: true,
			IsNXDomain: isNX,
		}
		select {
		case p.bus.DNS <- dnsEvt:
		default:
		}
	}

	if isNX && len(dns.Questions) > 0 {
		evt.Info = fmt.Sprintf("NXDOMAIN %s", string(dns.Questions[0].Name))
	} else if len(parts) > 0 {
		evt.Info = fmt.Sprintf("Response %s", strings.Join(parts, ", "))
	} else {
		evt.Info = fmt.Sprintf("Response (code=%s)", dns.ResponseCode.String())
	}

	evt.LayerData = append(evt.LayerData, events.LayerInfo{
		Name:   "DNS Response",
		Fields: dnsAnswerFields(dns),
	})
}

func dnsQuestionFields(dns *layers.DNS) []events.FieldInfo {
	var fields []events.FieldInfo
	fields = append(fields, events.FieldInfo{Key: "Transaction ID", Value: fmt.Sprintf("0x%04x", dns.ID)})
	for i, q := range dns.Questions {
		prefix := fmt.Sprintf("Q%d", i)
		fields = append(fields,
			events.FieldInfo{Key: prefix + " Name", Value: string(q.Name)},
			events.FieldInfo{Key: prefix + " Type", Value: dnsTypeString(q.Type)},
			events.FieldInfo{Key: prefix + " Class", Value: q.Class.String()},
		)
	}
	return fields
}

func dnsAnswerFields(dns *layers.DNS) []events.FieldInfo {
	var fields []events.FieldInfo
	fields = append(fields,
		events.FieldInfo{Key: "Transaction ID", Value: fmt.Sprintf("0x%04x", dns.ID)},
		events.FieldInfo{Key: "Response Code", Value: dns.ResponseCode.String()},
		events.FieldInfo{Key: "Answer Count", Value: fmt.Sprintf("%d", dns.ANCount)},
	)
	for i, ans := range dns.Answers {
		prefix := fmt.Sprintf("A%d", i)
		fields = append(fields,
			events.FieldInfo{Key: prefix + " Name", Value: string(ans.Name)},
			events.FieldInfo{Key: prefix + " Type", Value: dnsTypeString(ans.Type)},
			events.FieldInfo{Key: prefix + " TTL", Value: fmt.Sprintf("%d", ans.TTL)},
		)
		if ans.IP != nil {
			fields = append(fields, events.FieldInfo{Key: prefix + " IP", Value: ans.IP.String()})
		}
		if ans.CNAME != nil {
			fields = append(fields, events.FieldInfo{Key: prefix + " CNAME", Value: string(ans.CNAME)})
		}
	}
	return fields
}

func dnsTypeString(t layers.DNSType) string {
	switch t {
	case layers.DNSTypeA:
		return "A"
	case layers.DNSTypeAAAA:
		return "AAAA"
	case layers.DNSTypeCNAME:
		return "CNAME"
	case layers.DNSTypeMX:
		return "MX"
	case layers.DNSTypeNS:
		return "NS"
	case layers.DNSTypePTR:
		return "PTR"
	case layers.DNSTypeSOA:
		return "SOA"
	case layers.DNSTypeSRV:
		return "SRV"
	case layers.DNSTypeTXT:
		return "TXT"
	default:
		return fmt.Sprintf("TYPE%d", t)
	}
}
