package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"tap/server/logger"
)

type Client struct {
	conn      net.Conn
	addr      string
	send      chan string
	name      string
	groupID   string
	cmdCount  int
	cmdWindow time.Time
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn:      conn,
		addr:      conn.RemoteAddr().String(),
		send:      make(chan string, 128),
		cmdWindow: time.Now(),
	}
}

func (c *Client) WriteLoop() {
	for msg := range c.send {
		fmt.Fprintf(c.conn, "%s\n", msg)
	}
	c.conn.Close()
}

func (c *Client) ReadLoop(h *Hub) {
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() && Route(h, c, scanner.Text()) {
	}
	if c.name != "" {
		h.Unregister(c)
	}
	logger.Log.Info("player_disconnected", "player", c.name, "ip", c.addr)
	close(c.send)
}

func (c *Client) Enqueue(msg string) bool {
	logger.Log.Debug("response_sent", "player", c.name, "response", msg)
	if strings.HasPrefix(msg, "ERR ") {
		logger.Log.Warn("error_sent", "player", c.name, "error", msg)
	}
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}
