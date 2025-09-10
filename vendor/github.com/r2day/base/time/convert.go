package time

import (
	"fmt"
	"time"
)

// SecondsToMinutes 时间转换
func SecondsToMinutes(inSeconds int) string {
	minutes := inSeconds / 60
	seconds := inSeconds % 60
	str := fmt.Sprintf("%d:%d", minutes, seconds)
	return str
}


// 格式化时间戳
func FormatTime(timestamp int64, layout string) string {
    tm := time.Unix(timestamp, 0)
    return tm.Format(layout)
}

// 按照 2006-01-02 15:04:05 格式输出
func FomratTimeAsReader(timestamp int64) string {
	layout := "2006-01-02 15:04:05"
	return FormatTime(timestamp, layout)
}