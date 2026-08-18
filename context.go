package log

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Context keys. String values stay equal to historical keys so existing
// writers (CORS, JWT, model meta) keep working.
type ctxKey string

const (
	TraceIDKey ctxKey = "traceid"
	IPKey      ctxKey = "ip"

	// Legacy string keys still written by older middleware.
	legacyTraceIDKey = "traceid"
	legacyIPKey      = "ip"
	merchantCtxKey   = "MERCHANT_KEY"
	operatorCtxKey   = "OPERATOR_KEY"
)

// NewTraceID returns a 32-char hex id. Falls back to a timestamp if
// crypto/rand is unavailable.
func NewTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// WithTraceID stores the request trace id on ctx.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if traceID == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, TraceIDKey, traceID)
	ctx = context.WithValue(ctx, legacyTraceIDKey, traceID)
	return ctx
}

// WithIP stores the client IP on ctx.
func WithIP(ctx context.Context, ip string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if ip == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, IPKey, ip)
	ctx = context.WithValue(ctx, legacyIPKey, ip)
	return ctx
}

// Inject writes trace id and client IP using both typed and legacy keys.
func Inject(ctx context.Context, traceID, ip string) context.Context {
	return WithIP(WithTraceID(ctx, traceID), ip)
}

// TraceID returns the request trace id, or empty string.
func TraceID(ctx context.Context) string {
	return ctxString(ctx, TraceIDKey, legacyTraceIDKey)
}

// ClientIP returns the client IP stored on ctx, or empty string.
func ClientIP(ctx context.Context) string {
	return ctxString(ctx, IPKey, legacyIPKey)
}

func ctxString(ctx context.Context, keys ...any) string {
	if ctx == nil {
		return ""
	}
	for _, k := range keys {
		if v := ctx.Value(k); v != nil {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
