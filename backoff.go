package httpclient

import (
	"math"
	"time"
)

// CalculateBackoffDelay calculates the delay for exponential backoff with jitter.
func CalculateBackoffDelay(attempt int, baseDelay, maxDelay time.Duration, jitter float64) time.Duration {
	if attempt <= 1 {
		return 0
	}

	// Exponential backoff: baseDelay * 2^(attempt-2)
	const exponentialBase = 2.0
	backoffMultiplier := math.Pow(exponentialBase, float64(attempt-2)) //nolint:mnd // exponential backoff formula
	delay := time.Duration(float64(baseDelay) * backoffMultiplier)

	// Limit by maximum delay
	if delay > maxDelay {
		delay = maxDelay
	}

	// Apply jitter only if parameters are valid
	if isValidJitter(jitter, delay) {
		delay = applyJitterToDelay(delay, jitter, attempt)
	}

	// Ensure delay is not negative and doesn't exceed maximum
	if delay < 0 {
		delay = baseDelay
	}
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// isValidJitter checks jitter parameter validity.
func isValidJitter(jitter float64, delay time.Duration) bool {
	return jitter > 0 && jitter <= 1 && delay > 0
}

// applyJitterToDelay applies jitter to delay with deterministic pseudorandom value.
func applyJitterToDelay(delay time.Duration, jitter float64, attempt int) time.Duration {
	jitterRange := time.Duration(float64(delay) * jitter)
	if jitterRange <= 0 {
		return delay
	}

	// Calculate deterministic pseudorandom offset
	hash := calculateDeterministicHash(attempt)
	timeComponent := getTimeComponent()
	finalHash := hash + timeComponent

	// Safe conversion of jitterRange to uint64
	jitterRangeUint := safeTimeToUint64(jitterRange)
	if jitterRangeUint == 0 {
		return delay
	}

	// Safe conversion of modulo result to time.Duration
	jitterOffset := calculateSafeJitterOffset(finalHash, jitterRangeUint)

	// Apply jitter symmetrically (even/odd attempts)
	if attempt%2 == 0 {
		return delay + jitterOffset
	}
	return delay - jitterOffset
}

// calculateSafeJitterOffset calculates safe jitter offset.
func calculateSafeJitterOffset(finalHash, jitterRangeUint uint64) time.Duration {
	if jitterRangeUint == 0 {
		return 0
	}

	modResult := finalHash % jitterRangeUint
	const maxSafeInt64 = uint64(1<<63 - 1)

	if modResult <= maxSafeInt64 {
		return time.Duration(modResult)
	}

	// If result is too large, try dividing by two
	halfResult := modResult / 2 //nolint:mnd // safe overflow handling
	if halfResult <= maxSafeInt64 {
		return time.Duration(halfResult)
	}

	// As last resort return maximum safe value
	return time.Duration(maxSafeInt64)
}

// calculateDeterministicHash calculates a deterministic hash based on attempt.
func calculateDeterministicHash(attempt int) uint64 {
	// Constant for hash function (large prime number)
	const hashMultiplier = uint64(2654435761)
	if attempt >= 0 {
		return uint64(attempt) * hashMultiplier
	}
	return hashMultiplier // fallback for negative values
}

// getTimeComponent gets time component for pseudorandomness.
func getTimeComponent() uint64 {
	nanoTime := time.Now().UnixNano()
	const timeShiftBits = 20

	// Safe time conversion with range check
	if nanoTime >= 0 {
		shiftedTime := nanoTime >> timeShiftBits
		if shiftedTime >= 0 && shiftedTime <= (1<<63-1) { // Additional check after shift
			return uint64(shiftedTime)
		}
	}
	return 0 // fallback for negative time or overflow
}

// safeTimeToUint64 safely converts time.Duration to uint64.
func safeTimeToUint64(d time.Duration) uint64 {
	// Check positive value and safe range for conversion
	const maxSafeDuration = time.Duration(1 << 62)
	if d > 0 && d <= maxSafeDuration {
		// Leave margin to prevent overflow
		// Convert via int64 for safety
		dInt64 := int64(d)
		if dInt64 >= 0 { // Additional positivity check
			return uint64(dInt64)
		}
	}
	return 1 // fallback to prevent division by zero
}

// CalculateExponentialBackoff calculates exponential backoff without jitter.
func CalculateExponentialBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	return CalculateBackoffDelay(attempt, baseDelay, maxDelay, 0)
}

// CalculateLinearBackoff calculates linear delay.
func CalculateLinearBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	delay := time.Duration(attempt-1) * baseDelay
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// CalculateConstantBackoff returns a constant delay.
func CalculateConstantBackoff(baseDelay time.Duration) time.Duration {
	return baseDelay
}
