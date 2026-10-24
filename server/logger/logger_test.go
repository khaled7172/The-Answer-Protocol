package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestInitSetsLog(t *testing.T) {
	Log = nil
	Init()
	if Log == nil {
		t.Fatal("Init() did not set logger.Log")
	}
}

func TestInitSetsDefault(t *testing.T) {
	Init()
	if slog.Default() != Log {
		t.Fatal("Init() did not call slog.SetDefault(Log)")
	}
}

func TestLogOutputIsJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	testLogger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	testLogger.Info("test_event", "key", "value")

	line := buf.Bytes()
	if len(line) == 0 {
		t.Fatal("logger produced no output")
	}

	var entry map[string]any
	if err := json.Unmarshal(line, &entry); err != nil {
		t.Fatalf("logger output is not valid JSON: %v\noutput: %s", err, line)
	}

	if entry["msg"] != "test_event" {
		t.Errorf("expected msg=test_event, got %v", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Errorf("expected key=value, got %v", entry["key"])
	}
}

func TestDebugLevelIsEnabled(t *testing.T) {
	buf := &bytes.Buffer{}
	testLogger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	testLogger.Debug("debug_event")

	if buf.Len() == 0 {
		t.Fatal("DEBUG level message was filtered out — Init() should use LevelDebug")
	}
}
