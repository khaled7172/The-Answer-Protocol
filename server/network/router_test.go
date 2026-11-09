package main

import (
	"encoding/json"
	"net"
	"strings"
	"tap/server/logger"
	"testing"
	"time"
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

func TestRouteChatGlobalBroadcastsEvt(t *testing.T) {
	h := NewHub()
	go h.Run()

	c1 := newTestClient("alice")
	c2 := newTestClient("bob")
	h.Register(c1)
	h.Register(c2)

	Route(h, c1, "CHAT GLOBAL hello everyone")

	resp1 := <-c1.send
	if resp1 != "OK" {
		t.Fatalf("expected OK, got %q", resp1)
	}

	select {
	case msg := <-c2.send:
		if msg != "EVT GLOBAL CHAT alice hello everyone" {
			t.Fatalf("expected EVT GLOBAL CHAT alice hello everyone, got %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for global chat EVT")
	}
}

func TestRouteChatRoomBroadcastsToSameRoomOnly(t *testing.T) {
	h := NewHub()
	go h.Run()

	c1 := newTestClient("alice")
	c1.roomID = "dungeon"
	c2 := newTestClient("bob")
	c2.roomID = "dungeon"
	c3 := newTestClient("charlie")
	c3.roomID = "tavern"

	h.Register(c1)
	h.Register(c2)
	h.Register(c3)

	Route(h, c1, "CHAT ROOM watch out")

	resp1 := <-c1.send
	if resp1 != "OK" {
		t.Fatalf("expected OK, got %q", resp1)
	}

	select {
	case msg := <-c2.send:
		if msg != "EVT ROOM CHAT alice watch out" {
			t.Fatalf("expected EVT ROOM CHAT alice watch out, got %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for room chat EVT")
	}

	select {
	case msg := <-c3.send:
		t.Fatalf("charlie in tavern should not receive dungeon chat, got %q", msg)
	default:
	}
}

func TestRouteChatValidationErrors(t *testing.T) {
	h := NewHub()
	c := newTestClient("alice")
	h.Register(c)

	Route(h, c, "CHAT")
	if resp := <-c.send; resp != "ERR 100 MISSING_ARG" {
		t.Fatalf("expected ERR 100 MISSING_ARG, got %q", resp)
	}

	Route(h, c, "CHAT UNKNOWN hello")
	if resp := <-c.send; resp != "ERR 100 INVALID_SCOPE" {
		t.Fatalf("expected ERR 100 INVALID_SCOPE, got %q", resp)
	}

	Route(h, c, "CHAT ROOM hello")
	if resp := <-c.send; resp != "ERR 100 NOT_IN_ROOM" {
		t.Fatalf("expected ERR 100 NOT_IN_ROOM, got %q", resp)
	}
}

func TestRouteWhoReturnsJsonCounts(t *testing.T) {
	h := NewHub()
	c1 := newTestClient("alice")
	c1.roomID = "courtyard"
	c2 := newTestClient("bob")
	c2.roomID = "courtyard"
	h.Register(c1)
	h.Register(c2)

	Route(h, c1, "WHO")
	resp := <-c1.send
	if !strings.HasPrefix(resp, "OK ") {
		t.Fatalf("expected OK prefix, got %q", resp)
	}

	rawJSON := strings.TrimPrefix(resp, "OK ")
	var data whoResponse
	if err := json.Unmarshal([]byte(rawJSON), &data); err != nil {
		t.Fatalf("failed to parse WHO json: %v", err)
	}

	if data.Server != 2 {
		t.Fatalf("expected server total 2, got %d", data.Server)
	}
	if len(data.Room) != 2 {
		t.Fatalf("expected 2 room occupants, got %d", len(data.Room))
	}
}

func TestRouteGroupCreate(t *testing.T) {
	h := NewHub()
	c := newTestClient("alice")
	h.Register(c)

	Route(h, c, "GROUP CREATE")
	resp := <-c.send
	if resp != "OK group=grp_alice" {
		t.Fatalf("expected OK group=grp_alice, got %q", resp)
	}

	Route(h, c, "GROUP CREATE")
	if resp2 := <-c.send; resp2 != "ERR 402 ALREADY_IN_GROUP" {
		t.Fatalf("expected ERR 402 ALREADY_IN_GROUP, got %q", resp2)
	}
}

func TestRouteGroupInviteSucceeds(t *testing.T) {
	h := NewHub()
	go h.Run()

	c1 := newTestClient("alice")
	c1.groupID = "grp_alice"
	c2 := newTestClient("bob")
	h.Register(c1)
	h.Register(c2)

	Route(h, c1, "GROUP INVITE bob")

	resp := <-c1.send
	if resp != "OK" {
		t.Fatalf("expected OK, got %q", resp)
	}

	select {
	case msg := <-c2.send:
		if msg != "EVT GROUP INVITE alice" {
			t.Fatalf("expected EVT GROUP INVITE alice, got %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for group invite EVT")
	}
}

func TestRouteGroupInviteNotInGroup(t *testing.T) {
	h := NewHub()
	c := newTestClient("alice")
	h.Register(c)

	Route(h, c, "GROUP INVITE bob")
	if resp := <-c.send; resp != "ERR 401 NOT_IN_GROUP" {
		t.Fatalf("expected ERR 401 NOT_IN_GROUP, got %q", resp)
	}
}

func TestRouteGroupInviteTargetNotFound(t *testing.T) {
	h := NewHub()
	c := newTestClient("alice")
	c.groupID = "grp_alice"
	h.Register(c)

	Route(h, c, "GROUP INVITE ghost")
	if resp := <-c.send; resp != "ERR 404 NOT_FOUND" {
		t.Fatalf("expected ERR 404 NOT_FOUND, got %q", resp)
	}
}

func TestRouteGroupJoinAndLeave(t *testing.T) {
	h := NewHub()
	go h.Run()

	c1 := newTestClient("alice")
	c1.groupID = "grp_alice"
	c2 := newTestClient("bob")
	h.Register(c1)
	h.Register(c2)

	Route(h, c2, "GROUP JOIN alice")
	resp := <-c2.send
	if resp != "OK group=grp_alice" {
		t.Fatalf("expected OK group=grp_alice, got %q", resp)
	}

	select {
	case msg := <-c1.send:
		if msg != "EVT GROUP JOIN bob" {
			t.Fatalf("expected EVT GROUP JOIN bob, got %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for group join EVT on c1")
	}

	select {
	case <-c2.send:
	default:
	}

	Route(h, c2, "GROUP LEAVE")
	respLeave := <-c2.send
	if respLeave != "OK" {
		t.Fatalf("expected OK, got %q", respLeave)
	}

	select {
	case msg := <-c1.send:
		if msg != "EVT GROUP LEAVE bob" {
			t.Fatalf("expected EVT GROUP LEAVE bob, got %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for group leave EVT")
	}
}

func TestRouteChatGroup(t *testing.T) {
	h := NewHub()
	go h.Run()

	c1 := newTestClient("alice")
	c1.groupID = "grp_team"
	c2 := newTestClient("bob")
	c2.groupID = "grp_team"
	c3 := newTestClient("charlie")
	c3.groupID = "grp_other"

	h.Register(c1)
	h.Register(c2)
	h.Register(c3)

	Route(h, c1, "CHAT GROUP team meeting now")
	resp := <-c1.send
	if resp != "OK" {
		t.Fatalf("expected OK, got %q", resp)
	}

	select {
	case msg := <-c2.send:
		if msg != "EVT GROUP CHAT alice team meeting now" {
			t.Fatalf("expected EVT GROUP CHAT, got %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for group chat EVT")
	}

	select {
	case msg := <-c3.send:
		t.Fatalf("unexpected message delivered to other group: %q", msg)
	default:
	}
}

func TestRouteMoveEmitsLeaveAndEnterEvts(t *testing.T) {
	h := NewHub()
	go h.Run()

	c1 := newTestClient("alice")
	c1.roomID = "roomA"
	c2 := newTestClient("bob")
	c2.roomID = "roomA"
	c3 := newTestClient("charlie")
	c3.roomID = "roomB"

	h.Register(c1)
	h.Register(c2)
	h.Register(c3)

	Route(h, c1, "MOVE roomB")

	resp := <-c1.send
	if resp != "OK room=roomB" {
		t.Fatalf("expected OK room=roomB, got %q", resp)
	}

	select {
	case msg := <-c2.send:
		if msg != "EVT ROOM PRESENCE LEAVE alice" {
			t.Fatalf("expected LEAVE evt in roomA, got %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for LEAVE evt in roomA")
	}

	select {
	case msg := <-c3.send:
		if msg != "EVT ROOM PRESENCE ENTER alice" {
			t.Fatalf("expected ENTER evt in roomB, got %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ENTER evt in roomB")
	}
}

func TestRouteGameHandlerHook(t *testing.T) {
	h := NewHub()
	go h.Run()

	c := newTestClient("alice")
	c.roomID = "room1"
	h.Register(c)

	GameHandler = func(player, room string, fields []string) (string, string) {
		return "OK custom_handled", "room2"
	}
	defer func() {
		GameHandler = nil
	}()

	Route(h, c, "CUSTOM_CMD")
	resp := <-c.send
	if resp != "OK custom_handled" {
		t.Fatalf("expected OK custom_handled, got %q", resp)
	}
	if c.roomID != "room2" {
		t.Fatalf("expected c.roomID to update to room2, got %q", c.roomID)
	}
}

func TestRouteFloodReturnsRateLimited(t *testing.T) {
	h := NewHub()
	c := newTestClient("alice")
	h.Register(c)

	for i := 0; i < maxCmdsPerSec; i++ {
		Route(h, c, "LOOK")
		resp := <-c.send
		if resp != "OK" {
			t.Fatalf("expected OK at iteration %d, got %q", i, resp)
		}
	}

	Route(h, c, "LOOK")
	resp := <-c.send
	if resp != "ERR 429 RATE_LIMITED" {
		t.Fatalf("expected ERR 429 RATE_LIMITED on flood, got %q", resp)
	}
}
