package main

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"time"
)

type Client struct {
	conn      net.Conn
	send      chan string
	name      string
	roomID    string
	cmdCount  int
	cmdWindow time.Time
	closeOnce sync.Once
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn:      conn,
		send:      make(chan string, 128),
		cmdWindow: time.Now(),
	}
}

func (c *Client) WriteLoop() {
	defer c.conn.Close()
	for msg := range c.send {
		fmt.Fprintf(c.conn, "%s\n", msg)
	}
}

func (c *Client) ReadLoop(h *Hub) {
	defer func() {
		h.Unregister(c)
		c.conn.Close()
		c.Close()
	}()

	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		Route(h, c, scanner.Text())
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.send)
	})
}

func (c *Client) Enqueue(msg string) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}
