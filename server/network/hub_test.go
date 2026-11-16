package main

import (
	"slices"
	"testing"

	"tap/server/game"
	"tap/server/logger"
)

func init() {
	logger.Init()
}

func newTestHub() *Hub {
	return NewHub(game.NewGame(&game.World{
		StartingLoc: "square",
		Locations: map[string]game.Room{
			"square": {Name: "Square", Exits: map[string]string{"north": "tavern"}},
			"tavern": {Name: "Tavern", Exits: map[string]string{"south": "square"}},
		},
	}))
}

func connect(t *testing.T, h *Hub, name string) *Client {
	t.Helper()
	c := &Client{send: make(chan string, 128)}
	Route(h, c, "CONNECT "+name)
	if msgs := drain(c); len(msgs) == 0 || msgs[0] != "OK connected" {
		t.Fatalf("connect %s failed: %v", name, msgs)
	}
	return c
}

func drain(c *Client) []string {
	var msgs []string
	for {
		select {
		case m := <-c.send:
			msgs = append(msgs, m)
		default:
			return msgs
		}
	}
}

func TestHubRegisterRejectsDuplicateName(t *testing.T) {
	h := newTestHub()
	if !h.Register(&Client{}, "alice") {
		t.Fatal("first registration refused")
	}
	if h.Register(&Client{}, "alice") {
		t.Fatal("duplicate registration accepted")
	}
	if h.PlayerCount() != 1 {
		t.Fatalf("expected 1 player, got %d", h.PlayerCount())
	}
}

func TestHubUnregisterRemovesStateThenAnnounces(t *testing.T) {
	h := newTestHub()
	alice := connect(t, h, "alice")
	bob := connect(t, h, "bob")
	drain(alice)
	h.SetGroup(alice, "grp_bob")
	h.SetGroup(bob, "grp_bob")

	h.Unregister(alice)

	if _, ok := h.GroupOf("alice"); ok {
		t.Fatal("alice still registered")
	}
	if mates := h.game.RoomMates("bob"); !slices.Equal(mates, []string{"bob"}) {
		t.Fatalf("alice still in game state: %v", mates)
	}
	want := []string{"EVT ROOM PRESENCE LEAVE alice", "EVT GROUP LEAVE alice", "EVT STATS players=1"}
	if got := drain(bob); !slices.Equal(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestHubSendSkipsUnknownNames(t *testing.T) {
	h := newTestHub()
	alice := connect(t, h, "alice")
	h.Send([]game.Event{{To: []string{"ghost", "alice"}, Msg: "EVT TEST"}})
	if got := drain(alice); !slices.Equal(got, []string{"EVT TEST"}) {
		t.Fatalf("expected EVT TEST, got %v", got)
	}
}
