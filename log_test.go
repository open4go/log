package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func decodeLog(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, buf.String())
	}
	return rec
}

func TestLogCallerIsThisFunctionAndLine(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)

	_, _, line, _ := runtime.Caller(0)
	Log(context.Background()).Info("caller-check")

	rec := decodeLog(t, &buf)
	gotFile, _ := rec["file"].(string)
	gotFunc, _ := rec["func"].(string)
	gotLine, ok := rec["line"].(float64)
	if !ok {
		t.Fatalf("line type %T record=%v", rec["line"], rec)
	}
	if !strings.Contains(gotFile, "log_test.go") {
		t.Fatalf("file=%q want log_test.go", gotFile)
	}
	if int(gotLine) != line+1 {
		t.Fatalf("line=%v want %d file=%q", gotLine, line+1, gotFile)
	}
	if !strings.Contains(gotFunc, "TestLogCallerIsThisFunctionAndLine") {
		t.Fatalf("func=%q", gotFunc)
	}
	if _, hasStack := rec["stacktrace"]; hasStack {
		t.Fatalf("info logs must not include stacktrace: %v", rec)
	}
}

func TestErrorHelperCallerIsNotErrorWrapper(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)

	err := errors.New("save failed")
	_, _, line, _ := runtime.Caller(0)
	Error(context.Background(), err)

	rec := decodeLog(t, &buf)
	gotFunc, _ := rec["func"].(string)
	gotFile, _ := rec["file"].(string)
	gotLine, _ := rec["line"].(float64)
	if gotFunc == "Error" || strings.HasSuffix(gotFunc, ".Error") {
		t.Fatalf("func=%q should be the test, not the Error helper", gotFunc)
	}
	if !strings.Contains(gotFunc, "TestErrorHelperCallerIsNotErrorWrapper") {
		t.Fatalf("func=%q", gotFunc)
	}
	if !strings.Contains(gotFile, "log_test.go") {
		t.Fatalf("file=%q", gotFile)
	}
	if int(gotLine) != line+1 {
		t.Fatalf("line=%v want %d", gotLine, line+1)
	}
	if rec["error"] != "save failed" {
		t.Fatalf("error field=%v", rec["error"])
	}
	stack, _ := rec["stacktrace"].(string)
	if !strings.Contains(stack, "TestErrorHelperCallerIsNotErrorWrapper") {
		t.Fatalf("stacktrace missing test function:\n%s", stack)
	}
	if strings.Contains(stack, "runtime/debug.Stack") {
		t.Fatalf("stacktrace should not include debug.Stack internals:\n%s", stack)
	}
}

func TestLogErrorIncludesStack(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)

	nestedErrorLog(context.Background())

	rec := decodeLog(t, &buf)
	gotFunc, _ := rec["func"].(string)
	if !strings.Contains(gotFunc, "nestedErrorLog") {
		t.Fatalf("func=%q want nestedErrorLog", gotFunc)
	}
	stack, _ := rec["stacktrace"].(string)
	if !strings.Contains(stack, "nestedErrorLog") {
		t.Fatalf("stack missing nestedErrorLog:\n%s", stack)
	}
	if !strings.Contains(stack, "TestLogErrorIncludesStack") {
		t.Fatalf("stack missing test function:\n%s", stack)
	}
}

func nestedErrorLog(ctx context.Context) {
	Log(ctx).Error("boom")
}

func TestGinClosureCallerKeepsParentName(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)

	handler := func() {
		Log(context.Background()).Error("denied")
	}
	handler()

	rec := decodeLog(t, &buf)
	gotFunc, _ := rec["func"].(string)
	if !strings.Contains(gotFunc, "TestGinClosureCallerKeepsParentName") {
		t.Fatalf("func=%q should keep the enclosing function, not just func1", gotFunc)
	}
}

func TestRequestLoggerOmitsStack(t *testing.T) {
	var buf bytes.Buffer
	Init("info", &buf)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/v1/fail", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/fail", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	rec := decodeLog(t, &buf)
	if rec["msg"] != "request completed" {
		t.Fatalf("msg=%v body=%s", rec["msg"], buf.String())
	}
	if rec["status"] != float64(500) {
		t.Fatalf("status=%v", rec["status"])
	}
	if _, ok := rec["_skip_stack"]; ok {
		t.Fatalf("internal skip field leaked: %v", rec)
	}
	if _, ok := rec["stacktrace"]; ok {
		t.Fatalf("access logs must not include stacktrace: %v", rec)
	}
	gotFunc, _ := rec["func"].(string)
	if !strings.Contains(gotFunc, "RequestLogger") {
		t.Fatalf("access log func=%q", gotFunc)
	}
}

func TestStoredEntryReportsWriteSite(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)

	entry := Log(context.Background())
	writeStoredEntry(entry)

	rec := decodeLog(t, &buf)
	gotFunc, _ := rec["func"].(string)
	if !strings.Contains(gotFunc, "writeStoredEntry") {
		t.Fatalf("func=%q should be the write site, not Log() / middleware", gotFunc)
	}
}

func writeStoredEntry(entry *logrus.Entry) {
	entry.Error("later")
}

func TestErrorfCallerAndMessage(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)

	Errorf(context.Background(), errors.New("disk"), "save %s", "order")

	rec := decodeLog(t, &buf)
	if rec["msg"] != "save order" {
		t.Fatalf("msg=%v", rec["msg"])
	}
	if rec["error"] != "disk" {
		t.Fatalf("error=%v", rec["error"])
	}
	gotFunc, _ := rec["func"].(string)
	if !strings.Contains(gotFunc, "TestErrorfCallerAndMessage") {
		t.Fatalf("func=%q", gotFunc)
	}
	if rec["stacktrace"] == nil {
		t.Fatal("expected stacktrace")
	}
}

func TestNilContextDoesNotPanic(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)
	Log(nil).Info("ok")
	Error(nil, nil)
}

func TestHTMLNotEscaped(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)
	Log(context.Background()).Info("a < b & c")
	if !strings.Contains(buf.String(), `"msg":"a < b & c"`) {
		t.Fatalf("html escaped: %s", buf.String())
	}
}

func TestShortFuncAndFile(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"github.com/open4go/rest.MakeError", "rest.MakeError"},
		{"github.com/foo/bar/handler.(*OrderHandler).Create", "handler.(*OrderHandler).Create"},
		{"github.com/open4go/middle.JWTAuthMiddleware.func1", "middle.JWTAuthMiddleware.func1"},
		{"main.main", "main.main"},
	}
	for _, tt := range tests {
		if got := shortFunc(tt.in); got != tt.want {
			t.Errorf("shortFunc(%q)=%q want %q", tt.in, got, tt.want)
		}
	}

	if got := shortFile("/Users/frank/code/handler/order.go"); got != "handler/order.go" {
		t.Errorf("shortFile=%q", got)
	}
	if got := formatFileLine("/tmp/order.go", 42); got != "tmp/order.go:42" {
		t.Errorf("formatFileLine=%q", got)
	}
}
