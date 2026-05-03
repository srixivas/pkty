package parser

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/c0d343v3r/pkty/internal/events"
)

var httpMethods = [][]byte{
	[]byte("GET "),
	[]byte("POST "),
	[]byte("PUT "),
	[]byte("DELETE "),
	[]byte("HEAD "),
	[]byte("OPTIONS "),
	[]byte("PATCH "),
	[]byte("CONNECT "),
}

// isHTTP checks if the payload starts with an HTTP method or response.
func isHTTP(payload []byte) bool {
	if bytes.HasPrefix(payload, []byte("HTTP/")) {
		return true
	}
	for _, m := range httpMethods {
		if bytes.HasPrefix(payload, m) {
			return true
		}
	}
	return false
}

// parseHTTP extracts HTTP request/response info from TCP payload.
func (p *Parser) parseHTTP(payload []byte, evt *events.PacketEvent) {
	evt.Protocol = "HTTP"

	// Find the first line
	idx := bytes.IndexByte(payload, '\n')
	if idx < 0 {
		idx = len(payload)
	}
	firstLine := strings.TrimSpace(string(payload[:idx]))

	if bytes.HasPrefix(payload, []byte("HTTP/")) {
		// Response: "HTTP/1.1 200 OK"
		p.parseHTTPResponse(payload, firstLine, evt)
	} else {
		// Request: "GET /path HTTP/1.1"
		p.parseHTTPRequest(payload, firstLine, evt)
	}
}

func (p *Parser) parseHTTPRequest(payload []byte, firstLine string, evt *events.PacketEvent) {
	parts := strings.SplitN(firstLine, " ", 3)
	method := ""
	url := ""
	if len(parts) >= 2 {
		method = parts[0]
		url = parts[1]
	}

	evt.Info = fmt.Sprintf("%s %s", method, url)

	fields := []events.FieldInfo{
		{Key: "Method", Value: method},
		{Key: "URL", Value: url},
	}

	// Extract Host header
	if host := extractHeader(payload, "Host"); host != "" {
		fields = append(fields, events.FieldInfo{Key: "Host", Value: host})
	}
	if ct := extractHeader(payload, "Content-Type"); ct != "" {
		fields = append(fields, events.FieldInfo{Key: "Content-Type", Value: ct})
	}

	evt.LayerData = append(evt.LayerData, events.LayerInfo{
		Name:   "HTTP Request",
		Fields: fields,
	})

	httpEvt := events.HTTPEvent{
		Timestamp: evt.Timestamp,
		SrcIP:     evt.SrcIP,
		DstIP:     evt.DstIP,
		Method:    method,
		URL:       url,
	}
	select {
	case p.bus.HTTP <- httpEvt:
	default:
	}
}

func (p *Parser) parseHTTPResponse(payload []byte, firstLine string, evt *events.PacketEvent) {
	parts := strings.SplitN(firstLine, " ", 3)
	statusCode := 0
	statusText := ""
	if len(parts) >= 2 {
		statusCode, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		statusText = parts[2]
	}

	evt.Info = fmt.Sprintf("HTTP %d %s", statusCode, statusText)

	fields := []events.FieldInfo{
		{Key: "Status Code", Value: fmt.Sprintf("%d", statusCode)},
		{Key: "Status", Value: statusText},
	}

	if ct := extractHeader(payload, "Content-Type"); ct != "" {
		fields = append(fields, events.FieldInfo{Key: "Content-Type", Value: ct})
	}
	if cl := extractHeader(payload, "Content-Length"); cl != "" {
		fields = append(fields, events.FieldInfo{Key: "Content-Length", Value: cl})
	}

	evt.LayerData = append(evt.LayerData, events.LayerInfo{
		Name:   "HTTP Response",
		Fields: fields,
	})

	httpEvt := events.HTTPEvent{
		Timestamp:  evt.Timestamp,
		SrcIP:      evt.SrcIP,
		DstIP:      evt.DstIP,
		StatusCode: statusCode,
	}
	if ct := extractHeader(payload, "Content-Type"); ct != "" {
		httpEvt.ContentType = ct
	}
	select {
	case p.bus.HTTP <- httpEvt:
	default:
	}
}

func extractHeader(payload []byte, name string) string {
	search := []byte(name + ": ")
	idx := bytes.Index(bytes.ToLower(payload), bytes.ToLower(search))
	if idx < 0 {
		return ""
	}
	start := idx + len(search)
	end := bytes.IndexByte(payload[start:], '\r')
	if end < 0 {
		end = bytes.IndexByte(payload[start:], '\n')
	}
	if end < 0 {
		end = len(payload[start:])
	}
	return strings.TrimSpace(string(payload[start : start+end]))
}
