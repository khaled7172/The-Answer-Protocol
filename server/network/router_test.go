package main

import (
	"net"
	"tap/server/logger"
	"testing"
)

func init() {
	logger.Init()
}

func TestRouteEmptyLineProducesNoResponse(t *testing.T) {
	h := NewHub()
	c := &Client{send: make(chan string, 128)}
	Route(h, c, "")
	select {
	case msg := <-c.send:
		t.Fatalf("expected no response for empty line, got %q", msg)
	default:
	}
}

func TestRouteConnectRegistersClientAndResponds(t *testing.T) {
	h := NewHub()
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	c := NewClient(cli)
	Route(h, c, "CONNECT alice")

	resp := <-c.send
	if resp != "OK connected" {
		t.Fatalf("expected OK connected, got %q", resp)
	}
	if c.name != "alice" {
		t.Fatalf("expected name alice, got %q", c.name)
	}
	if !h.NameTaken("alice") {
		t.Fatal("alice not registered in hub after CONNECT")
	}
}

func TestRouteConnectDuplicateNameReturnsError(t *testing.T) {
	h := NewHub()
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	h.Register(newTestClient("alice"))

	c := NewClient(cli)
	Route(h, c, "CONNECT alice")

	resp := <-c.send
	if resp != "ERR 201 NAME_IN_USE" {
		t.Fatalf("expected ERR 201 NAME_IN_USE, got %q", resp)
	}
}

func TestRouteCommandBeforeConnectReturnsError(t *testing.T) {
	h := NewHub()
	c := &Client{send: make(chan string, 128)}
	Route(h, c, "LOOK")
	resp := <-c.send
	if resp != "ERR 100 NOT_CONNECTED" {
		t.Fatalf("expected ERR 100 NOT_CONNECTED, got %q", resp)
	}
}

func TestRouteUnknownCommandAfterConnectReturnsOK(t *testing.T) {
	h := NewHub()
	c := newTestClient("alice")
	h.Register(c)
	Route(h, c, "LOOK")
	resp := <-c.send
	if resp != "OK" {
		t.Fatalf("expected OK, got %q", resp)
	}
}

func TestRateAllowPermitsUpToLimit(t *testing.T) {
	c := &Client{send: make(chan string, 128)}
	for i := 0; i < maxCmdsPerSec; i++ {
		if !rateAllow(c) {
			t.Fatalf("rateAllow blocked at iteration %d, expected allow", i)
		}
	}
	if rateAllow(c) {
		t.Fatal("rateAllow allowed beyond limit")
	}
}
