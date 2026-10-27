package main

import (
	"net"
	"sync"
	"time"
)

type ReconnectTracker struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewReconnectTracker(limit int, window time.Duration) *ReconnectTracker {
	return &ReconnectTracker{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (t *ReconnectTracker) Record(ip string) (bool, int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-t.window)

	var valid []time.Time
	for _, ts := range t.attempts[ip] {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	valid = append(valid, now)
	t.attempts[ip] = valid

	if len(t.attempts) > 256 {
		for k, v := range t.attempts {
			var active []time.Time
			for _, ts := range v {
				if ts.After(cutoff) {
					active = append(active, ts)
				}
			}
			if len(active) == 0 {
				delete(t.attempts, k)
			} else {
				t.attempts[k] = active
			}
		}
	}

	count := len(valid)
	return count > t.limit, count
}

func extractIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
