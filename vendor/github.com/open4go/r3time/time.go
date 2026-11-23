package r3time

import (
	"fmt"
	"time"
)

// CurrentTimestamp 当前时间戳
func CurrentTimestamp() int64 {
	// 加载时区信息
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Println("Error loading location:", err)
		return 0
	}

	// Using time.Now() function.
	dt := time.Now().In(loc)
	return dt.Unix()
}

// CurrentTime 当前时间
// 格式 2006.01.02 15:04:05
func CurrentTime() string {
	// 加载时区信息
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Println("Error loading location:", err)
		return ""
	}

	// Using time.Now() function.
	dt := time.Now().In(loc)
	t := fmt.Sprintf("%v", dt.Format(time.DateTime))
	return t
}

// Today 获取当天日期
// 格式 20060102
func Today() string {
	// 加载时区信息
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Println("Error loading location:", err)
		return ""
	}

	// Using time.Now() function.
	dt := time.Now().In(loc)
	t := fmt.Sprintf("%v", dt.Format("20060102"))
	return t
}

// Timestamp2ReadTime 获取可读时间
// 格式 2006.01.02 15:04:05
func Timestamp2ReadTime(timestamp int64) string {
	// 加载时区信息
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Println("Error loading location:", err)
		return ""
	}

	// 将时间戳转换为 time.Time，并指定时区
	dt := time.Unix(timestamp, 0).In(loc)

	// 格式化时间为可读字符串
	// 使用 "2006-01-02 15:04:05" 这个固定的时间格式
	t := dt.Format("2006-01-02 15:04:05")
	return t
}
