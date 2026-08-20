package log

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// skipAccessLogPaths are infrastructure endpoints that should not emit
// an access line on every probe.
var skipAccessLogPaths = map[string]struct{}{
	"/healthz":     {},
	"/readyz":      {},
	"/health":      {},
	"/ping":        {},
	"/favicon.ico": {},
}

// RequestLogger writes one access log per request and a warning when
// latency exceeds server.maxLatency (milliseconds).
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		c.Next()

		path := c.Request.URL.Path
		if _, skip := skipAccessLogPaths[path]; skip {
			return
		}
		// metrics endpoints are prefixed per service
		if len(path) >= 8 && path[len(path)-8:] == "/metrics" {
			return
		}

		duration := time.Since(startTime)
		method := c.Request.Method
		statusCode := c.Writer.Status()

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
			traceID = c.GetHeader("X-Trace-ID")
		}
		if traceID == "" {
			traceID = c.GetHeader("X-Request-ID")
		}

		ip := ClientIP(ctx)
		if ip == "" {
			ip = c.ClientIP()
		}

		ctx = Inject(ctx, traceID, ip)

		fields := logrus.Fields{
			"method":       method,
			"path":         path,
			"status":       statusCode,
			"latency":      duration.Milliseconds(),
			"ip":           ip,
			"trace":        traceID,
			skipStackField: true,
		}
		if bytes := c.Writer.Size(); bytes >= 0 {
			fields["bytes"] = bytes
		}

		entry := Log(ctx).WithFields(fields)

		maxLatency := viper.GetInt64("server.maxLatency")
		if maxLatency > 0 && duration.Milliseconds() > maxLatency {
			entry.WithField("max_latency", maxLatency).
				Warning("request exceeded max latency")
			return
		}

		switch {
		case statusCode >= 500:
			entry.Error("request completed")
		case statusCode >= 400:
			entry.Warning("request completed")
		default:
			entry.Info("request completed")
		}
	}
}

// TraceMiddleware assigns a request id (reusing inbound X-Request-ID /
// X-Trace-ID when present), writes it into the request context, Gin
// context, and response header so subsequent Log(ctx) calls correlate.
func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Request-ID")
		if traceID == "" {
			traceID = c.GetHeader("X-Trace-ID")
		}
		if traceID == "" {
			traceID = NewTraceID()
		}

		ip := c.ClientIP()
		ctx := Inject(c.Request.Context(), traceID, ip)
		c.Request = c.Request.WithContext(ctx)

		c.Set("RequestID", traceID)
		c.Set("log", Log(ctx))
		c.Header("X-Request-ID", traceID)

		c.Next()
	}
}
