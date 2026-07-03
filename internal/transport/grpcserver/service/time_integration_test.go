//go:build integration

package service

import "time"

func fixedTestTime() time.Time {
	return time.Date(2026, time.June, 23, 10, 0, 0, 0, time.UTC).Truncate(time.Microsecond)
}
