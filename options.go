// Package httpclient provides an HTTP client with automatic metrics collection,
// configurable retry mechanisms, and OpenTelemetry integration.
package httpclient

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// RequestOption is a functional option for configuring HTTP requests.
type RequestOption func(*http.Request)

// WithHeader sets a single header in the request.
func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}

// WithHeaders sets multiple headers in the request.
func WithHeaders(headers map[string]string) RequestOption {
	return func(req *http.Request) {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}
}

// WithContentType sets the Content-Type header.
func WithContentType(contentType string) RequestOption {
	return WithHeader("Content-Type", contentType)
}

// WithAuthorization sets the Authorization header.
func WithAuthorization(auth string) RequestOption {
	return WithHeader("Authorization", auth)
}

// WithBearerToken sets the Authorization header with a Bearer token.
func WithBearerToken(token string) RequestOption {
	return WithAuthorization("Bearer " + token)
}

// WithIdempotencyKey sets the Idempotency-Key header to support retry for POST/PATCH requests.
func WithIdempotencyKey(key string) RequestOption {
	return WithHeader("Idempotency-Key", key)
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(userAgent string) RequestOption {
	return WithHeader("User-Agent", userAgent)
}

// WithAccept sets the Accept header.
func WithAccept(accept string) RequestOption {
	return WithHeader("Accept", accept)
}

// applyOptions applies all RequestOption to the request.
func applyOptions(req *http.Request, opts []RequestOption) {
	for _, opt := range opts {
		opt(req)
	}
}

// WithJSONBody sets the request body as JSON encoding of v and sets Content-Type to application/json.
func WithJSONBody(v interface{}) RequestOption {
	return func(req *http.Request) {
		var data []byte
		switch val := v.(type) {
		case string:
			data = []byte(val)
		case []byte:
			data = val
		default:
			dataBytes, err := json.Marshal(v)
			if err != nil {
				// In a real application it's better to return an error, but for compatibility with current API
				// set empty body and add header with error for debugging
				req.Body = io.NopCloser(strings.NewReader(""))
				req.Header.Set("X-JSON-Marshal-Error", err.Error())
				return
			}

			data = dataBytes
		}

		req.Body = io.NopCloser(bytes.NewReader(data))
		req.ContentLength = int64(len(data))
		req.Header.Set("Content-Type", "application/json")
	}
}

// WithFormBody sets the request body as URL-encoded form data and sets
// Content-Type to application/x-www-form-urlencoded.
func WithFormBody(values url.Values) RequestOption {
	return func(req *http.Request) {
		encoded := values.Encode()
		req.Body = io.NopCloser(strings.NewReader(encoded))
		req.ContentLength = int64(len(encoded))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
}

// WithXMLBody sets the request body as XML encoding of v and sets Content-Type to application/xml.
func WithXMLBody(v interface{}) RequestOption {
	return func(req *http.Request) {
		data, err := xml.Marshal(v)
		if err != nil {
			req.Body = io.NopCloser(strings.NewReader(""))
			req.Header.Set("X-XML-Marshal-Error", err.Error())
			return
		}
		req.Body = io.NopCloser(bytes.NewReader(data))
		req.ContentLength = int64(len(data))
		req.Header.Set("Content-Type", "application/xml")
	}
}

// WithTextBody sets the request body as the specified string and sets Content-Type to text/plain.
func WithTextBody(text string) RequestOption {
	return func(req *http.Request) {
		req.Body = io.NopCloser(strings.NewReader(text))
		req.ContentLength = int64(len(text))
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	}
}

// WithRawBody sets the request body from the specified reader without setting Content-Type.
// Useful when full control over the request body is needed.
func WithRawBody(body io.Reader) RequestOption {
	return func(req *http.Request) {
		if body == nil {
			req.Body = http.NoBody
			req.ContentLength = 0
			return
		}

		// Try to determine content length
		switch v := body.(type) {
		case *bytes.Buffer:
			req.ContentLength = int64(v.Len())
		case *bytes.Reader:
			req.ContentLength = int64(v.Len())
		case *strings.Reader:
			req.ContentLength = int64(v.Len())
		default:
			req.ContentLength = -1 // unknown length
		}

		rc, ok := body.(io.ReadCloser)
		if !ok {
			rc = io.NopCloser(body)
		}
		req.Body = rc
	}
}

// WithMultipartFormData creates a multipart form data request body.
// Note: this is a simplified version. For files, use a specialized multipart builder.
func WithMultipartFormData(fields map[string]string, boundary string) RequestOption {
	return func(req *http.Request) {
		var buf bytes.Buffer

		for key, value := range fields {
			fmt.Fprintf(&buf, "--%s\r\n", boundary)
			fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"%s\"\r\n\r\n", key)
			fmt.Fprintf(&buf, "%s\r\n", value)
		}
		fmt.Fprintf(&buf, "--%s--\r\n", boundary)

		req.Body = io.NopCloser(&buf)
		req.ContentLength = int64(buf.Len())
		req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))
	}
}
