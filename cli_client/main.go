package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

const (
	Green = "\033[32m"
	Red   = "\033[31m"
	Cyan  = "\033[36m"
	Reset = "\033[0m"
)

func readMessages(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "EVT") {
			fmt.Println("\n" + Green + line + Reset)
		} else if strings.HasPrefix(line, "ERR") {
			fmt.Println(Red + line + Reset)
		} else if strings.HasPrefix(line, "OK") {
			fmt.Println(Cyan + line + Reset)
		} else {
			fmt.Println(line)
		}
		fmt.Print("> ")
	}
	fmt.Println("\nServer exited")
	os.Exit(0)
}

func readInput(conn net.Conn) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		fmt.Fprintln(conn, scanner.Text())
	}
}

func main() {
	args := os.Args
	addr := "localhost:8080"
	if len(args) > 1 {
		addr = args[1]
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println("Couldn't connect to ", addr, "...")
		fmt.Println("Exiting...")
		os.Exit(1)
	}
	defer conn.Close()
	go readMessages(conn)
	readInput(conn)
}
