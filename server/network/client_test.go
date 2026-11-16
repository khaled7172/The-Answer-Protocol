package main

import (
	"bufio"
	"io"
	"net"
	"testing"
)

func TestEnqueueDropsWhenChannelFull(t *testing.T) {
	c := &Client{send: make(chan string, 1)}
	if !c.Enqueue("first") {
		t.Fatal("Enqueue refused on empty channel")
	}
	if c.Enqueue("second") {
		t.Fatal("Enqueue accepted on full channel")
	}
}

func TestQuitDeliversByeBeforeClosing(t *testing.T) {
	srv, cli := net.Pipe()
	c := NewClient(srv)
	go c.ReadLoop(newTestHub())
	go c.WriteLoop()

	r := bufio.NewReader(cli)
	go cli.Write([]byte("QUIT\n"))
	line, err := r.ReadString('\n')
	if err != nil || line != "OK bye\n" {
		t.Fatalf("expected OK bye, got %q (%v)", line, err)
	}
	if _, err := r.ReadByte(); err != io.EOF {
		t.Fatalf("expected connection closed after bye, got %v", err)
	}
}
