package plugin

import (
	"math"
	"time"
)

type BackoffStrategy struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

func NewBackoffStrategy(base, max time.Duration) *BackoffStrategy {
	return &BackoffStrategy{
		BaseDelay: base,
		MaxDelay:  max,
	}
}

// CalculateDelay returns the delay for the n-th retry (0-indexed)
// delay = min(BaseDelay * 2^attempt, MaxDelay)
func (b *BackoffStrategy) CalculateDelay(attempt int) time.Duration {
	if attempt < 0 {
		return b.BaseDelay
	}

	// prevent overflow
	if attempt > 30 {
		return b.MaxDelay
	}

	factor := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(b.BaseDelay) * factor)

	if delay > b.MaxDelay {
		return b.MaxDelay
	}
	return delay
}
