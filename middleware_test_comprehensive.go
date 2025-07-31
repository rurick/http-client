package httpclient

import (
        "net/http"
        "net/http/httptest"
        "sync"
        "testing"
        "time"

        "github.com/stretchr/testify/assert"
        "github.com/stretchr/testify/require"
        "go.uber.org/zap"
)

// TestMiddlewareChain_Empty проверяет пустую цепочку middleware
func TestMiddlewareChain_Empty(t *testing.T) {
        chain := NewMiddlewareChain()
        
        finalHandlerCalled := false
        finalHandler := func(req *http.Request) (*http.Response, error) {
                finalHandlerCalled = true
                return &http.Response{StatusCode: 200}, nil
        }
        
        req := httptest.NewRequest("GET", "http://example.com", nil)
        resp, err := chain.Execute(req, finalHandler)
        
        assert.NoError(t, err)
        assert.NotNil(t, resp)
        assert.Equal(t, 200, resp.StatusCode)
        assert.True(t, finalHandlerCalled)
}

// TestMiddlewareChain_AddAll проверяет добавление нескольких middleware
func TestMiddlewareChain_AddAll(t *testing.T) {
        chain := NewMiddlewareChain()
        
        middleware1 := NewHeaderMiddleware(map[string]string{"X-Test-1": "value1"})
        middleware2 := NewHeaderMiddleware(map[string]string{"X-Test-2": "value2"})
        
        chain.AddAll(middleware1, middleware2)
        
        middlewares := chain.GetMiddlewares()
        assert.Len(t, middlewares, 2)
}

// TestMiddlewareChain_AddAllEmpty проверяет добавление пустого списка middleware
func TestMiddlewareChain_AddAllEmpty(t *testing.T) {
        chain := NewMiddlewareChain()
        chain.AddAll() // Пустой список
        
        middlewares := chain.GetMiddlewares()
        assert.Len(t, middlewares, 0)
}

// TestHeaderMiddleware_Process проверяет обработку заголовков
func TestHeaderMiddleware_Process(t *testing.T) {
        headers := map[string]string{
                "X-Custom-Header": "custom-value",
                "X-API-Version":   "v1",
        }
        
        middleware := NewHeaderMiddleware(headers)
        
        finalHandler := func(req *http.Request) (*http.Response, error) {
                assert.Equal(t, "custom-value", req.Header.Get("X-Custom-Header"))
                assert.Equal(t, "v1", req.Header.Get("X-API-Version"))
                return &http.Response{StatusCode: 200}, nil
        }
        
        req := httptest.NewRequest("GET", "http://example.com", nil)
        resp, err := middleware.Process(req, finalHandler)
        
        assert.NoError(t, err)
        assert.Equal(t, 200, resp.StatusCode)
}

// TestLoggingMiddleware_Process проверяет логирование запросов
func TestLoggingMiddleware_Process(t *testing.T) {
        logger, err := zap.NewDevelopment()
        require.NoError(t, err)
        
        middleware := NewLoggingMiddleware(logger)
        
        finalHandler := func(req *http.Request) (*http.Response, error) {
                return &http.Response{StatusCode: 200}, nil
        }
        
        req := httptest.NewRequest("GET", "http://example.com", nil)
        resp, err := middleware.Process(req, finalHandler)
        
        assert.NoError(t, err)
        assert.Equal(t, 200, resp.StatusCode)
}

// TestAuthMiddleware_BasicAuth проверяет Basic Authentication
func TestAuthMiddleware_BasicAuth(t *testing.T) {
        middleware := NewAuthMiddleware("Basic", "user:pass")
        
        finalHandler := func(req *http.Request) (*http.Response, error) {
                auth := req.Header.Get("Authorization")
                assert.Contains(t, auth, "Basic")
                return &http.Response{StatusCode: 200}, nil
        }
        
        req := httptest.NewRequest("GET", "http://example.com", nil)
        resp, err := middleware.Process(req, finalHandler)
        
        assert.NoError(t, err)
        assert.Equal(t, 200, resp.StatusCode)
}

// TestAuthMiddleware_BearerToken проверяет Bearer Token Authentication
func TestAuthMiddleware_BearerToken(t *testing.T) {
        middleware := NewAuthMiddleware("Bearer", "secret-token")
        
        finalHandler := func(req *http.Request) (*http.Response, error) {
                auth := req.Header.Get("Authorization")
                assert.Equal(t, "Bearer secret-token", auth)
                return &http.Response{StatusCode: 200}, nil
        }
        
        req := httptest.NewRequest("GET", "http://example.com", nil)
        resp, err := middleware.Process(req, finalHandler)
        
        assert.NoError(t, err)
        assert.Equal(t, 200, resp.StatusCode)
}

// TestBearerTokenMiddleware проверяет специализированный Bearer Token middleware
func TestBearerTokenMiddleware(t *testing.T) {
        middleware := NewAuthMiddleware("Bearer", "my-token")
        
        finalHandler := func(req *http.Request) (*http.Response, error) {
                auth := req.Header.Get("Authorization")
                assert.Equal(t, "Bearer my-token", auth)
                return &http.Response{StatusCode: 200}, nil
        }
        
        req := httptest.NewRequest("GET", "http://example.com", nil)
        resp, err := middleware.Process(req, finalHandler)
        
        assert.NoError(t, err)
        assert.Equal(t, 200, resp.StatusCode)
}

// TestTimeoutMiddleware_WithTimeout проверяет установку таймаута
func TestTimeoutMiddleware_WithTimeout(t *testing.T) {
        middleware := NewTimeoutMiddleware(100 * time.Millisecond)
        
        finalHandler := func(req *http.Request) (*http.Response, error) {
                // Проверяем что контекст имеет дедлайн
                _, hasDeadline := req.Context().Deadline()
                assert.True(t, hasDeadline)
                return &http.Response{StatusCode: 200}, nil
        }
        
        req := httptest.NewRequest("GET", "http://example.com", nil)
        resp, err := middleware.Process(req, finalHandler)
        
        assert.NoError(t, err)
        assert.Equal(t, 200, resp.StatusCode)
}

// TestTimeoutMiddleware_ZeroTimeout проверяет нулевой таймаут
func TestTimeoutMiddleware_ZeroTimeout(t *testing.T) {
        middleware := NewTimeoutMiddleware(0)
        
        finalHandler := func(req *http.Request) (*http.Response, error) {
                // Контекст не должен измениться при нулевом таймауте
                return &http.Response{StatusCode: 200}, nil
        }
        
        req := httptest.NewRequest("GET", "http://example.com", nil)
        originalCtx := req.Context()
        resp, err := middleware.Process(req, finalHandler)
        
        assert.NoError(t, err)
        assert.Equal(t, 200, resp.StatusCode)
        // Контекст должен остаться тем же (без таймаута)
        assert.Equal(t, originalCtx, req.Context())
}

// TestUserAgentMiddleware_Process проверяет установку User-Agent
func TestUserAgentMiddleware_Process(t *testing.T) {
        middleware := NewUserAgentMiddleware("MyApp/1.0")
        
        finalHandler := func(req *http.Request) (*http.Response, error) {
                userAgent := req.Header.Get("User-Agent")
                assert.Equal(t, "MyApp/1.0", userAgent)
                return &http.Response{StatusCode: 200}, nil
        }
        
        req := httptest.NewRequest("GET", "http://example.com", nil)
        resp, err := middleware.Process(req, finalHandler)
        
        assert.NoError(t, err)
        assert.Equal(t, 200, resp.StatusCode)
}

// TestRateLimitMiddleware_Process проверяет ограничение скорости
func TestRateLimitMiddleware_Process(t *testing.T) {
        // Используем высокий лимит чтобы избежать задержек в тесте
        middleware := NewRateLimitMiddleware(100)
        
        finalHandler := func(req *http.Request) (*http.Response, error) {
                return &http.Response{StatusCode: 200}, nil
        }
        
        req := httptest.NewRequest("GET", "http://example.com", nil)
        
        // Делаем несколько запросов
        for i := 0; i < 5; i++ {
                resp, err := middleware.Process(req, finalHandler)
                assert.NoError(t, err)
                assert.Equal(t, 200, resp.StatusCode)
        }
}

// TestRateLimitMiddleware_Concurrent проверяет конкурентную работу rate limiter
func TestRateLimitMiddleware_Concurrent(t *testing.T) {
        middleware := NewRateLimitMiddleware(10)
        
        finalHandler := func(req *http.Request) (*http.Response, error) {
                return &http.Response{StatusCode: 200}, nil
        }
        
        var wg sync.WaitGroup
        const numRequests = 5
        
        for i := 0; i < numRequests; i++ {
                wg.Add(1)
                go func() {
                        defer wg.Done()
                        req := httptest.NewRequest("GET", "http://example.com", nil)
                        resp, err := middleware.Process(req, finalHandler)
                        assert.NoError(t, err)
                        assert.Equal(t, 200, resp.StatusCode)
                }()
        }
        
        wg.Wait()
}

// TestBasicAuth проверяет хелпер функцию для Basic Auth
func TestBasicAuth(t *testing.T) {
        encoded := basicAuth("user", "password")
        assert.NotEmpty(t, encoded)
        
        // Проверяем что результат корректный
        expected := "dXNlcjpwYXNzd29yZA==" // base64("user:password")
        assert.Equal(t, expected, encoded)
}

// TestMiddlewareChain_ExecutionOrder проверяет порядок выполнения middleware
func TestMiddlewareChain_ExecutionOrder(t *testing.T) {
        var executionOrder []string
        
        // Создаем middleware который записывает порядок выполнения
        createOrderMiddleware := func(name string) Middleware {
                return &orderTestMiddleware{
                        name:           name,
                        executionOrder: &executionOrder,
                }
        }
        
        chain := NewMiddlewareChain(
                createOrderMiddleware("first"),
                createOrderMiddleware("second"),
                createOrderMiddleware("third"),
        )
        
        finalHandler := func(req *http.Request) (*http.Response, error) {
                executionOrder = append(executionOrder, "final")
                return &http.Response{StatusCode: 200}, nil
        }
        
        req := httptest.NewRequest("GET", "http://example.com", nil)
        resp, err := chain.Execute(req, finalHandler)
        
        assert.NoError(t, err)
        assert.Equal(t, 200, resp.StatusCode)
        assert.Equal(t, []string{"first", "second", "third", "final"}, executionOrder)
}

// orderTestMiddleware - вспомогательный middleware для тестирования порядка выполнения
type orderTestMiddleware struct {
        name           string
        executionOrder *[]string
}

func (m *orderTestMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
        *m.executionOrder = append(*m.executionOrder, m.name)
        return next(req)
}