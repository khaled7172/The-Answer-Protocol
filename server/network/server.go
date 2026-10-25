package main

import (
	"fmt"
	"net"

	"tap/server/logger"
)

func main() {
	logger.Init()

	hub := NewHub()
	go hub.Run()

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		logger.Log.Error("listen_failed", "err", err)
		return
	}
	logger.Log.Info("server_started", "port", 8080)
	fmt.Println("TAP server listening on :8080")

	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Log.Warn("accept_error", "err", err)
			continue
		}
		logger.Log.Info("connection_accepted", "ip", conn.RemoteAddr())

		c := NewClient(conn)
		fmt.Fprintf(conn, "OK hello proto=1\n")

		go c.ReadLoop(hub)
		go c.WriteLoop()
	}
}
