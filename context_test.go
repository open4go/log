package log

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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

	ctx := context.WithValue(context.Background(), "traceid", "legacy-trace")
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

func TestTraceAndRequestLogger(t *testing.T) {
	var buf bytes.Buffer
	Init("info", &buf)
	viper.Set("server.name", "store-api")
	viper.Set("server.maxLatency", 10000)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(TraceMiddleware())
	r.Use(RequestLogger())
	r.GET("/v1/ping-biz", func(c *gin.Context) {
		if TraceID(c.Request.Context()) == "" {
			t.Error("missing trace on request context")
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/ping-biz", nil)
	req.Header.Set("X-Request-ID", "given-id")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") != "given-id" {
		t.Fatalf("response X-Request-ID=%q", w.Header().Get("X-Request-ID"))
	}

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, buf.String())
	}
	if rec["trace"] != "given-id" {
		t.Fatalf("access log trace=%v body=%s", rec["trace"], buf.String())
	}
	if rec["path"] != "/v1/ping-biz" {
		t.Fatalf("path=%v", rec["path"])
	}
	if rec["msg"] != "request completed" {
		t.Fatalf("msg=%v", rec["msg"])
	}
}

func TestRequestLoggerSkipsHealthz(t *testing.T) {
	var buf bytes.Buffer
	Init("info", &buf)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if buf.Len() != 0 {
		t.Fatalf("expected no access log for /healthz, got %s", buf.String())
	}
}
