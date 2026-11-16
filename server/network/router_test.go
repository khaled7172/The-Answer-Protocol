package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func expect(t *testing.T, c *Client, want ...string) {
	t.Helper()
	if got := drain(c); !slices.Equal(got, want) {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRouteEmptyLineProducesNoResponse(t *testing.T) {
	c := &Client{send: make(chan string, 1)}
	Route(newTestHub(), c, "   ")
	expect(t, c)
}

func TestRouteConnectAnnouncesToRoomAndServer(t *testing.T) {
	h := newTestHub()
	bob := connect(t, h, "bob")
	connect(t, h, "alice")
	expect(t, bob, "EVT ROOM PRESENCE ENTER alice", "EVT STATS players=2")
}

func TestRouteConnectErrors(t *testing.T) {
	h := newTestHub()
	alice := connect(t, h, "alice")

	c := &Client{send: make(chan string, 8)}
	Route(h, c, "CONNECT")
	Route(h, c, "CONNECT alice")
	Route(h, c, "LOOK")
	expect(t, c, "ERR 100 MISSING_ARG", "ERR 201 NAME_IN_USE", "ERR 100 NOT_CONNECTED")

	Route(h, alice, "CONNECT other")
	expect(t, alice, "ERR 100 ALREADY_CONNECTED")
	if h.PlayerCount() != 1 {
		t.Fatalf("second CONNECT must not register a new name, got %d players", h.PlayerCount())
	}
}

func TestRouteQuitStopsReadingAndSaysBye(t *testing.T) {
	h := newTestHub()
	c := &Client{send: make(chan string, 1)}
	if Route(h, c, "quit") {
		t.Fatal("QUIT should stop the read loop")
	}
	expect(t, c, "OK bye")
}

func TestRouteChatGlobal(t *testing.T) {
	h := newTestHub()
	alice := connect(t, h, "alice")
	bob := connect(t, h, "bob")
	drain(alice)

	Route(h, alice, "CHAT global hello everyone")
	expect(t, alice, "OK", "EVT GLOBAL CHAT alice hello everyone")
	expect(t, bob, "EVT GLOBAL CHAT alice hello everyone")
}

func TestRouteChatRoomOnlyReachesRoommates(t *testing.T) {
	h := newTestHub()
	alice := connect(t, h, "alice")
	bob := connect(t, h, "bob")
	carol := connect(t, h, "carol")
	Route(h, carol, "MOVE north")
	drain(alice)
	drain(bob)
	drain(carol)

	Route(h, alice, "CHAT ROOM watch out")
	expect(t, alice, "OK", "EVT ROOM CHAT alice watch out")
	expect(t, bob, "EVT ROOM CHAT alice watch out")
	expect(t, carol)
}

func TestRouteChatErrors(t *testing.T) {
	h := newTestHub()
	alice := connect(t, h, "alice")
	Route(h, alice, "CHAT")
	Route(h, alice, "CHAT SHOUT hi")
	Route(h, alice, "CHAT GROUP hi")
	expect(t, alice, "ERR 100 MISSING_ARG", "ERR 100 INVALID_SCOPE", "ERR 401 NOT_IN_GROUP")
}

func TestRouteMoveAnnouncesPresence(t *testing.T) {
	h := newTestHub()
	alice := connect(t, h, "alice")
	bob := connect(t, h, "bob")
	drain(alice)

	Route(h, alice, "MOVE north")
	expect(t, alice, "OK room=tavern")
	expect(t, bob, "EVT ROOM PRESENCE LEAVE alice")

	Route(h, bob, "MOVE north")
	expect(t, alice, "EVT ROOM PRESENCE ENTER bob")
}

func TestRouteWho(t *testing.T) {
	h := newTestHub()
	alice := connect(t, h, "alice")
	connect(t, h, "bob")
	carol := connect(t, h, "carol")
	Route(h, carol, "MOVE north")
	drain(alice)

	Route(h, alice, "WHO")
	resp := drain(alice)[0]
	var who whoResponse
	if err := json.Unmarshal([]byte(strings.TrimPrefix(resp, "OK ")), &who); err != nil {
		t.Fatalf("bad WHO json %q: %v", resp, err)
	}
	if who.Server != 3 || !slices.Equal(who.Room, []string{"alice", "bob"}) {
		t.Fatalf("unexpected WHO %+v", who)
	}
}

func TestRouteGroupLifecycle(t *testing.T) {
	h := newTestHub()
	alice := connect(t, h, "alice")
	bob := connect(t, h, "bob")
	drain(alice)

	Route(h, alice, "GROUP INVITE bob")
	Route(h, alice, "GROUP LEAVE")
	Route(h, bob, "GROUP JOIN alice")
	expect(t, alice, "ERR 401 NOT_IN_GROUP", "ERR 401 NOT_IN_GROUP")
	expect(t, bob, "ERR 404 GROUP_NOT_FOUND")

	Route(h, alice, "GROUP CREATE")
	Route(h, alice, "GROUP CREATE")
	Route(h, alice, "GROUP INVITE ghost")
	Route(h, alice, "GROUP INVITE bob")
	expect(t, alice, "OK group=grp_alice", "ERR 402 ALREADY_IN_GROUP", "ERR 404 PLAYER_NOT_FOUND", "OK")
	expect(t, bob, "EVT GROUP INVITE alice")

	Route(h, bob, "GROUP JOIN alice")
	expect(t, bob, "OK group=grp_alice", "EVT GROUP JOIN bob")
	expect(t, alice, "EVT GROUP JOIN bob")

	Route(h, bob, "CHAT GROUP regroup")
	expect(t, bob, "OK", "EVT GROUP CHAT bob regroup")
	expect(t, alice, "EVT GROUP CHAT bob regroup")

	Route(h, bob, "GROUP LEAVE")
	expect(t, bob, "OK")
	expect(t, alice, "EVT GROUP LEAVE bob")
}

func TestRouteGameErrorsReachClient(t *testing.T) {
	h := newTestHub()
	alice := connect(t, h, "alice")
	Route(h, alice, "MOVE west")
	Route(h, alice, "DANCE")
	expect(t, alice, "ERR 301 NO_EXIT", "ERR 100 UNKNOWN_COMMAND")
}

func TestRouteFloodReturnsRateLimited(t *testing.T) {
	h := newTestHub()
	alice := connect(t, h, "alice")
	for range maxCmdsPerSec - 1 {
		Route(h, alice, "STATUS")
	}
	drain(alice)
	Route(h, alice, "STATUS")
	expect(t, alice, "ERR 429 RATE_LIMITED")
}
