package httpclient

import (
	"math"
	"time"
)

// CalculateBackoffDelay вычисляет задержку для exponential backoff с jitter.
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

	// Применяем jitter (детерминированный на основе номера попытки)
	if jitter > 0 && jitter <= 1 && delay > 0 {
		// Full jitter: детерминированное отклонение на основе номера попытки
		jitterRange := time.Duration(float64(delay) * jitter)
		if jitterRange > 0 {
			// Детерминированное "случайное" число на основе attempt
			// Используем простую хэш-функцию для получения псевдослучайного значения
			hash := uint64(attempt)*2654435761 + uint64(time.Now().UnixNano()>>20) // берём старшие биты времени для стабильности
			jitterOffset := time.Duration(hash % uint64(jitterRange))

			// Применяем jitter симметрично (чётные/нечётные попытки)
			if attempt%2 == 0 {
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

// CalculateExponentialBackoff вычисляет exponential backoff без jitter.
func CalculateExponentialBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	return CalculateBackoffDelay(attempt, baseDelay, maxDelay, 0)
}

// CalculateLinearBackoff вычисляет линейную задержку.
func CalculateLinearBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	delay := time.Duration(attempt-1) * baseDelay
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// CalculateConstantBackoff возвращает константную задержку.
func CalculateConstantBackoff(baseDelay time.Duration) time.Duration {
	return baseDelay
}
