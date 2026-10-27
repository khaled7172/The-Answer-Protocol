package main

import (
	"fmt"
	"sync"
	"tap/server/logger"
)

type broadcastMsg struct {
	scope   string
	roomID  string
	target  string
	payload string
}

type Hub struct {
	mu        sync.RWMutex
	clients   map[string]*Client
	broadcast chan broadcastMsg
}

func NewHub() *Hub {
	return &Hub{
		clients:   make(map[string]*Client),
		broadcast: make(chan broadcastMsg, 256),
	}
}

func (h *Hub) Run() {
	for msg := range h.broadcast {
		h.dispatch(msg)
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c.name] = c
	h.mu.Unlock()
	logger.Log.Info("player_registered", "player", c.name)
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	_, ok := h.clients[c.name]
	if ok {
		delete(h.clients, c.name)
	}
	h.mu.Unlock()

	if !ok {
		return
	}
	c.Close()
	logger.Log.Info("player_disconnected", "player", c.name)
	if c.roomID != "" {
		h.Broadcast("room", c.roomID, "", fmt.Sprintf("EVT ROOM PRESENCE LEAVE %s", c.name))
	}
}

func (h *Hub) NameTaken(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.clients[name]
	return exists
}

func (h *Hub) PlayerCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) RoomPlayers(roomID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []string
	for _, c := range h.clients {
		if c.roomID == roomID {
			out = append(out, c.name)
		}
	}
	return out
}

func (h *Hub) Broadcast(scope, roomID, target, payload string) {
	h.broadcast <- broadcastMsg{scope, roomID, target, payload}
}

func (h *Hub) dispatch(msg broadcastMsg) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	switch msg.scope {
	case "global":
		for _, c := range h.clients {
			c.Enqueue(msg.payload)
		}
	case "room":
		for _, c := range h.clients {
			if c.roomID == msg.roomID {
				c.Enqueue(msg.payload)
			}
		}
	case "direct":
		if c, ok := h.clients[msg.target]; ok {
			c.Enqueue(msg.payload)
		}
	}
}
