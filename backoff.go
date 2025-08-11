package httpclient

import (
	"math"
	"math/rand"
	"time"
)

// CalculateBackoffDelay вычисляет задержку для exponential backoff с full jitter
func CalculateBackoffDelay(attempt int, baseDelay, maxDelay time.Duration, jitter float64) time.Duration {
	if attempt <= 1 {
		return 0
	}

	// Exponential backoff: baseDelay * 2^(attempt-2)
	backoffMultiplier := math.Pow(2, float64(attempt-2))
	delay := time.Duration(float64(baseDelay) * backoffMultiplier)

	// Ограничиваем максимальной задержкой
	if delay > maxDelay {
		delay = maxDelay
	}

	// Применяем full jitter
	if jitter > 0 && jitter <= 1 && delay > 0 {
		// Full jitter: random между 0 и вычисленной задержкой
		jitterRange := time.Duration(float64(delay) * jitter)
		if jitterRange > 0 {
			jitterOffset := time.Duration(rand.Int63n(int64(jitterRange)))

			// Применяем jitter симметрично
			if rand.Float64() < 0.5 {
				delay += jitterOffset
			} else {
				delay -= jitterOffset
			}
		}
	}

	// Убеждаемся, что задержка не отрицательная и не превышает максимум
	if delay < 0 {
		delay = baseDelay
	}
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// CalculateExponentialBackoff вычисляет exponential backoff без jitter
func CalculateExponentialBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	return CalculateBackoffDelay(attempt, baseDelay, maxDelay, 0)
}

// CalculateLinearBackoff вычисляет линейную задержку
func CalculateLinearBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	delay := time.Duration(attempt-1) * baseDelay
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// CalculateConstantBackoff возвращает константную задержку
func CalculateConstantBackoff(baseDelay time.Duration) time.Duration {
	return baseDelay
}
