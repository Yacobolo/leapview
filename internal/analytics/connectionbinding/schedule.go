package connectionbinding

import (
	"context"
	"fmt"
	"math"
	"time"
)

type RefreshSchedule struct {
	Interval       time.Duration
	BackoffInitial time.Duration
	BackoffMax     time.Duration
	JitterRatio    float64
	Random         func() float64
	Wait           func(context.Context, time.Duration) error
}

func (schedule RefreshSchedule) validate() error {
	if schedule.Interval <= 0 || schedule.BackoffInitial <= 0 ||
		schedule.BackoffMax < schedule.BackoffInitial ||
		schedule.JitterRatio < 0 || schedule.JitterRatio > 0.5 ||
		schedule.Random == nil || schedule.Wait == nil {
		return fmt.Errorf("%w: refresh interval, bounded backoff, jitter source, and cancellation-aware wait are required", ErrInvalidBinding)
	}
	return nil
}

func (schedule RefreshSchedule) delay(base time.Duration) time.Duration {
	random := schedule.Random()
	if math.IsNaN(random) || random < 0 {
		random = 0
	} else if random > 1 {
		random = 1
	}
	factor := 1 + schedule.JitterRatio*(2*random-1)
	return time.Duration(float64(base) * factor)
}

func (schedule RefreshSchedule) backoff(failures int) time.Duration {
	delay := schedule.BackoffInitial
	for attempt := 1; attempt < failures && delay < schedule.BackoffMax; attempt++ {
		if delay > schedule.BackoffMax/2 {
			return schedule.BackoffMax
		}
		delay *= 2
	}
	if delay > schedule.BackoffMax {
		return schedule.BackoffMax
	}
	return delay
}
