package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Messages of the paired access log lines.
const (
	MsgAPIStart = "api-start"
	MsgAPIEnd   = "api-end"
)

// gin.Context keys used by the access log.
const (
	// PanicContextKey is set by Recovery so api-end can flag the request.
	PanicContextKey = "open4go.log.panic"
	// verifiedUserIDKey mirrors middle.VerifiedUserIDKey (no import cycle).
	verifiedUserIDKey = "OPEN4GO_VERIFIED_USER_ID"
)

// AccessLogConfig controls the api-start / api-end middleware.
type AccessLogConfig struct {
	// Enabled turns the access log on (default true).
	Enabled bool
	// RequestBody / ResponseBody include payloads (default true).
	RequestBody  bool
	ResponseBody bool
	// ResponseBodyOnSuccess also logs the response body of 2xx/3xx replies
	// (default false: success bodies are large and rarely needed; error
	// responses (>=400), panics and slow requests always keep the body).
	ResponseBodyOnSuccess bool
	// MaxBodyBytes caps each logged payload (default 4096). Larger bodies are
	// truncated; the request itself is never modified.
	MaxBodyBytes int
	// SkipPaths are exact paths, or prefixes when they end with "*".
	// Paths ending in /metrics are always skipped.
	SkipPaths []string
	// SkipOptions skips CORS preflight requests (default true).
	SkipOptions bool
	// StatusLevel logs api-end at warn for 4xx and error for 5xx.
	// Default false: both lines are info.
	StatusLevel bool
	// MaxRequestBody caps inbound body via http.MaxBytesReader (0 = 1 MiB default).
	// Upload path prefixes use MaxUploadBody instead (0 = 32 MiB default).
	MaxRequestBody int64
	MaxUploadBody  int64
	// UploadPathPrefixes are path prefixes that use MaxUploadBody (e.g. "/upload").
	UploadPathPrefixes []string
	// SlowThreshold marks api-end with "slow": true (0 = off).
	SlowThreshold time.Duration
}

// DefaultSkipPaths are health probes that would otherwise flood the log.
var DefaultSkipPaths = []string{"/healthz", "/readyz", "/health", "/ping", "/favicon.ico"}

// DefaultAccessLogConfig returns the defaults described on AccessLogConfig.
func DefaultAccessLogConfig() AccessLogConfig {
	return AccessLogConfig{
		Enabled:      true,
		RequestBody:  true,
		ResponseBody: true,
		MaxBodyBytes: 4096,
		SkipPaths:    append([]string(nil), DefaultSkipPaths...),
		SkipOptions:  true,
	}
}

// AccessLogConfigFromViper reads:
//
//	log.access.enabled        bool   (default true)
//	log.access.request_body   bool   (default true)
//	log.access.response_body  bool   (default true)
//	log.access.response_body_on_success bool (default false)
//	log.access.max_body       int    bytes (default 4096)
//	log.access.skip_paths     []string, added to DefaultSkipPaths
//	log.access.skip_options   bool   (default true)
//	log.access.status_level   bool   (default false)
//	log.access.sensitive_keys []string, extra keys to redact
//	server.maxLatency         int    ms, marks slow requests
func AccessLogConfigFromViper() AccessLogConfig {
	cfg := DefaultAccessLogConfig()
	if viper.IsSet("log.access.enabled") {
		cfg.Enabled = viper.GetBool("log.access.enabled")
	}
	if viper.IsSet("log.access.request_body") {
		cfg.RequestBody = viper.GetBool("log.access.request_body")
	}
	if viper.IsSet("log.access.response_body") {
		cfg.ResponseBody = viper.GetBool("log.access.response_body")
	}
	if viper.IsSet("log.access.response_body_on_success") {
		cfg.ResponseBodyOnSuccess = viper.GetBool("log.access.response_body_on_success")
	}
	if n := viper.GetInt("log.access.max_body"); n > 0 {
		cfg.MaxBodyBytes = n
	}
	cfg.SkipPaths = append(cfg.SkipPaths, viper.GetStringSlice("log.access.skip_paths")...)
	if viper.IsSet("log.access.skip_options") {
		cfg.SkipOptions = viper.GetBool("log.access.skip_options")
	}
	cfg.StatusLevel = viper.GetBool("log.access.status_level")
	if ms := viper.GetInt64("server.maxLatency"); ms > 0 {
		cfg.SlowThreshold = time.Duration(ms) * time.Millisecond
	}
	if n := viper.GetInt64("server.max_request_body"); n > 0 {
		cfg.MaxRequestBody = n
	}
	if n := viper.GetInt64("server.max_upload_body"); n > 0 {
		cfg.MaxUploadBody = n
	}
	cfg.UploadPathPrefixes = append(cfg.UploadPathPrefixes, viper.GetStringSlice("server.upload_path_prefixes")...)
	AddSensitiveKeys(viper.GetStringSlice("log.access.sensitive_keys")...)
	return cfg
}

// RequestLogger is the access log middleware configured from viper
// (see AccessLogConfigFromViper). Mount it right after TraceMiddleware and
// before Recovery / auth middleware:
//
//	r.Use(middle.TraceMiddleware())
//	r.Use(log.RequestLogger())
//	r.Use(log.Recovery())
//
// Every request writes exactly two info lines sharing one trace id:
// "api-start" (method, route, query, request body) and "api-end"
// (status, latency, response body, merchant / operator after auth).
// api-end is written even when a handler aborts or panics.
func RequestLogger() gin.HandlerFunc {
	return AccessLogger(AccessLogConfigFromViper())
}

// AccessLogger builds the api-start / api-end middleware from cfg.
func AccessLogger(cfg AccessLogConfig) gin.HandlerFunc {
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 4096
	}
	exact := map[string]struct{}{}
	var prefixes []string
	for _, p := range cfg.SkipPaths {
		p = strings.TrimSpace(p)
		switch {
		case p == "":
		case strings.HasSuffix(p, "*"):
			prefixes = append(prefixes, strings.TrimSuffix(p, "*"))
		default:
			exact[p] = struct{}{}
		}
	}
	skip := func(c *gin.Context) bool {
		if cfg.SkipOptions && c.Request.Method == http.MethodOptions {
			return true
		}
		path := c.Request.URL.Path
		if strings.HasSuffix(path, "/metrics") {
			return true
		}
		if _, ok := exact[path]; ok {
			return true
		}
		for _, p := range prefixes {
			if strings.HasPrefix(path, p) {
				return true
			}
		}
		return false
	}

	return func(c *gin.Context) {
		if !cfg.Enabled || skip(c) {
			limitRequestBody(c, cfg)
			c.Next()
			return
		}
		limitRequestBody(c, cfg)
		start := time.Now()
		ctx := ensureTrace(c)

		startFields := logrus.Fields{
			"method":        c.Request.Method,
			"route":         c.FullPath(),
			"path":          c.Request.URL.Path,
			skipStackField:  true,
			skipCallerField: true,
		}
		if q := redactValues(c.Request.URL.Query(), true); len(q) > 0 {
			startFields["query"] = q
		}
		if ua := c.Request.UserAgent(); ua != "" {
			startFields["user_agent"] = truncate(ua, 256)
		}
		if cfg.RequestBody {
			captureRequestBody(c, cfg.MaxBodyBytes, startFields)
		}
		Log(ctx).WithFields(startFields).Info(MsgAPIStart)

		var rw *captureWriter
		if cfg.ResponseBody && !isUpgrade(c.Request) {
			rw = &captureWriter{ResponseWriter: c.Writer, max: cfg.MaxBodyBytes}
			c.Writer = rw
		}

		finished := false
		defer func() {
			if finished {
				return
			}
			// A panic escaped every inner recovery: still close the pair,
			// then let it propagate to the outer handler / net/http.
			rec := recover()
			writeAPIEnd(c, cfg, start, rw, rec)
			if rec != nil {
				panic(rec)
			}
		}()
		c.Next()
		finished = true
		writeAPIEnd(c, cfg, start, rw, nil)
	}
}

func ensureTrace(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	traceID := TraceID(ctx)
	if traceID == "" {
		if v, ok := c.Get("RequestID"); ok {
			if s, ok := v.(string); ok {
				traceID = s
			}
		}
	}
	if traceID == "" {
		traceID = c.GetHeader("X-Request-ID")
	}
	if traceID == "" {
		traceID = c.GetHeader("X-Trace-ID")
	}
	if traceID == "" {
		traceID = NewTraceID()
	}
	ip := ClientIP(ctx)
	if ip == "" {
		ip = c.ClientIP()
	}
	if TraceID(ctx) != traceID || ClientIP(ctx) != ip {
		ctx = Inject(ctx, traceID, ip)
		c.Request = c.Request.WithContext(ctx)
	}
	if _, ok := c.Get("RequestID"); !ok {
		c.Set("RequestID", traceID)
	}
	return ctx
}

func writeAPIEnd(c *gin.Context, cfg AccessLogConfig, start time.Time, rw *captureWriter, rec any) {
	latency := time.Since(start)
	status := c.Writer.Status()
	if rec != nil && !c.Writer.Written() {
		status = http.StatusInternalServerError
	}
	fields := logrus.Fields{
		"method":        c.Request.Method,
		"route":         c.FullPath(),
		"path":          c.Request.URL.Path,
		"status":        status,
		"latency_ms":    math.Round(float64(latency.Microseconds())/10) / 100,
		skipStackField:  true,
		skipCallerField: true,
	}
	size := c.Writer.Size()
	if size < 0 {
		size = 0
	}
	fields["resp_size"] = size
	if uid := ginString(c, verifiedUserIDKey); uid != "" {
		fields["user_id"] = uid
	}
	if c.IsAborted() {
		fields["aborted"] = true
	}
	if cfg.SlowThreshold > 0 && latency > cfg.SlowThreshold {
		fields["slow"] = true
	}
	if len(c.Errors) > 0 {
		fields[logrus.ErrorKey] = truncate(c.Errors.String(), 1024)
	}
	if p := ginString(c, PanicContextKey); p != "" {
		fields["panic"] = truncate(p, 512)
	}
	if rec != nil {
		fields["panic"] = truncate(fmt.Sprint(rec), 512)
	}
	if rw != nil && (cfg.ResponseBodyOnSuccess || status >= 400 || rec != nil || fields["slow"] == true || fields["panic"] != nil) {
		rw.addFields(fields)
	}

	entry := Log(c.Request.Context()).WithFields(fields)
	switch {
	case rec != nil:
		entry.Error(MsgAPIEnd)
	case cfg.StatusLevel && status >= 500:
		entry.Error(MsgAPIEnd)
	case cfg.StatusLevel && (status >= 400 || fields["slow"] == true):
		entry.Warn(MsgAPIEnd)
	default:
		entry.Info(MsgAPIEnd)
	}
}

func ginString(c *gin.Context, key string) string {
	if v, ok := c.Get(key); ok {
		switch s := v.(type) {
		case string:
			return s
		case fmt.Stringer:
			return s.String()
		case error:
			return s.Error()
		}
	}
	return ""
}

func isUpgrade(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") ||
		r.Header.Get("Upgrade") != ""
}

func mediaType(ct string) string {
	if ct == "" {
		return ""
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	}
	return mt
}

func isJSONType(mt string) bool {
	return mt == "application/json" || strings.HasSuffix(mt, "+json")
}

// captureRequestBody reads at most max+1 bytes and puts them back in front
// of the remaining stream, so handlers still see the full body.
func captureRequestBody(c *gin.Context, max int, fields logrus.Fields) {
	r := c.Request
	if r.Body == nil || r.Body == http.NoBody || r.ContentLength == 0 {
		return
	}
	mt := mediaType(r.Header.Get("Content-Type"))
	isForm := mt == "application/x-www-form-urlencoded"
	if !isJSONType(mt) && !isForm {
		fields["req_body"] = "[omitted " + orUnknown(mt) + "]"
		return
	}
	head, err := io.ReadAll(io.LimitReader(r.Body, int64(max)+1))
	r.Body = &replayBody{Reader: io.MultiReader(bytes.NewReader(head), r.Body), closer: r.Body}
	if err != nil && len(head) == 0 {
		return
	}
	truncated := len(head) > max
	if truncated {
		head = head[:max]
	}
	switch {
	case isForm && !truncated:
		vals, perr := url.ParseQuery(string(head))
		if perr == nil {
			if m := redactValues(vals, true); len(m) > 0 {
				fields["req_body"] = m
			}
			return
		}
		fields["req_body"] = "[unparsable form]"
	case isForm:
		fields["req_body"] = "[form larger than log.access.max_body]"
		fields["req_body_truncated"] = true
	default:
		putBody(fields, "req_body", head, truncated, true)
	}
}

func putBody(fields logrus.Fields, key string, b []byte, truncated, request bool) {
	if len(bytes.TrimSpace(b)) == 0 {
		return
	}
	if !truncated {
		if raw, ok := redactJSON(b, request); ok {
			fields[key] = raw
			return
		}
	}
	fields[key] = redactJSONText(string(b), request)
	if truncated {
		fields[key+"_truncated"] = true
	}
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown content type"
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

type replayBody struct {
	io.Reader
	closer io.Closer
}

func (b *replayBody) Close() error { return b.closer.Close() }

// captureWriter tees up to max bytes of a JSON response for api-end.
// Non-JSON, streaming (SSE) and file responses are not buffered.
type captureWriter struct {
	gin.ResponseWriter
	buf       bytes.Buffer
	max       int
	decided   bool
	capture   bool
	truncated bool
	mt        string
}

func (w *captureWriter) decide() {
	if w.decided {
		return
	}
	w.decided = true
	w.mt = mediaType(w.Header().Get("Content-Type"))
	w.capture = isJSONType(w.mt)
}

func (w *captureWriter) tee(b []byte) {
	w.decide()
	if !w.capture || w.truncated {
		return
	}
	room := w.max - w.buf.Len()
	if len(b) > room {
		w.buf.Write(b[:room])
		w.truncated = true
		return
	}
	w.buf.Write(b)
}

func (w *captureWriter) Write(b []byte) (int, error) {
	w.tee(b)
	return w.ResponseWriter.Write(b)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	w.tee([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

func (w *captureWriter) addFields(fields logrus.Fields) {
	if !w.decided {
		return
	}
	if !w.capture {
		if w.mt != "" && w.Size() > 0 {
			fields["resp_body"] = "[omitted " + w.mt + "]"
		}
		return
	}
	putBody(fields, "resp_body", w.buf.Bytes(), w.truncated, false)
}

// limitRequestBody wraps the request body with http.MaxBytesReader.
// Default 1 MiB; paths under UploadPathPrefixes use MaxUploadBody (default 32 MiB).
func limitRequestBody(c *gin.Context, cfg AccessLogConfig) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return
	}
	max := cfg.MaxRequestBody
	if max <= 0 {
		max = 1 << 20
	}
	path := c.Request.URL.Path
	for _, p := range cfg.UploadPathPrefixes {
		p = strings.TrimSpace(p)
		if p != "" && strings.HasPrefix(path, p) {
			max = cfg.MaxUploadBody
			if max <= 0 {
				max = 32 << 20
			}
			break
		}
	}
	// Common upload URL shapes when no prefixes configured.
	if len(cfg.UploadPathPrefixes) == 0 && (strings.Contains(path, "/upload") || strings.Contains(path, "/file")) {
		max = cfg.MaxUploadBody
		if max <= 0 {
			max = 32 << 20
		}
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
}
