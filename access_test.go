package log

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func decodeLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	sc := bufio.NewScanner(bytes.NewReader(buf.Bytes()))
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("bad json line %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

func byMsg(recs []map[string]any, msg string) []map[string]any {
	var out []map[string]any
	for _, r := range recs {
		if r["msg"] == msg {
			out = append(out, r)
		}
	}
	return out
}

func pair(t *testing.T, buf *bytes.Buffer) (start, end map[string]any) {
	t.Helper()
	recs := decodeLines(t, buf)
	s, e := byMsg(recs, MsgAPIStart), byMsg(recs, MsgAPIEnd)
	if len(s) != 1 || len(e) != 1 {
		t.Fatalf("want exactly one api-start and one api-end, got %d/%d:\n%s", len(s), len(e), buf.String())
	}
	if s[0]["trace"] == nil || s[0]["trace"] != e[0]["trace"] {
		t.Fatalf("pair must share trace: %v vs %v", s[0]["trace"], e[0]["trace"])
	}
	if s[0]["level"] != "info" || e[0]["level"] != "info" {
		t.Fatalf("levels %v/%v", s[0]["level"], e[0]["level"])
	}
	return s[0], e[0]
}

func newEngine(buf *bytes.Buffer, cfg AccessLogConfig) *gin.Engine {
	Init("info", buf)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(TraceMiddleware())
	r.Use(AccessLogger(cfg))
	r.Use(Recovery())
	return r
}

func TestAccessLogPairWithBodiesAndRedaction(t *testing.T) {
	var buf bytes.Buffer
	viper.Set("server.name", "order-api")
	t.Cleanup(func() { viper.Set("server.name", "") })
	okCfg := DefaultAccessLogConfig()
	okCfg.ResponseBodyOnSuccess = true
	r := newEngine(&buf, okCfg)
	r.POST("/v1/hlj/order/:id", func(c *gin.Context) {
		var in map[string]any
		if err := c.ShouldBindJSON(&in); err != nil {
			t.Errorf("handler must still read full body: %v", err)
		}
		if in["password"] != "p@ss" {
			t.Errorf("handler body altered: %v", in)
		}
		// simulate JWT middleware writing the merchant later in the chain
		c.Set(verifiedUserIDKey, "u-1")
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"id": "o-1", "access_token": "tok"}})
	})

	body := `{"name":"latte","password":"p@ss","nested":{"appSecret":"s","list":[{"newPassword":"x"}]},"code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/hlj/order/9?page=1&token=abc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-Request-ID", "given-id")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	start, end := pair(t, &buf)
	if start["trace"] != "given-id" || start["server"] != "order-api" {
		t.Fatalf("start=%v", start)
	}
	if start["route"] != "/v1/hlj/order/:id" || start["path"] != "/v1/hlj/order/9" || start["method"] != "POST" {
		t.Fatalf("start route fields %v", start)
	}
	q, _ := start["query"].(map[string]any)
	if q["page"] != "1" || q["token"] != RedactedValue {
		t.Fatalf("query=%v", start["query"])
	}
	rb, _ := start["req_body"].(map[string]any)
	if rb["name"] != "latte" || rb["password"] != RedactedValue || rb["code"] != RedactedValue {
		t.Fatalf("req_body=%v", start["req_body"])
	}
	nested := rb["nested"].(map[string]any)
	if nested["appSecret"] != RedactedValue || nested["list"].([]any)[0].(map[string]any)["newPassword"] != RedactedValue {
		t.Fatalf("nested redaction %v", nested)
	}
	if strings.Contains(buf.String(), "p@ss") || strings.Contains(buf.String(), `"tok"`) || strings.Contains(buf.String(), "abc") {
		t.Fatalf("secret leaked: %s", buf.String())
	}
	if end["status"] != float64(200) || end["user_id"] != "u-1" {
		t.Fatalf("end=%v", end)
	}
	if _, ok := end["latency_ms"].(float64); !ok {
		t.Fatalf("latency_ms=%v", end["latency_ms"])
	}
	resp, _ := end["resp_body"].(map[string]any)
	if resp["code"] != float64(0) {
		t.Fatalf("response code must stay visible: %v", end["resp_body"])
	}
	if resp["data"].(map[string]any)["access_token"] != RedactedValue {
		t.Fatalf("resp token not redacted: %v", resp)
	}
	for _, k := range []string{"stacktrace", "caller", "_skip_stack", "_skip_caller"} {
		if _, ok := end[k]; ok {
			t.Fatalf("api-end must not carry %q: %v", k, end)
		}
	}
	// key order: time, level, server, trace, msg first
	line := strings.SplitN(buf.String(), "\n", 2)[0]
	idx := func(k string) int { return strings.Index(line, `"`+k+`":`) }
	order := []int{idx("time"), idx("level"), idx("server"), idx("trace"), idx("msg"), idx("method")}
	inOrder := order[0] == 1
	for i := 1; i < len(order); i++ {
		inOrder = inOrder && order[i-1] < order[i]
	}
	if !inOrder {
		t.Fatalf("unexpected key order: %s", line)
	}
}

func TestAccessLogPairOnPanic(t *testing.T) {
	var buf bytes.Buffer
	r := newEngine(&buf, DefaultAccessLogConfig())
	r.GET("/boom", func(c *gin.Context) { panic("kaboom") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d", w.Code)
	}
	_, end := pair(t, &buf)
	if end["status"] != float64(500) || end["panic"] != "kaboom" {
		t.Fatalf("end=%v", end)
	}
	recs := byMsg(decodeLines(t, &buf), "panic recovered")
	if len(recs) != 1 || recs[0]["level"] != "error" {
		t.Fatalf("panic line missing: %s", buf.String())
	}
	if st, _ := recs[0]["stacktrace"].(string); !strings.Contains(st, "TestAccessLogPairOnPanic") || strings.Contains(st, "gin-gonic") {
		t.Fatalf("stacktrace=%q", st)
	}
}

func TestAccessLogPairOnPanicWithoutRecovery(t *testing.T) {
	var buf bytes.Buffer
	Init("info", &buf)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessLogger(DefaultAccessLogConfig()))
	r.GET("/boom", func(c *gin.Context) { panic("raw") })
	func() {
		defer func() {
			if rec := recover(); rec != "raw" {
				t.Fatalf("panic must propagate, got %v", rec)
			}
		}()
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))
	}()
	recs := decodeLines(t, &buf)
	if len(byMsg(recs, MsgAPIStart)) != 1 || len(byMsg(recs, MsgAPIEnd)) != 1 {
		t.Fatalf("pair broken: %s", buf.String())
	}
	end := byMsg(recs, MsgAPIEnd)[0]
	if end["status"] != float64(500) || end["panic"] != "raw" {
		t.Fatalf("end=%v", end)
	}
}

func TestAccessLogPairOnAbort(t *testing.T) {
	var buf bytes.Buffer
	r := newEngine(&buf, DefaultAccessLogConfig())
	called := false
	r.GET("/v1/secure", func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }, func(c *gin.Context) { called = true })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/secure", nil))
	if called {
		t.Fatal("abort ignored")
	}
	_, end := pair(t, &buf)
	if end["status"] != float64(401) || end["aborted"] != true {
		t.Fatalf("end=%v", end)
	}
}

func TestAccessLogStatusLevel(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultAccessLogConfig()
	cfg.StatusLevel = true
	r := newEngine(&buf, cfg)
	r.GET("/e", func(c *gin.Context) { c.Status(http.StatusBadGateway) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/e", nil))
	end := byMsg(decodeLines(t, &buf), MsgAPIEnd)
	if len(end) != 1 || end[0]["level"] != "error" {
		t.Fatalf("end=%v", end)
	}
}

func TestAccessLogTruncationAndBinary(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultAccessLogConfig()
	cfg.MaxBodyBytes = 64
	cfg.ResponseBodyOnSuccess = true
	r := newEngine(&buf, cfg)
	var got int
	r.POST("/big", func(c *gin.Context) {
		b, _ := c.GetRawData()
		got = len(b)
		c.JSON(http.StatusOK, gin.H{"blob": strings.Repeat("y", 200), "token": "zzz"})
	})
	r.POST("/upload", func(c *gin.Context) {
		c.Data(http.StatusOK, "image/png", []byte{0x89, 'P', 'N', 'G'})
	})

	big := `{"password":"hunter2","blob":"` + strings.Repeat("x", 500) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/big", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)
	if got != len(big) {
		t.Fatalf("handler saw %d bytes, want %d", got, len(big))
	}
	start, end := pair(t, &buf)
	if start["req_body_truncated"] != true || end["resp_body_truncated"] != true {
		t.Fatalf("truncation flags missing: %v / %v", start, end)
	}
	if s, _ := start["req_body"].(string); len(s) > 64 || strings.Contains(s, "hunter2") {
		t.Fatalf("req_body=%q", s)
	}
	if strings.Contains(buf.String(), "hunter2") {
		t.Fatalf("secret leaked in truncated body: %s", buf.String())
	}

	buf.Reset()
	req = httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("--x\r\n"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	r.ServeHTTP(httptest.NewRecorder(), req)
	start, end = pair(t, &buf)
	if start["req_body"] != "[omitted multipart/form-data]" || end["resp_body"] != "[omitted image/png]" {
		t.Fatalf("binary bodies should be omitted: %v / %v", start["req_body"], end["resp_body"])
	}
}

func TestAccessLogFormBody(t *testing.T) {
	var buf bytes.Buffer
	r := newEngine(&buf, DefaultAccessLogConfig())
	r.POST("/login", func(c *gin.Context) {
		if c.PostForm("pwd") != "secret1" {
			t.Errorf("form not readable")
		}
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("user=a&pwd=secret1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(httptest.NewRecorder(), req)
	start, _ := pair(t, &buf)
	rb, _ := start["req_body"].(map[string]any)
	if rb["user"] != "a" || rb["pwd"] != RedactedValue {
		t.Fatalf("req_body=%v", start["req_body"])
	}
}

func TestAccessLogSkipPaths(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultAccessLogConfig()
	cfg.SkipPaths = append(cfg.SkipPaths, "/internal/*")
	r := newEngine(&buf, cfg)
	ok := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	for _, p := range []string{"/healthz", "/health", "/ping", "/v1/order/metrics", "/internal/x"} {
		r.GET(p, ok)
	}
	r.OPTIONS("/v1/a", ok)
	for _, p := range []string{"/healthz", "/health", "/ping", "/v1/order/metrics", "/internal/x"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, p, nil))
	}
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodOptions, "/v1/a", nil))
	if buf.Len() != 0 {
		t.Fatalf("expected no access log, got %s", buf.String())
	}
}

func TestAccessLogDisabledAndViper(t *testing.T) {
	viper.Set("log.access.enabled", false)
	t.Cleanup(func() { viper.Set("log.access.enabled", nil) })
	var buf bytes.Buffer
	Init("info", &buf)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/x", func(c *gin.Context) { c.Status(200) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	if buf.Len() != 0 {
		t.Fatalf("disabled access log wrote %s", buf.String())
	}
}

func TestAccessLogWithoutTraceMiddlewareStillPairs(t *testing.T) {
	var buf bytes.Buffer
	Init("info", &buf)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/x", func(c *gin.Context) {
		if TraceID(c.Request.Context()) == "" {
			t.Error("trace not injected")
		}
		c.Status(200)
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	pair(t, &buf)
}

func TestRedactJSONText(t *testing.T) {
	in := `{"user":"a","password":"p1","nested":{"api_key":"k","code":0},"tok`
	got := redactJSONText(in, false)
	if strings.Contains(got, "p1") || strings.Contains(got, `"k"`) || !strings.Contains(got, `"code":0`) {
		t.Fatalf("got %s", got)
	}
}

func TestIsSensitiveKey(t *testing.T) {
	for _, k := range []string{"password", "newPassword", "app_secret", "Authorization", "X-Api-Key", "privateKey", "id_card", "cardNo", "otp", "refresh_token"} {
		if !IsSensitiveKey(k, false) {
			t.Errorf("%s should be sensitive", k)
		}
	}
	for _, k := range []string{"name", "code", "merchantKey", "token_type", "status"} {
		if IsSensitiveKey(k, false) {
			t.Errorf("%s should not be sensitive in responses", k)
		}
	}
	if !IsSensitiveKey("code", true) {
		t.Error("request code should be sensitive")
	}
	AddSensitiveKeys("wxOpenId")
	if !IsSensitiveKey("wx_open_id", false) {
		t.Error("extra key not applied")
	}
}

func TestAccessLogSuccessBodyOmittedByDefaultErrorsKeepBody(t *testing.T) {
	var buf bytes.Buffer
	r := newEngine(&buf, DefaultAccessLogConfig())
	r.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": strings.Repeat("z", 100)})
	})
	r.GET("/bad", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "bad"})
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))
	_, end := pair(t, &buf)
	if _, ok := end["resp_body"]; ok {
		t.Fatalf("2xx body must be omitted by default: %v", end)
	}
	if end["resp_size"] == nil || end["status"] != float64(200) {
		t.Fatalf("size and status must stay: %v", end)
	}

	buf.Reset()
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/bad", nil))
	_, end = pair(t, &buf)
	rb, _ := end["resp_body"].(map[string]any)
	if rb["msg"] != "bad" {
		t.Fatalf("error body must be kept: %v", end)
	}
}
