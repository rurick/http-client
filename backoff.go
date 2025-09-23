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
	const exponentialBase = 2.0
	backoffMultiplier := math.Pow(exponentialBase, float64(attempt-2)) //nolint:mnd // exponential backoff formula
	delay := time.Duration(float64(baseDelay) * backoffMultiplier)

	// Ограничиваем максимальной задержкой
	if delay > maxDelay {
		delay = maxDelay
	}

	// Применяем jitter только если параметры валидны
	if isValidJitter(jitter, delay) {
		delay = applyJitterToDelay(delay, jitter, attempt)
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

// isValidJitter проверяет валидность параметров jitter.
func isValidJitter(jitter float64, delay time.Duration) bool {
	return jitter > 0 && jitter <= 1 && delay > 0
}

// applyJitterToDelay применяет jitter к задержке с детерминированным псевдослучайным значением.
func applyJitterToDelay(delay time.Duration, jitter float64, attempt int) time.Duration {
	jitterRange := time.Duration(float64(delay) * jitter)
	if jitterRange <= 0 {
		return delay
	}

	// Вычисляем детерминированное псевдослучайное смещение
	hash := calculateDeterministicHash(attempt)
	timeComponent := getTimeComponent()
	finalHash := hash + timeComponent

	// Безопасное преобразование jitterRange в uint64
	jitterRangeUint := safeTimeToUint64(jitterRange)
	if jitterRangeUint == 0 {
		return delay
	}

	// Безопасное преобразование результата модуля в time.Duration
	jitterOffset := calculateSafeJitterOffset(finalHash, jitterRangeUint)

	// Применяем jitter симметрично (чётные/нечётные попытки)
	if attempt%2 == 0 {
		return delay + jitterOffset
	}
	return delay - jitterOffset
}

// calculateSafeJitterOffset вычисляет безопасное смещение jitter.
func calculateSafeJitterOffset(finalHash, jitterRangeUint uint64) time.Duration {
	if jitterRangeUint == 0 {
		return 0
	}

	modResult := finalHash % jitterRangeUint
	const maxSafeInt64 = uint64(1<<63 - 1)

	if modResult <= maxSafeInt64 {
		return time.Duration(modResult)
	}

	// Если результат слишком большой, попробуем поделить на два
	halfResult := modResult / 2 //nolint:mnd // safe overflow handling
	if halfResult <= maxSafeInt64 {
		return time.Duration(halfResult)
	}

	// В крайнем случае возвращаем максимальное безопасное значение
	return time.Duration(maxSafeInt64)
}

// calculateDeterministicHash вычисляет детерминированный хэш на основе attempt.
func calculateDeterministicHash(attempt int) uint64 {
	// Константа для хэш-функции (большое простое число)
	const hashMultiplier = uint64(2654435761)
	if attempt >= 0 {
		return uint64(attempt) * hashMultiplier
	}
	return hashMultiplier // fallback для отрицательных значений
}

// getTimeComponent получает компонент времени для псевдослучайности.
func getTimeComponent() uint64 {
	nanoTime := time.Now().UnixNano()
	const timeShiftBits = 20

	// Безопасное преобразование времени с проверкой диапазона
	if nanoTime >= 0 {
		shiftedTime := nanoTime >> timeShiftBits
		if shiftedTime >= 0 && shiftedTime <= (1<<63-1) { // Дополнительная проверка после сдвига
			return uint64(shiftedTime)
		}
	}
	return 0 // fallback для отрицательного времени или переполнения
}

// safeTimeToUint64 безопасно преобразует time.Duration в uint64.
func safeTimeToUint64(d time.Duration) uint64 {
	// Проверяем положительное значение и безопасный диапазон для преобразования
	const maxSafeDuration = time.Duration(1 << 62)
	if d > 0 && d <= maxSafeDuration {
		// Оставляем запас для предотвращения переполнения
		// Преобразовываем через int64 для безопасности
		dInt64 := int64(d)
		if dInt64 >= 0 { // Дополнительная проверка положительности
			return uint64(dInt64)
		}
	}
	return 1 // fallback для предотвращения деления на ноль
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
