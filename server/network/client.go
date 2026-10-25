package main

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

type Client struct {
	conn      net.Conn
	send      chan string
	name      string
	roomID    string
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
	for msg := range c.send {
		fmt.Fprintf(c.conn, "%s\n", msg)
	}
}

func (c *Client) ReadLoop(h *Hub) {
	defer func() {
		h.Unregister(c)
		c.conn.Close()
	}()

	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		Route(h, c, scanner.Text())
	}
}

func (c *Client) Enqueue(msg string) bool {
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}
