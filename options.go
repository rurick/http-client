// Package httpclient предоставляет HTTP клиент с автоматическим сбором метрик,
// настраиваемыми механизмами retry и интеграцией с OpenTelemetry.
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

// RequestOption — функциональная опция для настройки HTTP запросов.
type RequestOption func(*http.Request)

// WithHeader устанавливает один заголовок в запросе.
func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}

// WithHeaders устанавливает множественные заголовки в запросе.
func WithHeaders(headers map[string]string) RequestOption {
	return func(req *http.Request) {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}
}

// WithContentType устанавливает заголовок Content-Type.
func WithContentType(contentType string) RequestOption {
	return WithHeader("Content-Type", contentType)
}

// WithAuthorization устанавливает заголовок Authorization.
func WithAuthorization(auth string) RequestOption {
	return WithHeader("Authorization", auth)
}

// WithBearerToken устанавливает заголовок Authorization с Bearer токеном.
func WithBearerToken(token string) RequestOption {
	return WithAuthorization("Bearer " + token)
}

// WithIdempotencyKey устанавливает заголовок Idempotency-Key для поддержки retry POST/PATCH запросов.
func WithIdempotencyKey(key string) RequestOption {
	return WithHeader("Idempotency-Key", key)
}

// WithUserAgent устанавливает заголовок User-Agent.
func WithUserAgent(userAgent string) RequestOption {
	return WithHeader("User-Agent", userAgent)
}

// WithAccept устанавливает заголовок Accept.
func WithAccept(accept string) RequestOption {
	return WithHeader("Accept", accept)
}

// applyOptions применяет все RequestOption к запросу.
func applyOptions(req *http.Request, opts []RequestOption) {
	for _, opt := range opts {
		opt(req)
	}
}

// WithJSONBody устанавливает тело запроса как JSON кодировку v и устанавливает Content-Type в application/json.
func WithJSONBody(v interface{}) RequestOption {
	return func(req *http.Request) {
		data, err := json.Marshal(v)
		if err != nil {
			// В реальном приложении лучше возвращать ошибку, но для совместимости с текущим API
			// установим пустое тело и добавим заголовок с ошибкой для отладки
			req.Body = io.NopCloser(strings.NewReader(""))
			req.Header.Set("X-JSON-Marshal-Error", err.Error())
			return
		}
		req.Body = io.NopCloser(bytes.NewReader(data))
		req.ContentLength = int64(len(data))
		req.Header.Set("Content-Type", "application/json")
	}
}

// WithFormBody устанавливает тело запроса как URL-encoded form данные и устанавливает
// Content-Type в application/x-www-form-urlencoded.
func WithFormBody(values url.Values) RequestOption {
	return func(req *http.Request) {
		encoded := values.Encode()
		req.Body = io.NopCloser(strings.NewReader(encoded))
		req.ContentLength = int64(len(encoded))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
}

// WithXMLBody устанавливает тело запроса как XML кодировку v и устанавливает Content-Type в application/xml.
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

// WithTextBody устанавливает тело запроса как указанную строку и устанавливает Content-Type в text/plain.
func WithTextBody(text string) RequestOption {
	return func(req *http.Request) {
		req.Body = io.NopCloser(strings.NewReader(text))
		req.ContentLength = int64(len(text))
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	}
}

// WithRawBody устанавливает тело запроса из указанного reader без установки Content-Type.
// Полезно, когда нужен полный контроль над телом запроса.
func WithRawBody(body io.Reader) RequestOption {
	return func(req *http.Request) {
		if body == nil {
			req.Body = http.NoBody
			req.ContentLength = 0
			return
		}

		// Пытаемся определить длину контента
		switch v := body.(type) {
		case *bytes.Buffer:
			req.ContentLength = int64(v.Len())
		case *bytes.Reader:
			req.ContentLength = int64(v.Len())
		case *strings.Reader:
			req.ContentLength = int64(v.Len())
		default:
			req.ContentLength = -1 // неизвестная длина
		}

		rc, ok := body.(io.ReadCloser)
		if !ok {
			rc = io.NopCloser(body)
		}
		req.Body = rc
	}
}

// WithMultipartFormData создаёт multipart form data тело запроса.
// Примечание: это упрощённая версия. Для файлов используйте специализированный multipart builder.
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
