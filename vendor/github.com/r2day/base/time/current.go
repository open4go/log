package time

import (
	"fmt"
	"time"
)

func GetCurrentTime() string {
	// Using time.Now() function.
	dt := time.Now()
	t := fmt.Sprintf("%v", dt.Format("2006.01.02 15:04"))
	return t
}

func GetDaily() string {
	// Using time.Now() function.
	dt := time.Now()
	t := fmt.Sprintf("%v", dt.Format("20060102"))
	return t
}
