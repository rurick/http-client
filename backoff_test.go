package httpclient

import (
	"fmt"
	"testing"
	"time"
)

func TestCalculateBackoffDelay(t *testing.T) {
	testCases := []struct {
		name       string
		attempt    int
		baseDelay  time.Duration
		maxDelay   time.Duration
		jitter     float64
		expectZero bool
		expectMax  bool
	}{
		{
			name:       "first attempt",
			attempt:    1,
			baseDelay:  100 * time.Millisecond,
			maxDelay:   2 * time.Second,
			jitter:     0.2,
			expectZero: true,
		},
		{
			name:      "second attempt",
			attempt:   2,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  2 * time.Second,
			jitter:    0,
		},
		{
			name:      "third attempt",
			attempt:   3,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  2 * time.Second,
			jitter:    0,
		},
		{
			name:      "fourth attempt",
			attempt:   4,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  2 * time.Second,
			jitter:    0,
		},
		{
			name:      "high attempt with max delay",
			attempt:   10,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  1 * time.Second,
			jitter:    0,
			expectMax: true,
		},
		{
			name:      "with jitter",
			attempt:   3,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  2 * time.Second,
			jitter:    0.5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			delay := CalculateBackoffDelay(tc.attempt, tc.baseDelay, tc.maxDelay, tc.jitter)

			if tc.expectZero && delay != 0 {
				t.Errorf("expected zero delay for first attempt, got %v", delay)
				return
			}

			if tc.expectMax && delay != tc.maxDelay {
				t.Errorf("expected max delay %v, got %v", tc.maxDelay, delay)
				return
			}

			if delay < 0 {
				t.Errorf("delay should not be negative, got %v", delay)
			}

			if delay > tc.maxDelay {
				t.Errorf("delay %v should not exceed max delay %v", delay, tc.maxDelay)
			}

			// Для случаев без jitter проверяем точное значение
			if tc.jitter == 0 && !tc.expectZero && !tc.expectMax {
				expectedMultiplier := 1 << (tc.attempt - 2) // 2^(attempt-2)
				expectedDelay := time.Duration(expectedMultiplier) * tc.baseDelay
				if expectedDelay > tc.maxDelay {
					expectedDelay = tc.maxDelay
				}

				if delay != expectedDelay {
					t.Errorf("expected delay %v, got %v", expectedDelay, delay)
				}
			}
		})
	}
}

func TestCalculateBackoffDelayExponentialGrowth(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	maxDelay := 10 * time.Second

	// Проверяем экспоненциальный рост без jitter
	delays := make([]time.Duration, 5)
	for i := 2; i <= 5; i++ {
		delays[i-2] = CalculateBackoffDelay(i, baseDelay, maxDelay, 0)
	}

	// Каждая следующая задержка должна быть в 2 раза больше предыдущей
	// (до достижения максимума)
	expected := []time.Duration{
		100 * time.Millisecond, // attempt 2: base * 2^0
		200 * time.Millisecond, // attempt 3: base * 2^1
		400 * time.Millisecond, // attempt 4: base * 2^2
		800 * time.Millisecond, // attempt 5: base * 2^3
	}

	for i, expectedDelay := range expected {
		if delays[i] != expectedDelay {
			t.Errorf("attempt %d: expected delay %v, got %v", i+2, expectedDelay, delays[i])
		}
	}
}

func TestCalculateBackoffDelayWithJitter(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	maxDelay := 2 * time.Second
	jitter := 0.2

	// Запускаем много раз и проверяем, что результаты варьируются
	delays := make([]time.Duration, 100)
	for i := 0; i < 100; i++ {
		delays[i] = CalculateBackoffDelay(3, baseDelay, maxDelay, jitter)
	}

	// Проверяем, что есть вариация в результатах (jitter работает)
	hasVariation := false
	firstDelay := delays[0]
	for _, delay := range delays[1:] {
		if delay != firstDelay {
			hasVariation = true
			break
		}
	}

	if !hasVariation {
		t.Error("expected variation in delays due to jitter, but all delays were the same")
	}

	// Все задержки должны быть в разумных пределах
	baseExpected := 200 * time.Millisecond // базовая задержка для 3-й попытки без jitter
	maxExpected := time.Duration(float64(baseExpected) * 1.2)

	for i, delay := range delays {
		if delay < 0 {
			t.Errorf("delay %d should not be negative: %v", i, delay)
		}
		if delay > maxDelay {
			t.Errorf("delay %d should not exceed max delay: %v > %v", i, delay, maxDelay)
		}
		// Jitter может давать значения в широком диапазоне, поэтому проверяем более мягко
		if delay > maxExpected*2 { // допускаем большую вариацию
			t.Errorf("delay %d seems too large: %v", i, delay)
		}
	}
}

func TestCalculateExponentialBackoff(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	maxDelay := 1 * time.Second

	testCases := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 0},
		{2, 100 * time.Millisecond},
		{3, 200 * time.Millisecond},
		{4, 400 * time.Millisecond},
		{5, 800 * time.Millisecond},
		{6, 1 * time.Second},  // ограничено maxDelay
		{10, 1 * time.Second}, // ограничено maxDelay
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("attempt_%d", tc.attempt), func(t *testing.T) {
			delay := CalculateExponentialBackoff(tc.attempt, baseDelay, maxDelay)
			if delay != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, delay)
			}
		})
	}
}

func TestCalculateLinearBackoff(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	maxDelay := 500 * time.Millisecond

	testCases := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 0},
		{2, 100 * time.Millisecond},
		{3, 200 * time.Millisecond},
		{4, 300 * time.Millisecond},
		{5, 400 * time.Millisecond},
		{6, 500 * time.Millisecond},  // ограничено maxDelay
		{10, 500 * time.Millisecond}, // ограничено maxDelay
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("attempt_%d", tc.attempt), func(t *testing.T) {
			delay := CalculateLinearBackoff(tc.attempt, baseDelay, maxDelay)
			if delay != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, delay)
			}
		})
	}
}

func TestCalculateConstantBackoff(t *testing.T) {
	baseDelay := 250 * time.Millisecond

	// Константный backoff всегда должен возвращать одно и то же значение
	for attempt := 1; attempt <= 10; attempt++ {
		delay := CalculateConstantBackoff(baseDelay)
		if delay != baseDelay {
			t.Errorf("attempt %d: expected %v, got %v", attempt, baseDelay, delay)
		}
	}
}

func TestBackoffEdgeCases(t *testing.T) {
	t.Run("zero_base_delay", func(t *testing.T) {
		delay := CalculateBackoffDelay(3, 0, time.Second, 0.2)
		if delay != 0 {
			t.Errorf("expected zero delay when base delay is zero, got %v", delay)
		}
	})

	t.Run("zero_max_delay", func(t *testing.T) {
		delay := CalculateBackoffDelay(3, 100*time.Millisecond, 0, 0.2)
		if delay != 0 {
			t.Errorf("expected zero delay when max delay is zero, got %v", delay)
		}
	})

	t.Run("max_jitter", func(t *testing.T) {
		// С максимальным jitter (1.0) задержка может сильно варьироваться
		baseDelay := 100 * time.Millisecond
		maxDelay := 2 * time.Second

		for i := 0; i < 10; i++ {
			delay := CalculateBackoffDelay(3, baseDelay, maxDelay, 1.0)
			if delay < 0 || delay > maxDelay {
				t.Errorf("delay with max jitter should be between 0 and max delay, got %v", delay)
			}
		}
	})

	t.Run("negative_jitter", func(t *testing.T) {
		// Отрицательный jitter должен обрабатываться корректно
		delay := CalculateBackoffDelay(3, 100*time.Millisecond, time.Second, -0.5)
		if delay < 0 {
			t.Errorf("delay should not be negative even with negative jitter, got %v", delay)
		}
	})
}
