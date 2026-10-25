package main

import (
	"strings"
	"time"

	"tap/server/logger"
)

const maxCmdsPerSec = 20

func Route(h *Hub, c *Client, line string) {
	if !rateAllow(c) {
		c.Enqueue("ERR 429 RATE_LIMITED")
		logger.Log.Warn("abuse_cmd_flood", "player", c.name, "ip", c.conn.RemoteAddr())
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
		c.conn.Close()
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
	logger.Log.Info("player_connected", "player", name, "ip", c.conn.RemoteAddr())
}

func rateAllow(c *Client) bool {
	if time.Since(c.cmdWindow) > time.Second {
		c.cmdCount = 0
		c.cmdWindow = time.Now()
	}
	c.cmdCount++
	return c.cmdCount <= maxCmdsPerSec
}
