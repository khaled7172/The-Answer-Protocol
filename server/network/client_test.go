package main

import (
	"net"
	"testing"
)

func TestNewClientInitializesSendChannel(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	c := NewClient(cli)
	if c.send == nil {
		t.Fatal("NewClient did not initialize send channel")
	}
}

func TestEnqueueReturnsTrueWhenChannelHasSpace(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	c := NewClient(cli)
	if !c.Enqueue("hello") {
		t.Fatal("Enqueue returned false on non-full channel")
	}
}

func TestEnqueueReturnsFalseWhenChannelFull(t *testing.T) {
	c := &Client{send: make(chan string, 1)}
	c.send <- "first"
	if c.Enqueue("second") {
		t.Fatal("Enqueue returned true on full channel")
	}
}
