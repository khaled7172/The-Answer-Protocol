package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestComputeAcceptKey(t *testing.T) {
	sampleKey := "dGhlIHNhbXBsZSBub25jZQ=="
	expected := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	got := computeAcceptKey(sampleKey)
	if got != expected {
		t.Fatalf("expected accept key %s, got %s", expected, got)
	}
}

func TestEncodeTextFrameShort(t *testing.T) {
	payload := []byte("hello")
	frame := encodeTextFrame(payload)
	if len(frame) != 7 {
		t.Fatalf("expected frame len 7, got %d", len(frame))
	}
	if frame[0] != 0x81 || frame[1] != 5 {
		t.Fatalf("unexpected header: %v", frame[:2])
	}
	if string(frame[2:]) != "hello" {
		t.Fatalf("unexpected payload: %s", string(frame[2:]))
	}
}

func TestEncodeTextFrameMedium(t *testing.T) {
	payload := []byte(strings.Repeat("A", 200))
	frame := encodeTextFrame(payload)
	if len(frame) != 204 {
		t.Fatalf("expected frame len 204, got %d", len(frame))
	}
	if frame[0] != 0x81 || frame[1] != 126 {
		t.Fatalf("unexpected header: %v", frame[:2])
	}
}

func TestDecodeTextFrameUnmasked(t *testing.T) {
	payload := []byte("test message")
	frame := encodeTextFrame(payload)
	r := bytes.NewReader(frame)
	got, err := decodeTextFrame(r)
	if err != nil {
		t.Fatalf("failed to decode frame: %v", err)
	}
	if got != "test message" {
		t.Fatalf("expected 'test message', got %s", got)
	}
}

func TestDecodeTextFrameMasked(t *testing.T) {
	maskKey := []byte{0x37, 0xfa, 0x21, 0x3d}
	payload := []byte("Hello")
	maskedPayload := make([]byte, len(payload))
	for i := 0; i < len(payload); i++ {
		maskedPayload[i] = payload[i] ^ maskKey[i%4]
	}

	frame := []byte{0x81, byte(0x80 | len(payload))}
	frame = append(frame, maskKey...)
	frame = append(frame, maskedPayload...)

	r := bytes.NewReader(frame)
	got, err := decodeTextFrame(r)
	if err != nil {
		t.Fatalf("failed to decode masked frame: %v", err)
	}
	if got != "Hello" {
		t.Fatalf("expected 'Hello', got %s", got)
	}
}

func TestDecodeCloseFrameReturnsEOF(t *testing.T) {
	closeFrame := []byte{0x88, 0x00}
	r := bytes.NewReader(closeFrame)
	_, err := decodeTextFrame(r)
	if err != io.EOF {
		t.Fatalf("expected io.EOF on close frame, got %v", err)
	}
}
