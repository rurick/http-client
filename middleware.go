package httpclient

import (
	"context"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MiddlewareChain represents a chain of middleware
type MiddlewareChain struct {
	middlewares []Middleware
}

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain(middlewares ...Middleware) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: middlewares,
	}
}

// Add appends a middleware to the chain
func (mc *MiddlewareChain) Add(middleware Middleware) {
	mc.middlewares = append(mc.middlewares, middleware)
}

// AddAll добавляет несколько middleware в цепочку
func (mc *MiddlewareChain) AddAll(middlewares ...Middleware) {
	if len(middlewares) == 0 {
		return
	}
	mc.middlewares = append(mc.middlewares, middlewares...)
}

// GetMiddlewares возвращает копию списка middleware
func (mc *MiddlewareChain) GetMiddlewares() []Middleware {
	result := make([]Middleware, len(mc.middlewares))
	copy(result, mc.middlewares)
	return result
}

// Execute processes the request through the middleware chain
func (mc *MiddlewareChain) Execute(req *http.Request, finalHandler func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	if len(mc.middlewares) == 0 {
		return finalHandler(req)
	}

	// Build the chain from right to left
	handler := finalHandler
	for i := len(mc.middlewares) - 1; i >= 0; i-- {
		middleware := mc.middlewares[i]
		currentHandler := handler
		handler = func(r *http.Request) (*http.Response, error) {
			return middleware.Process(r, currentHandler)
		}
	}

	return handler(req)
}

// LoggingMiddleware logs HTTP requests and responses
type LoggingMiddleware struct {
	logger *zap.Logger
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(logger *zap.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

// Process implements the Middleware interface
func (lm *LoggingMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	start := time.Now()

	lm.logger.Info("HTTP request started",
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
		zap.String("user_agent", req.UserAgent()),
	)

	resp, err := next(req)

	duration := time.Since(start)

	if err != nil {
		lm.logger.Error("HTTP request failed",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
	} else {
		lm.logger.Info("HTTP request completed",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Int("status_code", resp.StatusCode),
			zap.Duration("duration", duration),
		)
	}

	return resp, err
}

// HeaderMiddleware adds headers to requests
type HeaderMiddleware struct {
	headers map[string]string
}

// NewHeaderMiddleware creates a new header middleware
func NewHeaderMiddleware(headers map[string]string) *HeaderMiddleware {
	return &HeaderMiddleware{
		headers: headers,
	}
}

// Process implements the Middleware interface
func (hm *HeaderMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	// Add headers to the request
	for key, value := range hm.headers {
		req.Header.Set(key, value)
	}

	return next(req)
}

// AuthMiddleware adds authentication to requests
type AuthMiddleware struct {
	authType string
	token    string
}

// NewAuthMiddleware creates a new authentication middleware (generic function)
func NewAuthMiddleware(authType, token string) *AuthMiddleware {
	return &AuthMiddleware{
		authType: authType,
		token:    token,
	}
}

// NewBearerAuthMiddleware creates a new Bearer token authentication middleware
func NewBearerAuthMiddleware(token string) *AuthMiddleware {
	return &AuthMiddleware{
		authType: "Bearer",
		token:    token,
	}
}

// NewBasicAuthMiddleware creates a new Basic authentication middleware
func NewBasicAuthMiddleware(username, password string) *AuthMiddleware {
	return &AuthMiddleware{
		authType: "Basic",
		token:    basicAuth(username, password),
	}
}

// Process implements the Middleware interface
func (am *AuthMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	if am.authType == "Bearer" {
		req.Header.Set("Authorization", "Bearer "+am.token)
	} else if am.authType == "Basic" {
		req.Header.Set("Authorization", "Basic "+am.token)
	}

	return next(req)
}

// TimeoutMiddleware adds timeout to requests
type TimeoutMiddleware struct {
	timeout time.Duration
}

// NewTimeoutMiddleware creates a new timeout middleware
func NewTimeoutMiddleware(timeout time.Duration) *TimeoutMiddleware {
	return &TimeoutMiddleware{
		timeout: timeout,
	}
}

// Process implements the Middleware interface
func (tm *TimeoutMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	ctx := req.Context()
	if tm.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, tm.timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	return next(req)
}

// UserAgentMiddleware sets a custom User-Agent header
type UserAgentMiddleware struct {
	userAgent string
}

// NewUserAgentMiddleware creates a new User-Agent middleware
func NewUserAgentMiddleware(userAgent string) *UserAgentMiddleware {
	return &UserAgentMiddleware{
		userAgent: userAgent,
	}
}

// Process implements the Middleware interface
func (uam *UserAgentMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	req.Header.Set("User-Agent", uam.userAgent)
	return next(req)
}

// RateLimitMiddleware implements rate limiting functionality
type RateLimitMiddleware struct {
	mu                sync.Mutex
	requestsPerSecond int
	lastRequest       time.Time
	tokens            int
	maxTokens         int
}

// NewRateLimitMiddleware creates a new rate limiting middleware using token bucket algorithm
func NewRateLimitMiddleware(requestsPerSecond int) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		requestsPerSecond: requestsPerSecond,
		lastRequest:       time.Now(),
		tokens:            requestsPerSecond,
		maxTokens:         requestsPerSecond,
	}
}

// Process implements the Middleware interface with token bucket rate limiting
func (rlm *RateLimitMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rlm.lastRequest)

	// Add tokens based on elapsed time
	tokensToAdd := int(elapsed.Seconds() * float64(rlm.requestsPerSecond))
	rlm.tokens += tokensToAdd
	if rlm.tokens > rlm.maxTokens {
		rlm.tokens = rlm.maxTokens
	}

	rlm.lastRequest = now

	// Check if we have tokens available
	if rlm.tokens <= 0 {
		// Calculate wait time
		waitTime := time.Second / time.Duration(rlm.requestsPerSecond)
		rlm.mu.Unlock() // Unlock during sleep
		time.Sleep(waitTime)
		rlm.mu.Lock()
	}

	// Consume a token
	if rlm.tokens > 0 {
		rlm.tokens--
	}

	return next(req)
}

// Helper function for basic auth
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64Encode([]byte(auth))
}

// Simple base64 encoding helper using standard library
func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
