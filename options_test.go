package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithJSONBody(t *testing.T) {
	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name     string
		input    interface{}
		expected string
		wantErr  bool
	}{
		{
			name:     "valid struct",
			input:    testData{Name: "test", Value: 123},
			expected: `{"name":"test","value":123}`,
		},
		{
			name:     "valid map",
			input:    map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:     "invalid input",
			input:    make(chan int), // channels cannot be marshaled
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://example.com", nil)
			opt := WithJSONBody(tt.input)
			opt(req)

			if tt.wantErr {
				assert.Equal(t, int64(0), req.ContentLength)
				assert.NotEmpty(t, req.Header.Get("X-JSON-Marshal-Error"))
			} else {
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				assert.JSONEq(t, tt.expected, string(body))
				assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
				assert.Equal(t, int64(len(tt.expected)), req.ContentLength)
			}
		})
	}
}

func TestWithFormBody(t *testing.T) {
	values := url.Values{
		"name":  {"John Doe"},
		"email": {"john@example.com"},
		"tags":  {"tag1", "tag2"},
	}

	req := httptest.NewRequest("POST", "http://example.com", nil)
	opt := WithFormBody(values)
	opt(req)

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	parsedValues, err := url.ParseQuery(string(body))
	require.NoError(t, err)

	assert.Equal(t, values, parsedValues)
	assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
	assert.Equal(t, int64(len(values.Encode())), req.ContentLength)
}

func TestWithXMLBody(t *testing.T) {
	type testData struct {
		XMLName xml.Name `xml:"data"`
		Name    string   `xml:"name"`
		Value   int      `xml:"value"`
	}

	tests := []struct {
		name     string
		input    interface{}
		expected string
		wantErr  bool
	}{
		{
			name:     "valid struct",
			input:    testData{Name: "test", Value: 123},
			expected: `<data><name>test</name><value>123</value></data>`,
		},
		{
			name:     "invalid input",
			input:    make(chan int), // channels cannot be marshaled
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://example.com", nil)
			opt := WithXMLBody(tt.input)
			opt(req)

			if tt.wantErr {
				assert.Equal(t, int64(0), req.ContentLength)
				assert.NotEmpty(t, req.Header.Get("X-XML-Marshal-Error"))
			} else {
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, string(body))
				assert.Equal(t, "application/xml", req.Header.Get("Content-Type"))
				assert.Equal(t, int64(len(tt.expected)), req.ContentLength)
			}
		})
	}
}

func TestWithTextBody(t *testing.T) {
	text := "Hello, World!\nThis is a test."

	req := httptest.NewRequest("POST", "http://example.com", nil)
	opt := WithTextBody(text)
	opt(req)

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	assert.Equal(t, text, string(body))
	assert.Equal(t, "text/plain; charset=utf-8", req.Header.Get("Content-Type"))
	assert.Equal(t, int64(len(text)), req.ContentLength)
}

func TestWithRawBody(t *testing.T) {
	tests := []struct {
		name           string
		body           io.Reader
		expectedLength int64
	}{
		{
			name:           "bytes.Buffer",
			body:           bytes.NewBufferString("test data"),
			expectedLength: 9,
		},
		{
			name:           "bytes.Reader",
			body:           bytes.NewReader([]byte("test data")),
			expectedLength: 9,
		},
		{
			name:           "strings.Reader",
			body:           strings.NewReader("test data"),
			expectedLength: 9,
		},
		{
			name:           "custom reader",
			body:           io.LimitReader(strings.NewReader("test data"), 100),
			expectedLength: -1, // unknown length
		},
		{
			name:           "nil body",
			body:           nil,
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://example.com", nil)
			opt := WithRawBody(tt.body)
			opt(req)

			assert.Equal(t, tt.expectedLength, req.ContentLength)

			if tt.body == nil {
				assert.Equal(t, http.NoBody, req.Body)
			} else {
				assert.NotNil(t, req.Body)
			}
		})
	}
}

func TestWithMultipartFormData(t *testing.T) {
	fields := map[string]string{
		"name":  "John Doe",
		"email": "john@example.com",
	}
	boundary := "test-boundary"

	req := httptest.NewRequest("POST", "http://example.com", nil)
	opt := WithMultipartFormData(fields, boundary)
	opt(req)

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	bodyStr := string(body)
	assert.Contains(t, bodyStr, "--test-boundary")
	assert.Contains(t, bodyStr, `Content-Disposition: form-data; name="name"`)
	assert.Contains(t, bodyStr, "John Doe")
	assert.Contains(t, bodyStr, `Content-Disposition: form-data; name="email"`)
	assert.Contains(t, bodyStr, "john@example.com")
	assert.Contains(t, bodyStr, "--test-boundary--")

	assert.Equal(t, "multipart/form-data; boundary=test-boundary", req.Header.Get("Content-Type"))
}

func TestOptionsIntegration(t *testing.T) {
	// Test that options can be combined
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "req-123", r.Header.Get("X-Request-ID"))

		// Check body
		var user User
		err := json.NewDecoder(r.Body).Decode(&user)
		assert.NoError(t, err)
		assert.Equal(t, 1, user.ID)
		assert.Equal(t, "Test User", user.Name)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(Config{}, "test")
	defer client.Close()

	user := User{ID: 1, Name: "Test User"}
	resp, err := client.Post(testContext(t), server.URL, nil,
		WithJSONBody(user),
		WithBearerToken("test-token"),
		WithHeader("X-Request-ID", "req-123"),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Helper function to create test context
func testContext(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}
