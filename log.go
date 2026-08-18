package log

import (
	"context"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var logger = logrus.New()

type buildMeta struct {
	Image     string
	GitCommit string
	GitBranch string
	BuildTime string
	Instance  string
}

var meta = buildMeta{
	Image:     os.Getenv("IMAGE_TAG"),
	GitCommit: os.Getenv("GIT_COMMIT"),
	GitBranch: os.Getenv("GIT_BRANCH"),
	BuildTime: os.Getenv("BUILD_TIME"),
	Instance:  os.Getenv("HOSTNAME"),
}

// Init configures the process-wide JSON logger. Call once from main.
// logLevel: debug | test | info | warn | error (unknown falls back to info).
func Init(logLevel string, output io.Writer) {
	if output != nil {
		logger.SetOutput(output)
	} else {
		logger.SetOutput(os.Stdout)
	}

	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})

	switch strings.ToLower(logLevel) {
	case "debug", "test":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn", "warning":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}
}

// Logger returns the process-wide logrus logger.
func Logger() *logrus.Logger {
	return logger
}

// Log is the common entry for Info / Debug / Warn / Error.
// It attaches server, caller, trace, ip, merchant and operator when present.
// Do not call Fatal from request handlers — Fatal exits the process.
func Log(ctx context.Context) *logrus.Entry {
	filename, fn := getCallerInfo(2)
	return getBaseEntry(ctx, filename, fn)
}

// ErrorWithStack is an alias of Error, kept for callers compiled against
// older versions of this package.
func ErrorWithStack(ctx context.Context, err error, args ...interface{}) {
	filename, fn := getCallerInfo(2)
	entry := getBaseEntry(ctx, filename, fn).
		WithField("stacktrace", getStackTrace())
	if len(args) == 0 {
		entry.Error(err)
		return
	}
	entry.WithError(err).Error(args...)
}

// ErrorfWithStack is an alias of Errorf, kept for older callers.
func ErrorfWithStack(ctx context.Context, err error, format string, args ...interface{}) {
	filename, fn := getCallerInfo(2)
	getBaseEntry(ctx, filename, fn).
		WithField("stacktrace", getStackTrace()).
		WithError(err).
		Errorf(format, args...)
}

// Error writes an error log with a stack trace.
func Error(ctx context.Context, err error, args ...interface{}) {
	filename, fn := getCallerInfo(2)
	entry := getBaseEntry(ctx, filename, fn).
		WithField("stacktrace", getStackTrace())

	if len(args) == 0 {
		entry.Error(err)
		return
	}
	entry.WithError(err).Error(args...)
}

// Errorf writes a formatted error log with a stack trace.
func Errorf(ctx context.Context, err error, format string, args ...interface{}) {
	filename, fn := getCallerInfo(2)
	getBaseEntry(ctx, filename, fn).
		WithField("stacktrace", getStackTrace()).
		WithError(err).
		Errorf(format, args...)
}

// WarnWithStack writes a warning with a stack trace.
func WarnWithStack(ctx context.Context, msg interface{}, args ...interface{}) {
	filename, fn := getCallerInfo(2)
	entry := getBaseEntry(ctx, filename, fn).
		WithField("stacktrace", getStackTrace())

	if len(args) == 0 {
		entry.Warn(msg)
		return
	}
	entry.WithFields(logrus.Fields{"details": args}).Warn(msg)
}

// WarnfWithStack writes a formatted warning with a stack trace.
func WarnfWithStack(ctx context.Context, format string, args ...interface{}) {
	filename, fn := getCallerInfo(2)
	getBaseEntry(ctx, filename, fn).
		WithField("stacktrace", getStackTrace()).
		Warnf(format, args...)
}

func getBaseEntry(ctx context.Context, filename, fn string) *logrus.Entry {
	serverName := viper.GetString("server.name")

	logCtx := logger.
		WithField("server", serverName).
		WithField("file", filename).
		WithField("func", fn)

	if ctx != nil {
		if traceID := TraceID(ctx); traceID != "" {
			logCtx = logCtx.WithField("trace", traceID)
		}
		if ip := ClientIP(ctx); ip != "" {
			logCtx = logCtx.WithField("ip", ip)
		}
		if merchantId := ctxString(ctx, merchantCtxKey); merchantId != "" {
			logCtx = logCtx.WithField("merchantId", merchantId)
		}
		if operator := ctxString(ctx, operatorCtxKey); operator != "" {
			logCtx = logCtx.WithField("operator", operator)
		}
	}

	if meta.Image != "" {
		logCtx = logCtx.WithField("image", meta.Image)
	}
	if meta.GitCommit != "" {
		logCtx = logCtx.WithField("git_commit", meta.GitCommit)
	}
	if meta.GitBranch != "" {
		logCtx = logCtx.WithField("git_branch", meta.GitBranch)
	}
	if meta.BuildTime != "" {
		logCtx = logCtx.WithField("build_time", meta.BuildTime)
	}
	if meta.Instance != "" {
		logCtx = logCtx.WithField("instance", meta.Instance)
	}

	return logCtx
}

func getCallerInfo(skip int) (string, string) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", "unknown"
	}

	fileParts := strings.Split(file, "/")
	filename := fileParts[len(fileParts)-1] + ":" + strconv.Itoa(line)

	funcName := runtime.FuncForPC(pc).Name()
	fn := funcName[strings.LastIndex(funcName, ".")+1:]

	return filename, fn
}

func getStackTrace() string {
	return string(debug.Stack())
}
