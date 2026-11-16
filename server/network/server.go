package main

import (
	"net"
	"os"
	"time"

	"tap/server/game"
	"tap/server/logger"
)

func main() {
	logger.Init()

	world, err := game.LoadWorld("world_data/world.yaml")
	if err == nil {
		err = game.ValidateWorld(world)
	}
	if err != nil {
		logger.Log.Error("world_load_failed", "err", err)
		os.Exit(1)
	}
	hub := NewHub(game.NewGame(world))
	tracker := NewReconnectTracker(5, 10*time.Second)

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		logger.Log.Error("listen_failed", "err", err)
		os.Exit(1)
	}
	logger.Log.Info("server_started", "port", 8080)

	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Log.Warn("accept_error", "err", err)
			continue
		}
		c := NewClient(conn)
		ip := extractIP(c.addr)
		if abused, count := tracker.Record(ip); abused {
			logger.Log.Warn("abuse_rapid_reconnect", "ip", ip, "attempts", count)
		}
		logger.Log.Info("connection_accepted", "ip", c.addr)
		c.Enqueue("OK hello proto=1")
		go c.ReadLoop(hub)
		go c.WriteLoop()
	}
}
