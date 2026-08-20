package log

import (
	"context"
	"io"
	"os"
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

func init() {
	logger.SetFormatter(jsonFormatter())
	logger.SetOutput(os.Stdout)
	logger.AddHook(locationHook{})
}

func jsonFormatter() logrus.Formatter {
	return &logrus.JSONFormatter{
		TimestampFormat:   "2006-01-02T15:04:05.000Z07:00",
		DisableHTMLEscape: true,
	}
}

// Init configures the process-wide JSON logger. Call once from main.
// logLevel: debug | test | info | warn | error (unknown falls back to info).
func Init(logLevel string, output io.Writer) {
	if output != nil {
		logger.SetOutput(output)
	} else {
		logger.SetOutput(os.Stdout)
	}

	logger.SetFormatter(jsonFormatter())
	logger.SetReportCaller(false)

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
// It attaches server, trace, ip, merchant and operator when present.
// Caller file/func/line are filled when the line is written.
// Error-level lines also get a cleaned stacktrace.
// Do not call Fatal from request handlers — Fatal exits the process.
func Log(ctx context.Context) *logrus.Entry {
	return getBaseEntry(ctx)
}

// ErrorWithStack is an alias of Error, kept for callers compiled against
// older versions of this package.
func ErrorWithStack(ctx context.Context, err error, args ...interface{}) {
	Error(ctx, err, args...)
}

// ErrorfWithStack is an alias of Errorf, kept for older callers.
func ErrorfWithStack(ctx context.Context, err error, format string, args ...interface{}) {
	Errorf(ctx, err, format, args...)
}

// Error writes an error log with caller location and a cleaned stack trace.
func Error(ctx context.Context, err error, args ...interface{}) {
	entry := errorEntry(ctx, err)
	if len(args) == 0 {
		if err != nil {
			entry.Error(err)
			return
		}
		entry.Error("error")
		return
	}
	entry.Error(args...)
}

// Errorf writes a formatted error log with caller location and a cleaned stack trace.
func Errorf(ctx context.Context, err error, format string, args ...interface{}) {
	errorEntry(ctx, err).Errorf(format, args...)
}

// WarnWithStack writes a warning with a stack trace.
func WarnWithStack(ctx context.Context, msg interface{}, args ...interface{}) {
	entry := getBaseEntry(ctx).WithField(wantStackField, true)
	if len(args) == 0 {
		entry.Warn(msg)
		return
	}
	entry.WithFields(logrus.Fields{"details": args}).Warn(msg)
}

// WarnfWithStack writes a formatted warning with a stack trace.
func WarnfWithStack(ctx context.Context, format string, args ...interface{}) {
	getBaseEntry(ctx).WithField(wantStackField, true).Warnf(format, args...)
}

func errorEntry(ctx context.Context, err error) *logrus.Entry {
	entry := getBaseEntry(ctx)
	if err != nil {
		entry = entry.WithError(err)
	}
	return entry
}

func getBaseEntry(ctx context.Context) *logrus.Entry {
	fields := make(logrus.Fields, 12)
	fields["server"] = viper.GetString("server.name")

	if traceID := TraceID(ctx); traceID != "" {
		fields["trace"] = traceID
	}
	if ip := ClientIP(ctx); ip != "" {
		fields["ip"] = ip
	}
	if merchantId := ctxString(ctx, merchantCtxKey); merchantId != "" {
		fields["merchantId"] = merchantId
	}
	if operator := ctxString(ctx, operatorCtxKey); operator != "" {
		fields["operator"] = operator
	}

	if meta.Image != "" {
		fields["image"] = meta.Image
	}
	if meta.GitCommit != "" {
		fields["git_commit"] = meta.GitCommit
	}
	if meta.GitBranch != "" {
		fields["git_branch"] = meta.GitBranch
	}
	if meta.BuildTime != "" {
		fields["build_time"] = meta.BuildTime
	}
	if meta.Instance != "" {
		fields["instance"] = meta.Instance
	}

	return logger.WithFields(fields)
}
