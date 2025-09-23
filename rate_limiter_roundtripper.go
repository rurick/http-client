package httpclient

import (
	"net/http"
)

// RateLimiterRoundTripper обертка для RoundTripper с rate limiting.
type RateLimiterRoundTripper struct {
	base    http.RoundTripper
	config  RateLimiterConfig
	limiter RateLimiter // глобальный лимитер
}

// NewRateLimiterRoundTripper создает новый RoundTripper с rate limiting.
func NewRateLimiterRoundTripper(base http.RoundTripper, config RateLimiterConfig) *RateLimiterRoundTripper {
	config = config.withDefaults()
	return &RateLimiterRoundTripper{
		base:    base,
		config:  config,
		limiter: NewTokenBucketLimiter(config.RequestsPerSecond, config.BurstCapacity),
	}
}

// RoundTrip выполняет HTTP запрос с учетом rate limiting.
func (rt *RateLimiterRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Ждем доступности токена.
	if err := rt.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}

	// Выполняем запрос через базовый RoundTripper.
	return rt.base.RoundTrip(req)
}
