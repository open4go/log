package log

import (
	"encoding/json"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// LogEntry represents a single log entry with additional metadata
type LogEntry struct {
	File       string `json:"file"`
	Func       string `json:"func"`
	Level      string `json:"level"`
	Msg        string `json:"msg"`
	Port       string `json:"port"`
	Prefix     string `json:"prefix"`
	Server     string `json:"server"`
	Time       string `json:"time"`
	Image      string `json:"image"`
	Container  string `json:"container"`
	InstanceID string `json:"instance_id"`
}

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

func demo() {
	image, container, instanceID, err := getDockerMetadata()
	if err != nil {
		log.Fatalf("Failed to get Docker metadata: %v", err)
	}

	// Example log entry
	entry := LogEntry{
		File:       "init.go:53",
		Func:       "loadRoutesAndStartServer",
		Level:      "info",
		Msg:        "Server is running successfully",
		Port:       "8080",
		Prefix:     "/v1/order",
		Server:     "order-api",
		Time:       time.Now().Format(time.RFC3339),
		Image:      image,
		Container:  container,
		InstanceID: instanceID,
	}

	logData, err := json.Marshal(entry)
	if err != nil {
		log.Fatalf("Failed to marshal log entry: %v", err)
	}

	fmt.Println(string(logData))
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

func Log() *logrus.Entry {
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		panic("Could not get context info for logger!")
	}

	serverName := viper.GetString("server.name")

	// 拼接必要字段
	filename := file[strings.LastIndex(file, "/")+1:] + ":" + strconv.Itoa(line)
	funcName := runtime.FuncForPC(pc).Name()
	fn := funcName[strings.LastIndex(funcName, ".")+1:]

	// 获取镜像元数据
	image, container, instanceID, err := getDockerMetadata()
	if err != nil {
		log.Fatalf("Failed to get Docker metadata: %v", err)
	}

	return logger.WithField("file", filename).
		WithField("func", fn).
		WithField("server", serverName).
		WithField("image", image).
		WithField("container", container).
		WithField("instance", instanceID)
}
