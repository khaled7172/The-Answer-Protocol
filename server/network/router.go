package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tap/server/logger"
)

const maxCmdsPerSec = 20

var GameHandler func(playerName, roomID string, fields []string) (string, string)

type whoResponse struct {
	Room   []string `json:"room"`
	Server int      `json:"server"`
}

func Route(h *Hub, c *Client, line string) {
	if !rateAllow(c) {
		c.Enqueue("ERR 429 RATE_LIMITED")
		logger.Log.Warn("abuse_cmd_flood", "player", c.name, "ip", clientRemoteAddr(c), "count_per_sec", c.cmdCount)
		return
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return
	}
	cmd := strings.ToUpper(fields[0])

	logger.Log.Debug("command_received", "player", c.name, "cmd", cmd)

	if cmd == "CONNECT" {
		handleConnect(h, c, fields)
		return
	}

	if c.name == "" {
		c.Enqueue("ERR 100 NOT_CONNECTED")
		return
	}

	if cmd == "QUIT" {
		c.Enqueue("OK bye")
		c.Close()
		return
	}

	if cmd == "CHAT" {
		handleChat(h, c, fields)
		return
	}

	if cmd == "WHO" {
		handleWho(h, c)
		return
	}

	if cmd == "GROUP" {
		handleGroup(h, c, fields)
		return
	}

	if GameHandler != nil {
		resp, newRoomID := GameHandler(c.name, c.roomID, fields)
		if newRoomID != "" && newRoomID != c.roomID {
			if c.roomID != "" {
				h.Broadcast("room", c.roomID, "", fmt.Sprintf("EVT ROOM PRESENCE LEAVE %s", c.name))
			}
			c.roomID = newRoomID
			h.Broadcast("room", c.roomID, "", fmt.Sprintf("EVT ROOM PRESENCE ENTER %s", c.name))
		}
		c.Enqueue(resp)
		return
	}

	if cmd == "MOVE" {
		handleMove(h, c, fields)
		return
	}

	c.Enqueue("OK")
}

func handleConnect(h *Hub, c *Client, fields []string) {
	if len(fields) < 2 {
		c.Enqueue("ERR 100 MISSING_ARG")
		return
	}
	name := fields[1]
	if h.NameTaken(name) {
		c.Enqueue("ERR 201 NAME_IN_USE")
		return
	}
	c.name = name
	h.Register(c)
	c.Enqueue("OK connected")
	logger.Log.Info("player_connected", "player", name, "ip", clientRemoteAddr(c))
}

func handleChat(h *Hub, c *Client, fields []string) {
	if len(fields) < 3 {
		c.Enqueue("ERR 100 MISSING_ARG")
		return
	}
	scope := strings.ToUpper(fields[1])
	msg := strings.Join(fields[2:], " ")
	switch scope {
	case "GLOBAL":
		h.Broadcast("global", "", "", fmt.Sprintf("EVT GLOBAL CHAT %s %s", c.name, msg))
		c.Enqueue("OK")
	case "ROOM":
		if c.roomID == "" {
			c.Enqueue("ERR 100 NOT_IN_ROOM")
			return
		}
		h.Broadcast("room", c.roomID, "", fmt.Sprintf("EVT ROOM CHAT %s %s", c.name, msg))
		c.Enqueue("OK")
	default:
		c.Enqueue("ERR 100 INVALID_SCOPE")
	}
}

func handleWho(h *Hub, c *Client) {
	roomList := h.RoomPlayers(c.roomID)
	if roomList == nil {
		roomList = []string{}
	}
	data, _ := json.Marshal(whoResponse{
		Room:   roomList,
		Server: h.PlayerCount(),
	})
	c.Enqueue(fmt.Sprintf("OK %s", string(data)))
}

func handleGroup(h *Hub, c *Client, fields []string) {
	if len(fields) < 3 {
		c.Enqueue("ERR 100 MISSING_ARG")
		return
	}
	action := strings.ToUpper(fields[1])
	target := fields[2]
	if action == "INVITE" {
		if !h.NameTaken(target) {
			c.Enqueue("ERR 401 NOT_FOUND")
			return
		}
		h.Broadcast("direct", "", target, fmt.Sprintf("EVT GROUP INVITE %s", c.name))
		c.Enqueue("OK")
		return
	}
	c.Enqueue("ERR 100 UNKNOWN_ACTION")
}

func handleMove(h *Hub, c *Client, fields []string) {
	if len(fields) < 2 {
		c.Enqueue("ERR 100 MISSING_ARG")
		return
	}
	newRoom := fields[1]
	if c.roomID != "" && c.roomID != newRoom {
		h.Broadcast("room", c.roomID, "", fmt.Sprintf("EVT ROOM PRESENCE LEAVE %s", c.name))
	}
	c.roomID = newRoom
	h.Broadcast("room", c.roomID, "", fmt.Sprintf("EVT ROOM PRESENCE ENTER %s", c.name))
	c.Enqueue(fmt.Sprintf("OK room=%s", newRoom))
}

func rateAllow(c *Client) bool {
	if time.Since(c.cmdWindow) > time.Second {
		c.cmdCount = 0
		c.cmdWindow = time.Now()
	}
	c.cmdCount++
	return c.cmdCount <= maxCmdsPerSec
}

func clientRemoteAddr(c *Client) string {
	if c.conn != nil && c.conn.RemoteAddr() != nil {
		return c.conn.RemoteAddr().String()
	}
	return ""
}
