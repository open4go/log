package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
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
	gotCaller, _ := rec["caller"].(string)
	if !strings.HasSuffix(gotCaller, "log_test.go:"+strconv.Itoa(line+1)) {
		t.Fatalf("caller=%q want log_test.go:%d", gotCaller, line+1)
	}
	for _, k := range []string{"file", "func", "line"} {
		if _, ok := rec[k]; ok {
			t.Fatalf("legacy field %q should be gone: %v", k, rec)
		}
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
	gotCaller, _ := rec["caller"].(string)
	if gotCaller != "log/log_test.go:"+strconv.Itoa(line+1) {
		t.Fatalf("caller=%q want log/log_test.go:%d", gotCaller, line+1)
	}
	if rec["error"] != "save failed" {
		t.Fatalf("error field=%v", rec["error"])
	}
	if _, ok := rec["stacktrace"]; ok {
		t.Fatalf("plain error must not carry a stacktrace by default: %v", rec)
	}
}

func TestErrorWithStackIncludesStack(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)

	ErrorWithStack(context.Background(), errors.New("x"))
	rec := decodeLog(t, &buf)
	stack, _ := rec["stacktrace"].(string)
	if !strings.Contains(stack, "TestErrorWithStackIncludesStack") {
		t.Fatalf("stacktrace missing test function:\n%s", stack)
	}
	if strings.Contains(stack, "runtime") {
		t.Fatalf("stacktrace should not include runtime internals:\n%s", stack)
	}
}

func TestLogErrorStackOptIn(t *testing.T) {
	var buf bytes.Buffer
	t.Setenv("LOG_STACKTRACE", "error")
	t.Setenv("LOG_CALLER_FUNC", "true")
	Init("debug", &buf)
	t.Cleanup(func() { _ = os.Unsetenv("LOG_STACKTRACE"); _ = os.Unsetenv("LOG_CALLER_FUNC"); Init("debug", nil) })

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
	if !strings.Contains(stack, "TestLogErrorStackOptIn") {
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
	if c, _ := rec["caller"].(string); !strings.HasPrefix(c, "log/log_test.go:") {
		t.Fatalf("caller=%q", c)
	}
}

func TestStoredEntryReportsWriteSite(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)

	entry := Log(context.Background())
	_, _, line, _ := runtime.Caller(0)
	writeStoredEntry(entry)

	rec := decodeLog(t, &buf)
	if c, _ := rec["caller"].(string); c == "" || c == "log/log_test.go:"+strconv.Itoa(line+1) {
		t.Fatalf("caller=%q should be the write site inside writeStoredEntry", c)
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
	if c, _ := rec["caller"].(string); !strings.HasPrefix(c, "log/log_test.go:") {
		t.Fatalf("caller=%q", c)
	}
}

func TestNilContextDoesNotPanic(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)
	Log(nil).Info("ok") //nolint:staticcheck // SA1012: test intentionally passes nil
	Error(nil, nil)     //nolint:staticcheck // SA1012: test intentionally passes nil
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
	if got := shortFile("/root/go/pkg/mod/github.com/open4go/middle@v0.3.62/jwt.go"); got != "middle/jwt.go" {
		t.Errorf("shortFile(module)=%q", got)
	}
	if got := formatFileLine("/tmp/order.go", 42); got != "tmp/order.go:42" {
		t.Errorf("formatFileLine=%q", got)
	}
}

func TestOrderedFieldsAndNoisyLineShape(t *testing.T) {
	var buf bytes.Buffer
	viper.Set("server.name", "order-api")
	t.Cleanup(func() { viper.Set("server.name", "") })
	Init("debug", &buf)
	ctx := Inject(context.Background(), "t-1", "1.2.3.4")
	Log(ctx).WithError(errors.New("http: named cookie not present")).WithField("zeta", 1).WithField("alpha", 2).Error("Failed")
	line := strings.TrimSpace(buf.String())
	want := []string{`{"time":`, `"level":"error"`, `"server":"order-api"`, `"trace":"t-1"`, `"msg":"Failed"`, `"error":"http: named cookie not present"`, `"caller":"log/log_test.go:`, `"ip":"1.2.3.4"`, `"alpha":2`, `"zeta":1`}
	pos := -1
	for _, w := range want {
		i := strings.Index(line, w)
		if i < 0 || i < pos {
			t.Fatalf("%s out of order/missing in %s", w, line)
		}
		pos = i
	}
	if strings.Contains(line, "stacktrace") {
		t.Fatalf("error line should not carry stacktrace: %s", line)
	}
}

func TestLogLevelEnvOverride(t *testing.T) {
	t.Setenv("LOG_LEVEL", "warn")
	var buf bytes.Buffer
	Init("debug", &buf)
	t.Cleanup(func() { _ = os.Unsetenv("LOG_LEVEL"); Init("debug", nil) })
	Log(context.Background()).Info("hidden")
	if buf.Len() != 0 {
		t.Fatalf("LOG_LEVEL ignored: %s", buf.String())
	}
}
