package httpclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMiddlewareAddAllMethod проверяет метод AddAll
func TestMiddlewareAddAllMethod(t *testing.T) {
	t.Parallel()

	chain := NewMiddlewareChain()

	middleware1 := NewHeaderMiddleware(map[string]string{"X-Test-1": "value1"})
	middleware2 := NewHeaderMiddleware(map[string]string{"X-Test-2": "value2"})

	chain.AddAll(middleware1, middleware2)

	middlewares := chain.GetMiddlewares()
	assert.Len(t, middlewares, 2)
}

// TestMiddlewareGetMiddlewaresMethod проверяет метод GetMiddlewares
func TestMiddlewareGetMiddlewaresMethod(t *testing.T) {
	t.Parallel()

	chain := NewMiddlewareChain()
	middleware := NewHeaderMiddleware(map[string]string{"X-Test": "value"})

	chain.Add(middleware)

	middlewares := chain.GetMiddlewares()
	assert.Len(t, middlewares, 1)
	assert.Equal(t, middleware, middlewares[0])
}
