package logging

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"
)

func TestJSONLoggingAndLevels(t *testing.T) {
	var buf bytes.Buffer
	lg := New(&buf, "json", "info")
	lg.Debug("hidden", map[string]any{"a": 1}) // should be filtered out
	lg.Info("shown", map[string]any{"k": "v"})
	s := buf.String()
	if s == "" {
		t.Fatalf("expected output")
	}
	// Ensure only one line (the info)
	scanner := bufio.NewScanner(bytes.NewReader([]byte(s)))
	count := 0
	var line string
	for scanner.Scan() {
		count++
		line = scanner.Text()
	}
	if count != 1 {
		t.Fatalf("expected 1 line, got %d", count)
	}
	// JSON parseable
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if m["msg"] != "shown" {
		t.Fatalf("unexpected msg: %v", m["msg"])
	}
	if m["lvl"] != "info" {
		t.Fatalf("unexpected level: %v", m["lvl"])
	}
	if m["k"] != "v" {
		t.Fatalf("missing field k=v")
	}
}
