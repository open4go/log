package log

import (
	"context"
	"io"
	"os"
	"strings"
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var logger = logrus.New()

// formatSettings are read on every log line; swapped atomically by Init.
type formatSettings struct {
	// stackLevel: lines at this level or more severe get a stacktrace.
	// noStack disables automatic stacks (explicit *WithStack still works).
	stackLevel logrus.Level
	noStack    bool
	callerFunc bool
}

func (s *formatSettings) stackEnabled(l logrus.Level) bool {
	return !s.noStack && l <= s.stackLevel
}

var current atomic.Pointer[formatSettings]

func settings() *formatSettings {
	if s := current.Load(); s != nil {
		return s
	}
	return defaultSettings()
}

func defaultSettings() *formatSettings {
	return &formatSettings{stackLevel: logrus.FatalLevel}
}

func init() {
	logger.SetFormatter(jsonFormatter())
	logger.SetOutput(os.Stdout)
	logger.AddHook(locationHook{})
	current.Store(loadSettings())
	refreshMeta()
	refreshKind()
}

func jsonFormatter() logrus.Formatter {
	return &orderedJSONFormatter{TimestampFormat: "2006-01-02T15:04:05.000Z07:00"}
}

// configValue returns env (if set) else viper key.
func configValue(env, key string) string {
	if v := strings.TrimSpace(os.Getenv(env)); v != "" {
		return v
	}
	return strings.TrimSpace(viper.GetString(key))
}

func loadSettings() *formatSettings {
	s := defaultSettings()
	switch strings.ToLower(configValue("LOG_STACKTRACE", "log.stacktrace")) {
	case "error":
		s.stackLevel = logrus.ErrorLevel
	case "warn", "warning":
		s.stackLevel = logrus.WarnLevel
	case "none", "off", "false":
		s.noStack = true
	}
	switch strings.ToLower(configValue("LOG_CALLER_FUNC", "log.caller_func")) {
	case "1", "true", "yes", "on":
		s.callerFunc = true
	}
	return s
}

// ParseLevel maps debug | test | info | warn | error (unknown → info).
func ParseLevel(logLevel string) logrus.Level {
	switch strings.ToLower(strings.TrimSpace(logLevel)) {
	case "trace":
		return logrus.TraceLevel
	case "debug", "test":
		return logrus.DebugLevel
	case "warn", "warning":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	default:
		return logrus.InfoLevel
	}
}

// Init configures the process-wide JSON logger. Call once from main, after
// the config file is loaded.
//
// logLevel: debug | test | info | warn | error (unknown falls back to info).
// The LOG_LEVEL environment variable, when set, overrides logLevel.
// Other options (env overrides viper):
//
//	log.stacktrace  / LOG_STACKTRACE : fatal (default) | error | warn | none
//	log.caller_func / LOG_CALLER_FUNC: true adds a "func" field next to "caller"
//
// Init also refreshes build metadata used on every line:
//
//	image      ← IMAGE_TAG, else SERVER_VERSION / APP_VERSION, else ldImage, else VERSION file
//	git_commit ← GIT_COMMIT, else ldGitCommit, else runtime/debug vcs.revision
//	git_branch ← GIT_BRANCH, else ldGitBranch
//	build_time ← BUILD_TIME, else ldBuildTime, else vcs.time
//	instance   ← HOSTNAME
//	kind       ← SERVER_KIND / LOG_KIND, else server.kind / log.kind,
//	             else COMPOSE_PROJECT_NAME / hostname (api, rpc, or task)
func Init(logLevel string, output io.Writer) {
	if output != nil {
		logger.SetOutput(output)
	} else {
		logger.SetOutput(os.Stdout)
	}

	logger.SetFormatter(jsonFormatter())
	logger.SetReportCaller(false)

	if env := strings.TrimSpace(os.Getenv("LOG_LEVEL")); env != "" {
		logLevel = env
	}
	logger.SetLevel(ParseLevel(logLevel))
	current.Store(loadSettings())
	// Re-read IMAGE_TAG / GIT_COMMIT and fallbacks so values injected after
	// package import (compose env is actually present at import, but tests
	// and late config load can set them afterwards) are not sticky.
	refreshMeta()
	refreshKind()
}

// SetLevel changes the level at runtime (debug | info | warn | error).
func SetLevel(logLevel string) {
	logger.SetLevel(ParseLevel(logLevel))
}

// Logger returns the process-wide logrus logger.
func Logger() *logrus.Logger {
	return logger
}

// Log is the common entry for Info / Debug / Warn / Error.
// It attaches server, kind, trace, ip, merchant and operator when present.
// A short caller ("pkg/file.go:line") is filled when the line is written.
// Stack traces are only attached to panic / fatal lines (see Init).
// Do not call Fatal from request handlers — Fatal exits the process.
func Log(ctx context.Context) *logrus.Entry {
	return getBaseEntry(ctx)
}

// ErrorWithStack is Error plus a cleaned stack trace.
func ErrorWithStack(ctx context.Context, err error, args ...interface{}) {
	writeError(errorEntry(ctx, err).WithField(wantStackField, true), err, args...)
}

// ErrorfWithStack is Errorf plus a cleaned stack trace.
func ErrorfWithStack(ctx context.Context, err error, format string, args ...interface{}) {
	errorEntry(ctx, err).WithField(wantStackField, true).Errorf(format, args...)
}

// Error writes an error log with caller location (no stack trace unless
// log.stacktrace=error).
func Error(ctx context.Context, err error, args ...interface{}) {
	writeError(errorEntry(ctx, err), err, args...)
}

func writeError(entry *logrus.Entry, err error, args ...interface{}) {
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

// Errorf writes a formatted error log with caller location.
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
	fields := make(logrus.Fields, 13)
	fields["server"] = viper.GetString("server.name")
	fields["kind"] = Kind()

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

	m := currentMeta()
	if m.Image != "" {
		fields["image"] = m.Image
	}
	if m.GitCommit != "" {
		fields["git_commit"] = m.GitCommit
	}
	if m.GitBranch != "" {
		fields["git_branch"] = m.GitBranch
	}
	if m.BuildTime != "" {
		fields["build_time"] = m.BuildTime
	}
	if m.Instance != "" {
		fields["instance"] = m.Instance
	}

	return logger.WithFields(fields)
}
