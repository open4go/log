package log

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/viper"
)

func TestNewTraceID(t *testing.T) {
	a := NewTraceID()
	b := NewTraceID()
	if a == "" || b == "" {
		t.Fatal("expected non-empty trace id")
	}
	if a == b {
		t.Fatal("expected unique trace ids")
	}
	if len(a) != 32 {
		t.Fatalf("expected 32 hex chars, got %d", len(a))
	}
}

func TestInjectRoundTrip(t *testing.T) {
	ctx := Inject(context.Background(), "trace-1", "1.2.3.4")
	if got := TraceID(ctx); got != "trace-1" {
		t.Fatalf("TraceID=%q", got)
	}
	if got := ClientIP(ctx); got != "1.2.3.4" {
		t.Fatalf("ClientIP=%q", got)
	}
	// legacy string keys must still resolve
	if got, _ := ctx.Value("traceid").(string); got != "trace-1" {
		t.Fatalf("legacy traceid=%q", got)
	}
}

func TestLogIncludesTraceFromLegacyKey(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)
	viper.Set("server.name", "store-api")

	ctx := context.WithValue(context.Background(), "traceid", "legacy-trace") //nolint:staticcheck // SA1029: test covers the legacy plain-string key
	Log(ctx).Info("hello")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, buf.String())
	}
	if rec["trace"] != "legacy-trace" {
		t.Fatalf("trace=%v record=%v", rec["trace"], rec)
	}
	if rec["server"] != "store-api" {
		t.Fatalf("server=%v", rec["server"])
	}
	if rec["msg"] != "hello" {
		t.Fatalf("msg=%v", rec["msg"])
	}
}
