// Package resolve provides a transient in-session reverse-DNS cache.
// When the layout sees a new IP address it calls Resolve(); a background
// goroutine fires a PTR lookup and delivers the result back to the bubbletea
// update loop as a Msg.  All data is in-memory and resets on exit.
package resolve

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Msg is delivered to the bubbletea Update loop when a PTR lookup completes.
type Msg struct {
	IP   string
	Name string
}

// Resolver maintains an in-memory IP → hostname cache populated from two
// sources:
//   - Observed DNS traffic (via Seed) — takes priority, no extra lookup fired.
//   - Async PTR lookups (via Resolve) — used for IPs we never saw DNS for.
type Resolver struct {
	mu      sync.Mutex
	cache   map[string]string // "" means "tried, got NXDOMAIN — don't retry"
	pending map[string]bool
	sem     chan struct{} // cap = max concurrent PTR goroutines
}

// New returns a Resolver ready for use.
func New() *Resolver {
	return &Resolver{
		cache:   make(map[string]string),
		pending: make(map[string]bool),
		sem:     make(chan struct{}, 20),
	}
}

// Name returns the cached hostname for ip, or "" if not yet resolved.
func (r *Resolver) Name(ip string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cache[ip]
}

// Seed records an IP→name mapping observed from DNS traffic without firing a
// PTR lookup.  This also prevents a redundant lookup for that IP later.
func (r *Resolver) Seed(ip, name string) {
	if ip == "" || name == "" {
		return
	}
	r.mu.Lock()
	r.cache[ip] = name
	r.mu.Unlock()
}

// Resolve returns a tea.Cmd that fires a reverse DNS PTR lookup for ip.
// Returns nil (no-op) if ip is empty, already in the cache, or already
// in-flight.  The Cmd itself acquires one slot from the semaphore so at most
// 20 goroutines are doing PTR lookups at a time.
func (r *Resolver) Resolve(ip string) tea.Cmd {
	if ip == "" {
		return nil
	}
	r.mu.Lock()
	if _, exists := r.cache[ip]; exists {
		r.mu.Unlock()
		return nil
	}
	if r.pending[ip] {
		r.mu.Unlock()
		return nil
	}
	r.pending[ip] = true
	r.mu.Unlock()

	return func() tea.Msg {
		r.sem <- struct{}{}
		defer func() { <-r.sem }()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		names, err := net.DefaultResolver.LookupAddr(ctx, ip)

		r.mu.Lock()
		delete(r.pending, ip)
		if err == nil && len(names) > 0 {
			name := strings.TrimSuffix(names[0], ".")
			r.cache[ip] = name
			r.mu.Unlock()
			return Msg{IP: ip, Name: name}
		}
		r.cache[ip] = "" // mark tried; don't retry
		r.mu.Unlock()
		return nil
	}
}
