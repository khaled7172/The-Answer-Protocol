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
	send      chan string
	name      string
	roomID    string
	groupID   string
	cmdCount  int
	cmdWindow time.Time
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn:      conn,
		send:      make(chan string, 128),
		cmdWindow: time.Now(),
	}
}

func (c *Client) WriteLoop() {
	defer c.Close()
	for msg := range c.send {
		fmt.Fprintf(c.conn, "%s\n", msg)
	}
}

func (c *Client) ReadLoop(h *Hub) {
	defer func() {
		h.Unregister(c)
		c.Close()
		close(c.send)
	}()

	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		Route(h, c, scanner.Text())
	}
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) RemoteAddr() string {
	if c.conn != nil {
		return c.conn.RemoteAddr().String()
	}
	return ""
}

func (c *Client) Enqueue(msg string) bool {
	logger.Log.Debug("response_sent", "player", c.name, "response", msg)
	if strings.HasPrefix(msg, "ERR ") {
		logger.Log.Warn("error_sent", "player", c.name, "code", msg)
	}
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}
