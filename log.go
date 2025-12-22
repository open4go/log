package log

import (
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

// getDockerMetadata fetches the Docker container metadata
func getDockerMetadata() (string, string, string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", "", "", err
	}

	ctx := context.Background()
	containerID := os.Getenv("HOSTNAME")
	containerJSON, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", "", "", err
	}

	return containerJSON.Config.Image, containerJSON.Name, containerID, nil
}

var logger = logrus.New()

// Init 在main函数中必须初始化
func Init(logLevel string, output io.Writer) {
	if output != nil {
		logger.SetOutput(output)
	} else {
		// 输出到终端
		logger.SetOutput(os.Stdout)
	}
	// 强制使用json日志格式
	logger.SetFormatter(&logrus.JSONFormatter{})
	// 设置日志级别
	switch logLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "test":
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

// getStackTrace 获取完整的调用栈信息
func getStackTrace() string {
	// 获取当前goroutine的堆栈信息
	stack := debug.Stack()
	return string(stack)
}

// getCallerInfo 获取调用者信息
func getCallerInfo(skip int) (string, string) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", "0"
	}

	// 获取文件名（包含父目录）
	fileParts := strings.Split(file, "/")
	var filename string
	if len(fileParts) > 1 {
		filename = strings.Join(fileParts[len(fileParts)-2:], "/") + ":" + strconv.Itoa(line)
	} else {
		filename = fileParts[0] + ":" + strconv.Itoa(line)
	}

	// 获取函数名
	funcName := runtime.FuncForPC(pc).Name()
	fn := funcName[strings.LastIndex(funcName, ".")+1:]

	return filename, fn
}

// Log 获取日志记录器，默认不包含堆栈信息
func Log(ctx context.Context) *logrus.Entry {
	filename, fn := getCallerInfo(2)
	return getBaseEntry(ctx, filename, fn)
}

// ErrorWithStack 专门用于错误日志，包含堆栈信息
func ErrorWithStack(ctx context.Context, err error, args ...interface{}) {
	filename, fn := getCallerInfo(2)
	entry := getBaseEntry(ctx, filename, fn)

	// 添加堆栈信息
	stack := getStackTrace()
	entry = entry.WithField("stacktrace", stack)

	// 记录错误
	if len(args) == 0 {
		entry.Error(err)
	} else {
		entry.WithError(err).Error(args...)
	}
}

// ErrorfWithStack 格式化错误日志，包含堆栈信息
func ErrorfWithStack(ctx context.Context, err error, format string, args ...interface{}) {
	filename, fn := getCallerInfo(2)
	entry := getBaseEntry(ctx, filename, fn)

	// 添加堆栈信息
	stack := getStackTrace()
	entry = entry.WithField("stacktrace", stack)

	// 记录错误
	entry.WithError(err).Errorf(format, args...)
}

// getBaseEntry 获取基础日志条目，包含公共字段
func getBaseEntry(ctx context.Context, filename, fn string) *logrus.Entry {
	serverName := viper.GetString("server.name")

	logCtx := logger.
		WithField("file", filename).
		WithField("func", fn).
		WithField("server", serverName)

	// 增加traceid
	traceID := ctx.Value("traceid")
	if traceID != "" {
		logCtx = logCtx.WithField("trace", traceID)
	}

	// 增加请求ip
	ip := ctx.Value("ip")
	if ip != "" {
		logCtx = logCtx.WithField("ip", ip)
	}

	merchantId := ctx.Value("MERCHANT_KEY")
	if merchantId != "" {
		logCtx = logCtx.WithField("merchantId", merchantId)
	}

	operator := ctx.Value("OPERATOR_KEY")
	if operator != "" {
		logCtx = logCtx.WithField("operator", operator)
	}

	// 获取镜像元数据
	image, container, instanceID, err := getDockerMetadata()
	if err != nil {
		//log.Printf("Failed to get Docker metadata (when run it on local, can ignore this) %v", err)
		return logCtx
	} else {
		// 只有容器中运行才能获取到相关信息
		// 并且运行的容器需要挂着配置 /var/run/docker.sock
		return logCtx.
			WithField("image", image).
			WithField("container", container).
			WithField("instance", instanceID)
	}
}

// 可以添加一个钩子，自动为所有error级别日志添加堆栈信息
func init() {
	// 添加自定义钩子，当记录error级别日志时自动添加堆栈信息
	// 注意：这种方式会影响所有error级别日志，包括非error方法记录的错误
	// 可以根据需求决定是否启用

	// logger.AddHook(&errorStackHook{})
}

// errorStackHook 错误堆栈钩子，自动为error级别日志添加堆栈信息
type errorStackHook struct{}

func (h *errorStackHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
}

func (h *errorStackHook) Fire(entry *logrus.Entry) error {
	// 只有在没有stacktrace字段时才添加
	if _, ok := entry.Data["stacktrace"]; !ok {
		stack := getStackTrace()
		entry.Data["stacktrace"] = stack
	}
	return nil
}
