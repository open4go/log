package log

import (
	"context"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

var logger = logrus.New()

// =======================
// 构建 & 实例元信息
// =======================

type buildMeta struct {
	Image     string
	GitCommit string
	GitBranch string
	BuildTime string
	Instance  string
}

var meta = buildMeta{
	Image:     os.Getenv("IMAGE_TAG"),  // 例如 product-api:v1.2.3
	GitCommit: os.Getenv("GIT_COMMIT"), // git rev-parse --short HEAD
	GitBranch: os.Getenv("GIT_BRANCH"), // main / release
	BuildTime: os.Getenv("BUILD_TIME"), // 2025-12-28T15:00:00Z
	Instance:  os.Getenv("HOSTNAME"),   // 容器实例ID
}

// =======================
// 初始化
// =======================

// Init 在 main 函数中调用
func Init(logLevel string, output io.Writer) {
	if output != nil {
		logger.SetOutput(output)
	} else {
		logger.SetOutput(os.Stdout)
	}

	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})

	switch logLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}
}

// =======================
// 公共日志入口
// =======================

// Log 普通日志（无堆栈）
func Log(ctx context.Context) *logrus.Entry {
	filename, fn := getCallerInfo(2)
	return getBaseEntry(ctx, filename, fn)
}

// ErrorWithStack 错误日志（带堆栈）
func ErrorWithStack(ctx context.Context, err error, args ...interface{}) {
	filename, fn := getCallerInfo(2)
	entry := getBaseEntry(ctx, filename, fn).
		WithField("stacktrace", getStackTrace())

	if len(args) == 0 {
		entry.Error(err)
	} else {
		entry.WithError(err).Error(args...)
	}
}

// ErrorfWithStack 格式化错误日志（带堆栈）
func ErrorfWithStack(ctx context.Context, err error, format string, args ...interface{}) {
	filename, fn := getCallerInfo(2)
	entry := getBaseEntry(ctx, filename, fn).
		WithField("stacktrace", getStackTrace())

	entry.WithError(err).Errorf(format, args...)
}

// =======================
// 内部工具函数
// =======================

func getBaseEntry(ctx context.Context, filename, fn string) *logrus.Entry {
	serverName := viper.GetString("server.name")

	logCtx := logger.
		WithField("server", serverName).
		WithField("file", filename).
		WithField("func", fn)

	// ===== 请求上下文 =====
	if ctx != nil {
		if traceID := ctx.Value("traceid"); traceID != "" {
			logCtx = logCtx.WithField("trace", traceID)
		}
		if ip := ctx.Value("ip"); ip != "" {
			logCtx = logCtx.WithField("ip", ip)
		}
		if merchantId := ctx.Value("MERCHANT_KEY"); merchantId != "" {
			logCtx = logCtx.WithField("merchantId", merchantId)
		}
		if operator := ctx.Value("OPERATOR_KEY"); operator != "" {
			logCtx = logCtx.WithField("operator", operator)
		}
	}

	// ===== 构建 & 实例信息（稳定）=====
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

// 获取调用者信息
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

// 获取堆栈
func getStackTrace() string {
	return string(debug.Stack())
}
