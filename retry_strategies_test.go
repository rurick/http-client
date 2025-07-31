package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRetryableStatusCodeStrategy проверяет функцию IsRetryableStatusCode
func TestRetryableStatusCodeStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		statusCode int
		expected   bool
	}{
		{500, true},
		{502, true},
		{503, true},
		{504, true},
		{429, true},
		{200, false},
		{404, false},
		{400, false},
	}

	for _, test := range tests {
		strategy := NewExponentialBackoffStrategy(3, time.Millisecond, time.Second)

		// Создаем фиктивный response с нужным статус кодом
		resp := &http.Response{StatusCode: test.statusCode}
		result := strategy.ShouldRetry(resp, nil)

		assert.Equal(t, test.expected, result)
	}
}
