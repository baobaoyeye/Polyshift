package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackoffStrategy(t *testing.T) {
	strategy := NewBackoffStrategy(100*time.Millisecond, 2*time.Second)

	// Attempt 0: 100ms
	assert.Equal(t, 100*time.Millisecond, strategy.CalculateDelay(0))

	// Attempt 1: 200ms
	assert.Equal(t, 200*time.Millisecond, strategy.CalculateDelay(1))

	// Attempt 2: 400ms
	assert.Equal(t, 400*time.Millisecond, strategy.CalculateDelay(2))

	// Attempt 3: 800ms
	assert.Equal(t, 800*time.Millisecond, strategy.CalculateDelay(3))

	// Attempt 4: 1600ms
	assert.Equal(t, 1600*time.Millisecond, strategy.CalculateDelay(4))

	// Attempt 5: 3200ms -> capped at 2000ms
	assert.Equal(t, 2*time.Second, strategy.CalculateDelay(5))
}
