package utils

import "time"

func IntervalSleep(intervalSec time.Duration, millieOffset time.Duration) {
	now := time.Now()
	next := now.Truncate(intervalSec).
		Add(intervalSec).
		Add(millieOffset)

	time.Sleep(time.Until(next))
}
