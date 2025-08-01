package mock

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

// MockClient implements a mock HTTP client for testing
type MockClient struct {
	responses       map[string]*http.Response
	errors          map[string]error
	requestHistory  []*http.Request
	callCount       map[string]int
	delay           time.Duration
	defaultResponse *http.Response
	defaultError    error
}

// NewMockClient creates a new mock HTTP client
func NewMockClient() *MockClient {
	return &MockClient{
		responses: make(map[string]*http.Response),
		errors:    make(map[string]error),
		callCount: make(map[string]int),
	}
}

// SetResponse sets a mock response for a specific URL
func (mc *MockClient) SetResponse(url string, resp *http.Response) {
	mc.responses[url] = resp
}

// SetError sets a mock error for a specific URL
func (mc *MockClient) SetError(url string, err error) {
	mc.errors[url] = err
}

// SetDelay sets a delay for all requests
func (mc *MockClient) SetDelay(delay time.Duration) {
	mc.delay = delay
}

// SetDefaultResponse sets a default response for unmatched URLs
func (mc *MockClient) SetDefaultResponse(resp *http.Response) {
	mc.defaultResponse = resp
}

// SetDefaultError sets a default error for unmatched URLs
func (mc *MockClient) SetDefaultError(err error) {
	mc.defaultError = err
}

// Do implements the HTTPClient interface
func (mc *MockClient) Do(req *http.Request) (*http.Response, error) {
	// Add delay if configured
	if mc.delay > 0 {
		time.Sleep(mc.delay)
	}

	// Record the request
	mc.requestHistory = append(mc.requestHistory, req)

	url := req.URL.String()
	mc.callCount[url]++

	// Check for specific error
	if err, exists := mc.errors[url]; exists {
		return nil, err
	}

	// Check for specific response
	if resp, exists := mc.responses[url]; exists {
		return mc.cloneResponse(resp), nil
	}

	// Return default error if set
	if mc.defaultError != nil {
		return nil, mc.defaultError
	}

	// Return default response if set
	if mc.defaultResponse != nil {
		return mc.cloneResponse(mc.defaultResponse), nil
	}

	// Return 404 by default
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Status:     "404 Not Found",
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString("Not Found")),
		Request:    req,
	}, nil
}

// Get implements the HTTPClient interface
func (mc *MockClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return mc.Do(req)
}

// Post implements the HTTPClient interface
func (mc *MockClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return mc.Do(req)
}

// PostForm implements the HTTPClient interface
func (mc *MockClient) PostForm(url string, data map[string][]string) (*http.Response, error) {
	// Convert map to url.Values and encode
	values := make(map[string][]string)
	for k, v := range data {
		values[k] = v
	}

	body := bytes.NewBufferString("")
	first := true
	for key, vals := range values {
		for _, val := range vals {
			if !first {
				_, _ = body.WriteString("&")
			}
			_, _ = body.WriteString(key + "=" + val)
			first = false
		}
	}

	return mc.Post(url, "application/x-www-form-urlencoded", body)
}

// Head implements the HTTPClient interface
func (mc *MockClient) Head(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	return mc.Do(req)
}

// DoCtx implements CtxHTTPClient interface
func (mc *MockClient) DoCtx(ctx context.Context, req *http.Request) (*http.Response, error) {
	return mc.Do(req.WithContext(ctx))
}

// GetCtx implements CtxHTTPClient interface
func (mc *MockClient) GetCtx(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return mc.Do(req)
}

// PostCtx implements CtxHTTPClient interface
func (mc *MockClient) PostCtx(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return mc.Do(req)
}

// PostFormCtx implements CtxHTTPClient interface
func (mc *MockClient) PostFormCtx(ctx context.Context, requestURL string, data map[string][]string) (*http.Response, error) {
	values := make(map[string][]string)
	for k, v := range data {
		values[k] = v
	}

	body := bytes.NewBufferString("")
	first := true
	for key, vals := range values {
		for _, val := range vals {
			if !first {
				_, _ = body.WriteString("&")
			}
			_, _ = body.WriteString(key + "=" + val)
			first = false
		}
	}

	return mc.PostCtx(ctx, requestURL, "application/x-www-form-urlencoded", body)
}

// HeadCtx implements CtxHTTPClient interface
func (mc *MockClient) HeadCtx(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	return mc.Do(req)
}

// GetRequestHistory returns all recorded requests
func (mc *MockClient) GetRequestHistory() []*http.Request {
	return mc.requestHistory
}

// GetCallCount returns the number of calls made to a specific URL
func (mc *MockClient) GetCallCount(url string) int {
	return mc.callCount[url]
}

// Reset clears all mock data
func (mc *MockClient) Reset() {
	mc.responses = make(map[string]*http.Response)
	mc.errors = make(map[string]error)
	mc.requestHistory = nil
	mc.callCount = make(map[string]int)
	mc.delay = 0
	mc.defaultResponse = nil
	mc.defaultError = nil
}

// cloneResponse creates a copy of the response to avoid body reading issues
func (mc *MockClient) cloneResponse(resp *http.Response) *http.Response {
	if resp == nil {
		return nil
	}

	// Read the body if it exists
	var bodyBytes []byte
	if resp.Body != nil {
		bodyBytes, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
	}

	// Create new response with cloned body
	newResp := &http.Response{
		Status:           resp.Status,
		StatusCode:       resp.StatusCode,
		Proto:            resp.Proto,
		ProtoMajor:       resp.ProtoMajor,
		ProtoMinor:       resp.ProtoMinor,
		Header:           make(http.Header),
		ContentLength:    resp.ContentLength,
		TransferEncoding: resp.TransferEncoding,
		Close:            resp.Close,
		Uncompressed:     resp.Uncompressed,
		Trailer:          resp.Trailer,
		Request:          resp.Request,
		TLS:              resp.TLS,
	}

	// Copy headers
	for key, values := range resp.Header {
		newResp.Header[key] = make([]string, len(values))
		copy(newResp.Header[key], values)
	}

	// Set body
	if bodyBytes != nil {
		newResp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		// Restore original body
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	return newResp
}

// MockExtendedClient implements ExtendedHTTPClient for testing
type MockExtendedClient struct {
	*MockClient
	jsonResponses map[string]interface{}
	xmlResponses  map[string]interface{}
	metrics       *httpclient.ClientMetrics
}

// NewMockExtendedClient creates a new mock extended HTTP client
func NewMockExtendedClient() *MockExtendedClient {
	return &MockExtendedClient{
		MockClient:    NewMockClient(),
		jsonResponses: make(map[string]interface{}),
		xmlResponses:  make(map[string]interface{}),
		metrics:       httpclient.NewClientMetrics(),
	}
}

// SetJSONResponse sets a JSON response for a specific URL
func (mec *MockExtendedClient) SetJSONResponse(url string, data interface{}) {
	mec.jsonResponses[url] = data
}

// SetXMLResponse sets an XML response for a specific URL
func (mec *MockExtendedClient) SetXMLResponse(url string, data interface{}) {
	mec.xmlResponses[url] = data
}

// GetJSON implements ExtendedHTTPClient interface
func (mec *MockExtendedClient) GetJSON(_ context.Context, url string, result interface{}) error {
	if data, exists := mec.jsonResponses[url]; exists {
		// Simulate JSON unmarshaling by copying data
		return mec.copyInterface(data, result)
	}
	return mec.defaultError
}

// PostJSON implements ExtendedHTTPClient interface
func (mec *MockExtendedClient) PostJSON(ctx context.Context, url string, _ interface{}, result interface{}) error {
	return mec.GetJSON(ctx, url, result)
}

// PutJSON implements ExtendedHTTPClient interface
func (mec *MockExtendedClient) PutJSON(ctx context.Context, url string, _ interface{}, result interface{}) error {
	return mec.GetJSON(ctx, url, result)
}

// PatchJSON implements ExtendedHTTPClient interface
func (mec *MockExtendedClient) PatchJSON(ctx context.Context, url string, _ interface{}, result interface{}) error {
	return mec.GetJSON(ctx, url, result)
}

// DeleteJSON implements ExtendedHTTPClient interface
func (mec *MockExtendedClient) DeleteJSON(ctx context.Context, url string, result interface{}) error {
	return mec.GetJSON(ctx, url, result)
}

// GetXML implements ExtendedHTTPClient interface
func (mec *MockExtendedClient) GetXML(_ context.Context, url string, result interface{}) error {
	if data, exists := mec.xmlResponses[url]; exists {
		return mec.copyInterface(data, result)
	}
	return mec.defaultError
}

// PostXML implements ExtendedHTTPClient interface
func (mec *MockExtendedClient) PostXML(ctx context.Context, url string, _ interface{}, result interface{}) error {
	return mec.GetXML(ctx, url, result)
}

// DoWithContext implements ExtendedHTTPClient interface
func (mec *MockExtendedClient) DoWithContext(_ context.Context, req *http.Request) (*http.Response, error) {
	return mec.Do(req)
}

// GetMetrics implements ExtendedHTTPClient interface
func (mec *MockExtendedClient) GetMetrics() *httpclient.ClientMetrics {
	return mec.metrics
}

// copyInterface is a simple interface copier for testing
func (mec *MockExtendedClient) copyInterface(_, _ interface{}) error {
	// This is a simplified implementation for testing
	// In real scenarios, you'd use JSON marshal/unmarshal or reflection
	return nil
}
