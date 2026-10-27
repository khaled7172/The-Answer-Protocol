package main

import (
	"sync"
	"testing"
	"time"
)

func TestReconnectTrackerAllowsUpToLimit(t *testing.T) {
	tracker := NewReconnectTracker(5, 10*time.Second)
	ip := "192.168.1.10"

	for i := 1; i <= 5; i++ {
		abused, count := tracker.Record(ip)
		if abused {
			t.Fatalf("expected attempt %d to not trigger abuse", i)
		}
		if count != i {
			t.Fatalf("expected count %d, got %d", i, count)
		}
	}
}

func TestReconnectTrackerTriggersAbuseBeyondLimit(t *testing.T) {
	tracker := NewReconnectTracker(5, 10*time.Second)
	ip := "192.168.1.10"

	for i := 1; i <= 5; i++ {
		tracker.Record(ip)
	}

	abused, count := tracker.Record(ip)
	if !abused {
		t.Fatal("expected 6th attempt to trigger abuse")
	}
	if count != 6 {
		t.Fatalf("expected count 6, got %d", count)
	}

	abused7, count7 := tracker.Record(ip)
	if !abused7 || count7 != 7 {
		t.Fatalf("expected 7th attempt to trigger abuse with count 7, got %v, %d", abused7, count7)
	}
}

func TestReconnectTrackerDifferentIPsIsolated(t *testing.T) {
	tracker := NewReconnectTracker(5, 10*time.Second)
	ip1 := "10.0.0.1"
	ip2 := "10.0.0.2"

	for i := 0; i < 5; i++ {
		tracker.Record(ip1)
	}

	abused, count := tracker.Record(ip2)
	if abused || count != 1 {
		t.Fatalf("expected ip2 to be clean on first attempt, got abused=%v count=%d", abused, count)
	}
}

func TestReconnectTrackerWindowExpiry(t *testing.T) {
	tracker := NewReconnectTracker(2, 50*time.Millisecond)
	ip := "172.16.0.1"

	tracker.Record(ip)
	tracker.Record(ip)

	time.Sleep(60 * time.Millisecond)

	abused, count := tracker.Record(ip)
	if abused {
		t.Fatal("expected attempt after window to not be abused")
	}
	if count != 1 {
		t.Fatalf("expected count 1 after expiry, got %d", count)
	}
}

func TestExtractIP(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"127.0.0.1:8080", "127.0.0.1"},
		{"[::1]:9000", "::1"},
		{"192.168.1.100", "192.168.1.100"},
		{"invalid", "invalid"},
	}

	for _, tc := range cases {
		got := extractIP(tc.input)
		if got != tc.expected {
			t.Fatalf("extractIP(%q) = %q; expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestReconnectTrackerConcurrentAccess(t *testing.T) {
	tracker := NewReconnectTracker(100, time.Second)
	ip := "1.2.3.4"
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.Record(ip)
		}()
	}

	wg.Wait()
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if len(tracker.attempts[ip]) != 20 {
		t.Fatalf("expected 20 attempts recorded concurrently, got %d", len(tracker.attempts[ip]))
	}
}
