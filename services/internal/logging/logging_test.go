package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, buf)
	}
	buf.Reset()
	return line
}

func TestNew(t *testing.T) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6, 0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36},
		SpanID:  trace.SpanID{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7},
	})
	traced := trace.ContextWithSpanContext(context.Background(), sc)

	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo).With("service", "test")

	logger.InfoContext(traced, "hello", "n", 1)
	line := decode(t, &buf)
	for key, want := range map[string]any{
		"level":    "INFO",
		"msg":      "hello",
		"n":        1.0,
		"service":  "test",
		"trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
		"span_id":  "00f067aa0ba902b7",
	} {
		if line[key] != want {
			t.Errorf("%s = %v, want %v", key, line[key], want)
		}
	}
	if _, ok := line["time"]; !ok {
		t.Error("time missing")
	}

	logger.InfoContext(context.Background(), "untraced")
	line = decode(t, &buf)
	if _, ok := line["trace_id"]; ok {
		t.Errorf("trace_id present without a span context: %v", line)
	}

	logger.DebugContext(traced, "below level")
	if buf.Len() != 0 {
		t.Errorf("debug record logged at info level: %s", buf.String())
	}
}
