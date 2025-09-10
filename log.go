package log

import (
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"io"
	"os"
	"runtime"
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

func Log(ctx context.Context) *logrus.Entry {
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		panic("Could not get context info for logger!")
	}

	serverName := viper.GetString("server.name")

	// 拼接必要字段
	//filename := file[strings.LastIndex(file, "/")+1:] + ":" + strconv.Itoa(line)
	// Modify how the filename is extracted to include at least /parent/child.go
	// Split the file path into its components
	fileParts := strings.Split(file, "/")

	// Get at least two levels (parent/child.go), or just child.go if less
	var filename string
	if len(fileParts) > 1 {
		filename = strings.Join(fileParts[len(fileParts)-2:], "/") + ":" + strconv.Itoa(line)
	} else {
		filename = fileParts[0] + ":" + strconv.Itoa(line)
	}

	funcName := runtime.FuncForPC(pc).Name()
	fn := funcName[strings.LastIndex(funcName, ".")+1:]

	logCtx := logger.
		WithField("file", filename).
		WithField("func", fn).
		WithField("server", serverName)
	// 增加traceid
	// 部分情况下无法获取到
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
		// 例如:
		// services:
		//  member:
		//    image: r2day/member-api:pro
		//    volumes:
		//      - /var/run/docker.sock:/var/run/docker.sock
		return logCtx.
			WithField("image", image).
			WithField("container", container).
			WithField("instance", instanceID)
	}
}
