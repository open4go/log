package log

import "github.com/gin-gonic/gin"

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
