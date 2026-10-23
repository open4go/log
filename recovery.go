package log

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Recovery replaces gin.Recovery: a panic becomes one JSON error line
// ("panic recovered", with trace id and a cleaned stacktrace) instead of
// gin's multi-line text dump on stderr, and the request answers 500.
// The access log's api-end line is flagged with "panic".
// Broken-pipe panics are handled by gin as before (no 500 written).
func Recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, rec any) {
		msg := fmt.Sprint(rec)
		c.Set(PanicContextKey, msg)
		Log(c.Request.Context()).
			WithField("panic", truncate(msg, 512)).
			WithField("method", c.Request.Method).
			WithField("route", c.FullPath()).
			WithField("path", c.Request.URL.Path).
			WithField(wantStackField, true).
			Error("panic recovered")
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}
