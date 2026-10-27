package main

import (
	"fmt"
	"net"
	"time"

	"tap/server/logger"
)

func main() {
	logger.Init()

	hub := NewHub()
	go hub.Run()

	tracker := NewReconnectTracker(5, 10*time.Second)

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
		rawAddr := conn.RemoteAddr().String()
		ip := extractIP(rawAddr)
		if abused, count := tracker.Record(ip); abused {
			logger.Log.Warn("abuse_rapid_reconnect", "ip", ip, "attempts", count)
		}
		logger.Log.Info("connection_accepted", "ip", rawAddr)

		c := NewClient(conn)
		fmt.Fprintf(conn, "OK hello proto=1\n")

		go c.ReadLoop(hub)
		go c.WriteLoop()
	}
}
