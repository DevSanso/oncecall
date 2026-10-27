package utils

import "time"

func IntervalSleep(interval time.Duration, offset time.Duration) {
	now := time.Now()
	next := now.Truncate(interval).
		Add(interval).
		Add(offset)

	time.Sleep(time.Until(next))
}
