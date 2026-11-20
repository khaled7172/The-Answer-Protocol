package main

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"tap/server/game"
	"tap/server/logger"
)

const maxCmdsPerSec = 20

func Route(h *Hub, c *Client, line string) bool {
	if !rateAllow(c) {
		c.Enqueue("ERR 429 RATE_LIMITED")
		logger.Log.Warn("abuse_cmd_flood", "player", c.name, "ip", c.addr, "count_per_sec", c.cmdCount)
		return true
	}
	if !utf8.ValidString(line) || strings.ContainsFunc(line, unicode.IsControl) {
		c.Enqueue("ERR 100 INVALID_CHARACTERS")
		logger.Log.Warn("invalid_input", "player", c.name, "ip", c.addr, "line", line)
		return true
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return true
	}
	cmd := strings.ToUpper(fields[0])
	logger.Log.Info("command_received", "player", c.name, "cmd", cmd, "args", strings.Join(fields[1:], " "))

	switch {
	case cmd == "QUIT":
		c.Enqueue("OK bye")
		return false
	case cmd == "CONNECT":
		handleConnect(h, c, fields)
	case c.name == "":
		c.Enqueue("ERR 100 NOT_CONNECTED")
	case cmd == "CHAT":
		handleChat(h, c, fields)
	case cmd == "WHO":
		c.Enqueue(fmt.Sprintf("OK players=%d", h.PlayerCount()))
	case cmd == "GROUP":
		handleGroup(h, c, fields)
	default:
		resp, events := h.game.HandleCommand(c.name, fields)
		c.Enqueue(resp)
		h.Send(events)
	}
	return true
}

func handleConnect(h *Hub, c *Client, fields []string) {
	switch {
	case len(fields) < 2:
		c.Enqueue(game.ErrMissingArg)
	case c.name != "":
		c.Enqueue("ERR 100 ALREADY_CONNECTED")
	case !h.Register(c, fields[1]):
		c.Enqueue("ERR 201 NAME_IN_USE")
	default:
		c.Enqueue("OK connected")
		logger.Log.Info("player_connected", "player", c.name, "ip", c.addr)
		h.Send(h.game.AddPlayer(c.name))
		h.SendStats()
	}
}

func handleChat(h *Hub, c *Client, fields []string) {
	if len(fields) < 3 {
		c.Enqueue(game.ErrMissingArg)
		return
	}
	scope := strings.ToUpper(fields[1])
	evt := fmt.Sprintf("EVT %s CHAT %s %s", scope, c.name, strings.Join(fields[2:], " "))
	switch {
	case scope == "GLOBAL":
		c.Enqueue("OK")
		h.SendAll(evt)
	case scope == "ROOM":
		c.Enqueue("OK")
		h.Send([]game.Event{{To: h.game.RoomMates(c.name), Msg: evt}})
	case scope == "GROUP" && c.groupID == "":
		c.Enqueue("ERR 401 NOT_IN_GROUP")
	case scope == "GROUP":
		c.Enqueue("OK")
		h.SendGroup(c.groupID, evt)
	default:
		c.Enqueue("ERR 100 INVALID_SCOPE")
	}
}

func handleGroup(h *Hub, c *Client, fields []string) {
	action := ""
	if len(fields) > 1 {
		action = strings.ToUpper(fields[1])
	}
	switch {
	case (action == "INVITE" || action == "JOIN") && len(fields) < 3:
		c.Enqueue(game.ErrMissingArg)
		return
	case (action == "INVITE" || action == "LEAVE") && c.groupID == "":
		c.Enqueue("ERR 401 NOT_IN_GROUP")
		return
	case (action == "CREATE" || action == "JOIN") && c.groupID != "":
		c.Enqueue("ERR 402 ALREADY_IN_GROUP")
		return
	}
	switch action {
	case "CREATE":
		c.Enqueue("OK group=" + h.NewGroup(c))
	case "INVITE":
		if _, ok := h.GroupOf(fields[2]); !ok {
			c.Enqueue("ERR 404 PLAYER_NOT_FOUND")
			return
		}
		c.Enqueue("OK")
		h.Send([]game.Event{{To: []string{fields[2]}, Msg: "EVT GROUP INVITE " + c.name}})
	case "JOIN":
		group, _ := h.GroupOf(fields[2])
		if group == "" {
			c.Enqueue("ERR 404 GROUP_NOT_FOUND")
			return
		}
		h.SetGroup(c, group)
		c.Enqueue("OK group=" + group)
		h.SendGroup(group, "EVT GROUP JOIN "+c.name)
	case "LEAVE":
		group := c.groupID
		h.SetGroup(c, "")
		c.Enqueue("OK")
		h.SendGroup(group, "EVT GROUP LEAVE "+c.name)
	default:
		c.Enqueue("ERR 100 UNKNOWN_ACTION")
	}
}

func rateAllow(c *Client) bool {
	if time.Since(c.cmdWindow) > time.Second {
		c.cmdCount = 0
		c.cmdWindow = time.Now()
	}
	c.cmdCount++
	return c.cmdCount <= maxCmdsPerSec
}
