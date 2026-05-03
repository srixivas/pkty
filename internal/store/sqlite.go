package store

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"

	"github.com/c0d343v3r/pkty/internal/events"
)

const schema = `
CREATE TABLE IF NOT EXISTS packets (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	ts         DATETIME NOT NULL,
	number     INTEGER,
	src_ip     TEXT,
	src_port   INTEGER,
	dst_ip     TEXT,
	dst_port   INTEGER,
	protocol   TEXT,
	length     INTEGER,
	info       TEXT,
	raw_bytes  BLOB
);
CREATE TABLE IF NOT EXISTS dns_events (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	ts          DATETIME NOT NULL,
	src_ip      TEXT,
	dst_ip      TEXT,
	query_name  TEXT,
	record_type TEXT,
	resolved_ip TEXT,
	ttl         INTEGER,
	is_response INTEGER,
	is_nxdomain INTEGER
);
CREATE TABLE IF NOT EXISTS tls_events (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	ts           DATETIME NOT NULL,
	src_ip       TEXT,
	src_port     INTEGER,
	dst_ip       TEXT,
	dst_port     INTEGER,
	sni          TEXT,
	version      TEXT,
	cipher_suite TEXT
);
`

// DefaultPath returns the default SQLite database path.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "pkty.db"
	}
	return filepath.Join(home, ".local", "share", "pkty", "pkty.db")
}

type writeKind int

const (
	kindPacket writeKind = iota
	kindDNS
	kindTLS
)

type writeItem struct {
	kind writeKind
	pkt  events.PacketEvent
	dns  events.DNSEvent
	tls  events.TLSEvent
}

// SQLiteStore writes captured events to a SQLite database asynchronously.
type SQLiteStore struct {
	db   *sql.DB
	ch   chan writeItem
	done chan struct{}
}

// NewSQLiteStore opens (or creates) the SQLite database at path and starts a
// background goroutine that drains the write queue in batched transactions.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	// Enable WAL mode for better write throughput.
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	s := &SQLiteStore{
		db:   db,
		ch:   make(chan writeItem, 8192),
		done: make(chan struct{}),
	}
	go s.run()
	return s, nil
}

// WritePacket queues a packet event for async write. Never blocks.
func (s *SQLiteStore) WritePacket(evt events.PacketEvent) {
	select {
	case s.ch <- writeItem{kind: kindPacket, pkt: evt}:
	default: // drop on overflow rather than blocking the UI
	}
}

// WriteDNS queues a DNS event for async write. Never blocks.
func (s *SQLiteStore) WriteDNS(evt events.DNSEvent) {
	select {
	case s.ch <- writeItem{kind: kindDNS, dns: evt}:
	default:
	}
}

// WriteTLS queues a TLS event for async write. Never blocks.
func (s *SQLiteStore) WriteTLS(evt events.TLSEvent) {
	select {
	case s.ch <- writeItem{kind: kindTLS, tls: evt}:
	default:
	}
}

// Close flushes pending writes and closes the database.
func (s *SQLiteStore) Close() error {
	close(s.ch)
	<-s.done
	return s.db.Close()
}

func (s *SQLiteStore) run() {
	defer close(s.done)

	insertPkt, _ := s.db.Prepare(
		`INSERT INTO packets(ts,number,src_ip,src_port,dst_ip,dst_port,protocol,length,info,raw_bytes)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`)
	insertDNS, _ := s.db.Prepare(
		`INSERT INTO dns_events(ts,src_ip,dst_ip,query_name,record_type,resolved_ip,ttl,is_response,is_nxdomain)
		 VALUES(?,?,?,?,?,?,?,?,?)`)
	insertTLS, _ := s.db.Prepare(
		`INSERT INTO tls_events(ts,src_ip,src_port,dst_ip,dst_port,sni,version,cipher_suite)
		 VALUES(?,?,?,?,?,?,?,?)`)
	defer func() {
		if insertPkt != nil {
			insertPkt.Close()
		}
		if insertDNS != nil {
			insertDNS.Close()
		}
		if insertTLS != nil {
			insertTLS.Close()
		}
	}()

	const batchSize = 500
	buf := make([]writeItem, 0, batchSize)

	flush := func() {
		if len(buf) == 0 {
			return
		}
		tx, err := s.db.Begin()
		if err != nil {
			buf = buf[:0]
			return
		}
		tPkt := tx.Stmt(insertPkt)
		tDNS := tx.Stmt(insertDNS)
		tTLS := tx.Stmt(insertTLS)
		defer tPkt.Close()
		defer tDNS.Close()
		defer tTLS.Close()

		for _, item := range buf {
			switch item.kind {
			case kindPacket:
				e := item.pkt
				srcIP, dstIP := ipStr(e.SrcIP), ipStr(e.DstIP)
				tPkt.Exec(e.Timestamp, e.Number, srcIP, e.SrcPort, dstIP, e.DstPort, e.Protocol, e.Length, e.Info, e.RawBytes) //nolint:errcheck
			case kindDNS:
				e := item.dns
				tDNS.Exec(e.Timestamp, ipStr(e.SrcIP), ipStr(e.DstIP), e.QueryName, e.RecordType, ipStr(e.ResolvedIP), e.TTL, boolInt(e.IsResponse), boolInt(e.IsNXDomain)) //nolint:errcheck
			case kindTLS:
				e := item.tls
				tTLS.Exec(e.Timestamp, ipStr(e.SrcIP), e.SrcPort, ipStr(e.DstIP), e.DstPort, e.SNI, e.Version, e.CipherSuite) //nolint:errcheck
			}
		}
		tx.Commit() //nolint:errcheck
		buf = buf[:0]
	}

	for item := range s.ch {
		buf = append(buf, item)
		if len(buf) >= batchSize {
			flush()
		}
	}
	flush() // drain remaining items after channel close
}

func ipStr(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
