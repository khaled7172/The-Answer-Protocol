package main

import (
	"fmt"
	"sync"

	"tap/server/game"
	"tap/server/logger"
)

type Hub struct {
	mu      sync.RWMutex
	game    *game.Game
	clients map[string]*Client
}

func NewHub(g *game.Game) *Hub {
	return &Hub{game: g, clients: make(map[string]*Client)}
}

func (h *Hub) Register(c *Client, name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, taken := h.clients[name]; taken {
		return false
	}
	c.name = name
	h.clients[name] = c
	return true
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	delete(h.clients, c.name)
	group := c.groupID
	h.mu.Unlock()

	h.Send(h.game.RemovePlayer(c.name))
	if group != "" {
		h.SendGroup(group, "EVT GROUP LEAVE "+c.name)
	}
	h.SendStats()
}

func (h *Hub) SetGroup(c *Client, group string) {
	h.mu.Lock()
	c.groupID = group
	h.mu.Unlock()
}

func (h *Hub) GroupOf(name string) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.clients[name]
	if !ok {
		return "", false
	}
	return c.groupID, true
}

func (h *Hub) PlayerCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) Send(events []game.Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, e := range events {
		logger.Log.Info("event_broadcast", "to", e.To, "payload", e.Msg)
		for _, name := range e.To {
			if c, ok := h.clients[name]; ok {
				c.Enqueue(e.Msg)
			}
		}
	}
}

func (h *Hub) SendAll(msg string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	logger.Log.Info("event_broadcast", "to", "all", "payload", msg)
	for _, c := range h.clients {
		c.Enqueue(msg)
	}
}

func (h *Hub) SendGroup(group, msg string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	logger.Log.Info("event_broadcast", "to", group, "payload", msg)
	for _, c := range h.clients {
		if c.groupID == group {
			c.Enqueue(msg)
		}
	}
}

func (h *Hub) SendStats() {
	h.SendAll(fmt.Sprintf("EVT STATS players=%d", h.PlayerCount()))
}
