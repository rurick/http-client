package httpclient

import (
	"net/http"
)

// RateLimiterRoundTripper is a wrapper for RoundTripper with rate limiting.
type RateLimiterRoundTripper struct {
	base    http.RoundTripper
	config  RateLimiterConfig
	limiter RateLimiter // global limiter
}

// NewRateLimiterRoundTripper creates a new RoundTripper with rate limiting.
func NewRateLimiterRoundTripper(base http.RoundTripper, config RateLimiterConfig) *RateLimiterRoundTripper {
	config = config.withDefaults()
	return &RateLimiterRoundTripper{
		base:    base,
		config:  config,
		limiter: NewTokenBucketLimiter(config.RequestsPerSecond, config.BurstCapacity),
	}
}

// RoundTrip executes an HTTP request with rate limiting.
func (rt *RateLimiterRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Wait for token availability.
	if err := rt.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}

	// Execute request through base RoundTripper.
	return rt.base.RoundTrip(req)
}
