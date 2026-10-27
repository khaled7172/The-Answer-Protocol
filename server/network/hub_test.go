package main

import (
	"tap/server/logger"
	"testing"
	"time"
)

func init() {
	logger.Init()
}

func newTestClient(name string) *Client {
	return &Client{
		name: name,
		send: make(chan string, 128),
	}
}

func TestHubNameTakenReturnsFalseInitially(t *testing.T) {
	h := NewHub()
	if h.NameTaken("alice") {
		t.Fatal("NameTaken returned true before any registration")
	}
}

func TestHubNameTakenReturnsTrueAfterRegister(t *testing.T) {
	h := NewHub()
	h.Register(newTestClient("alice"))
	if !h.NameTaken("alice") {
		t.Fatal("NameTaken returned false after Register")
	}
}

func TestHubPlayerCountTracksRegistrations(t *testing.T) {
	h := NewHub()
	if h.PlayerCount() != 0 {
		t.Fatalf("expected 0, got %d", h.PlayerCount())
	}
	h.Register(newTestClient("alice"))
	h.Register(newTestClient("bob"))
	if h.PlayerCount() != 2 {
		t.Fatalf("expected 2, got %d", h.PlayerCount())
	}
}

func TestHubUnregisterRemovesClient(t *testing.T) {
	h := NewHub()
	c := newTestClient("alice")
	h.Register(c)
	h.Unregister(c)
	if h.NameTaken("alice") {
		t.Fatal("alice still registered after Unregister")
	}
}

func TestHubRoomPlayersReturnsMatchingClients(t *testing.T) {
	h := NewHub()
	c1 := newTestClient("alice")
	c1.roomID = "hall"
	c2 := newTestClient("bob")
	c2.roomID = "hall"
	c3 := newTestClient("charlie")
	c3.roomID = "tavern"
	h.Register(c1)
	h.Register(c2)
	h.Register(c3)

	players := h.RoomPlayers("hall")
	if len(players) != 2 {
		t.Fatalf("expected 2 players in hall, got %d", len(players))
	}
}

func TestHubSendDispatchesBroadcast(t *testing.T) {
	h := NewHub()
	c := newTestClient("alice")
	h.Register(c)
	go h.Run()

	h.Send("global", "", "", "EVT TEST")

	select {
	case msg := <-c.send:
		if msg != "EVT TEST" {
			t.Fatalf("expected EVT TEST, got %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast message")
	}
}

func TestHubUnregisterBroadcastsLeaveEvt(t *testing.T) {
	h := NewHub()
	c1 := newTestClient("alice")
	c1.roomID = "hall"
	c2 := newTestClient("bob")
	c2.roomID = "hall"
	h.Register(c1)
	h.Register(c2)
	go h.Run()

	h.Unregister(c1)

	select {
	case msg := <-c2.send:
		if msg != "EVT ROOM PRESENCE LEAVE alice" {
			t.Fatalf("expected EVT ROOM PRESENCE LEAVE alice, got %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for presence leave event")
	}
}
