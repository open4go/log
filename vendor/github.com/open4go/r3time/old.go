package r3time

import "time"

func GetOldTime(n int) []time.Time {
	now := time.Now().UTC()

	times := make([]time.Time, 0)
	for i := 1; i <= n; i++ {
		oldTime := now.AddDate(0, 0, -i)
		times = append(times, oldTime)
	}
	return times
}

func GetRecentlyWeek() []string {
	times := GetOldTime(7)
	timesString := make([]string, 0)
	for _, t := range times {
		timeVal := t.Format("20060102")
		timesString = append(timesString, timeVal)
	}
	return timesString
}
