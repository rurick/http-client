package httpclient

import (
	"net/http"
	"testing"
	"time"
)

func TestConfigWithDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected Config
	}{
		{
			name:   "zero values should get defaults",
			config: Config{},
			expected: Config{
				Timeout:       5 * time.Second,
				PerTryTimeout: 2 * time.Second,
				Transport:     http.DefaultTransport,
				RetryEnabled:  false,
			},
		},
		{
			name: "custom values should be preserved",
			config: Config{
				Timeout:        10 * time.Second,
				PerTryTimeout:  3 * time.Second,
				TracingEnabled: true,
			},
			expected: Config{
				Timeout:        10 * time.Second,
				PerTryTimeout:  3 * time.Second,
				Transport:      http.DefaultTransport,
				TracingEnabled: true,
			},
		},
		{
			name: "retry enabled should apply retry defaults",
			config: Config{
				RetryEnabled: true,
			},
			expected: Config{
				Timeout:       5 * time.Second,
				PerTryTimeout: 2 * time.Second,
				Transport:     http.DefaultTransport,
				RetryEnabled:  true,
				RetryConfig: RetryConfig{
					MaxAttempts: 3,
					BaseDelay:   100 * time.Millisecond,
					MaxDelay:    5 * time.Second,
					Jitter:      0.2,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.withDefaults()

			if result.Timeout != tt.expected.Timeout {
				t.Errorf("Timeout = %v, want %v", result.Timeout, tt.expected.Timeout)
			}
			if result.PerTryTimeout != tt.expected.PerTryTimeout {
				t.Errorf("PerTryTimeout = %v, want %v", result.PerTryTimeout, tt.expected.PerTryTimeout)
			}
			if result.TracingEnabled != tt.expected.TracingEnabled {
				t.Errorf("TracingEnabled = %v, want %v", result.TracingEnabled, tt.expected.TracingEnabled)
			}
			if result.RetryEnabled != tt.expected.RetryEnabled {
				t.Errorf("RetryEnabled = %v, want %v", result.RetryEnabled, tt.expected.RetryEnabled)
			}

			if tt.expected.RetryEnabled {
				if result.RetryConfig.MaxAttempts != tt.expected.RetryConfig.MaxAttempts {
					t.Errorf("MaxAttempts = %v, want %v", result.RetryConfig.MaxAttempts, tt.expected.RetryConfig.MaxAttempts)
				}
			}
		})
	}
}

func TestRetryConfigWithDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   RetryConfig
		expected RetryConfig
	}{
		{
			name:   "zero values should get defaults",
			config: RetryConfig{},
			expected: RetryConfig{
				MaxAttempts: 3,
				BaseDelay:   100 * time.Millisecond,
				MaxDelay:    2 * time.Second,
				Jitter:      0.2,
			},
		},
		{
			name: "custom values should be preserved",
			config: RetryConfig{
				MaxAttempts: 5,
				BaseDelay:   200 * time.Millisecond,
				MaxDelay:    10 * time.Second,
				Jitter:      0.3,
			},
			expected: RetryConfig{
				MaxAttempts: 5,
				BaseDelay:   200 * time.Millisecond,
				MaxDelay:    10 * time.Second,
				Jitter:      0.3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.withDefaults()

			if result.MaxAttempts != tt.expected.MaxAttempts {
				t.Errorf("MaxAttempts = %v, want %v", result.MaxAttempts, tt.expected.MaxAttempts)
			}
			if result.BaseDelay != tt.expected.BaseDelay {
				t.Errorf("BaseDelay = %v, want %v", result.BaseDelay, tt.expected.BaseDelay)
			}
			if result.MaxDelay != tt.expected.MaxDelay {
				t.Errorf("MaxDelay = %v, want %v", result.MaxDelay, tt.expected.MaxDelay)
			}
			if result.Jitter != tt.expected.Jitter {
				t.Errorf("Jitter = %v, want %v", result.Jitter, tt.expected.Jitter)
			}
		})
	}
}
