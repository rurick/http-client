package httpclient

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStreamingResponseHeaderMethod проверяет получение заголовков из streaming response
func TestStreamingResponseHeaderMethod(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Custom":     []string{"test-value"},
		},
	}

	streamResp := NewStreamResponse(resp)

	assert.Equal(t, "application/json", streamResp.Header().Get("Content-Type"))
	assert.Equal(t, "test-value", streamResp.Header().Get("X-Custom"))
}
