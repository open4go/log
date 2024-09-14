package log

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"time"
)

// RequestLogger logs the request time and other relevant details
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Process the request
		c.Next()

		// Calculate request duration
		duration := time.Since(startTime)

		// Get request details
		path := c.Request.URL.Path
		method := c.Request.Method
		statusCode := c.Writer.Status()

		// Add trace ID, IP, and other fields to the context
		ctx := c.Request.Context()
		traceID := ctx.Value("traceid")
		if traceID == "" {
			traceID = c.GetHeader("X-Trace-ID") // Assuming trace ID comes from header
			ctx = context.WithValue(ctx, "traceid", traceID)
		}

		ip := c.ClientIP()

		// Attach context values for trace ID and IP
		ctx = context.WithValue(ctx, "ip", ip)

		currentLatency := duration.Milliseconds()
		maxLatency := viper.GetInt64("server.maxLatency")

		// 如果一个请求时间超过设定的最大时长则应该认为是异常情况
		// 因此打印输出日志便于排查问题
		if currentLatency > maxLatency {
			// Log the request details with the custom logger
			Log(ctx).WithFields(logrus.Fields{
				"method":      method,
				"path":        path,
				"trace":       ctx.Value("traceid"),
				"status":      statusCode,
				"max_latency": maxLatency,
				"latency":     duration.Milliseconds(),
			}).Warning("current request has reached latency")
		}
	}
}
