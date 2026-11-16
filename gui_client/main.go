package main

import (
	"bufio"
	"crypto/sha1"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

//go:embed index.html
var indexHTML []byte

func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func encodeTextFrame(payload []byte) []byte {
	length := len(payload)
	var header []byte
	if length <= 125 {
		header = []byte{0x81, byte(length)}
	} else if length <= 65535 {
		header = make([]byte, 4)
		header[0] = 0x81
		header[1] = 126
		binary.BigEndian.PutUint16(header[2:], uint16(length))
	} else {
		header = make([]byte, 10)
		header[0] = 0x81
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:], uint64(length))
	}
	return append(header, payload...)
}

func decodeTextFrame(r io.Reader) (string, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(r, header); err != nil {
		return "", err
	}

	opcode := header[0] & 0x0f
	if opcode == 0x08 {
		return "", io.EOF
	}

	masked := (header[1] & 0x80) != 0
	length := int(header[1] & 0x7f)

	if length == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return "", err
		}
		length = int(binary.BigEndian.Uint16(ext))
	} else if length == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return "", err
		}
		length = int(binary.BigEndian.Uint64(ext))
	}

	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(r, maskKey); err != nil {
			return "", err
		}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return "", err
	}

	if masked {
		for i := 0; i < length; i++ {
			payload[i] ^= maskKey[i%4]
		}
	}

	return string(payload), nil
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, tcpAddr string) {
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}

	clientConn, buf, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	acceptKey := computeAcceptKey(key)
	handshake := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"
	if _, err := clientConn.Write([]byte(handshake)); err != nil {
		return
	}

	tcpConn, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		clientConn.Write(encodeTextFrame([]byte("ERR 900 CONNECTION_FAILED")))
		return
	}
	defer tcpConn.Close()

	done := make(chan struct{}, 2)

	go func() {
		defer func() { done <- struct{}{} }()
		scanner := bufio.NewScanner(tcpConn)
		for scanner.Scan() {
			line := scanner.Text()
			frame := encodeTextFrame([]byte(line))
			if _, err := clientConn.Write(frame); err != nil {
				return
			}
		}
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			msg, err := decodeTextFrame(buf)
			if err != nil {
				return
			}
			msg = strings.TrimSpace(msg)
			if msg != "" {
				fmt.Fprintf(tcpConn, "%s\n", msg)
			}
		}
	}()

	<-done
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func main() {
	httpPort := flag.String("port", "3000", "HTTP server port")
	tcpTarget := flag.String("server", "localhost:8080", "TAP TCP server target")
	noLaunch := flag.Bool("no-browser", false, "Do not automatically launch web browser")
	flag.Parse()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(w, r, *tcpTarget)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(indexHTML)
	})

	url := fmt.Sprintf("http://localhost:%s", *httpPort)
	fmt.Printf("TAP Web Client listening on %s (bridging to %s)\n", url, *tcpTarget)

	if !*noLaunch {
		go openBrowser(url)
	}

	if err := http.ListenAndServe(":"+*httpPort, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
	}
}
