package httpclient

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientHeadMethod проверяет выполнение HEAD запросов
func TestClientHeadMethod(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodHead, r.Method)
		w.Header().Set("X-Test-Header", "test-value")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	resp, err := client.Head(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "test-value", resp.Header.Get("X-Test-Header"))
}

// TestClientPutJSONMethod проверяет PUT запросы с JSON
func TestClientPutJSONMethod(t *testing.T) {
	t.Parallel()

	requestData := map[string]interface{}{
		"id":   123,
		"name": "Updated Name",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"updated"}`))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	var result map[string]interface{}
	err = client.PutJSON(context.Background(), server.URL, requestData, &result)
	require.NoError(t, err)

	assert.Equal(t, "updated", result["status"])
}

// TestClientPatchJSONMethod проверяет PATCH запросы с JSON
func TestClientPatchJSONMethod(t *testing.T) {
	t.Parallel()

	requestData := map[string]interface{}{
		"name": "Patched Name",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"patched"}`))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	var result map[string]interface{}
	err = client.PatchJSON(context.Background(), server.URL, requestData, &result)
	require.NoError(t, err)

	assert.Equal(t, "patched", result["status"])
}

// TestClientDeleteJSONMethod проверяет DELETE запросы с JSON
func TestClientDeleteJSONMethod(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"deleted"}`))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	var result map[string]interface{}
	err = client.DeleteJSON(context.Background(), server.URL, &result)
	require.NoError(t, err)

	assert.Equal(t, "deleted", result["status"])
}

// TestClientGetXMLMethod проверяет GET запросы с XML
func TestClientGetXMLMethod(t *testing.T) {
	t.Parallel()

	type TestXMLData struct {
		XMLName xml.Name `xml:"data"`
		Name    string   `xml:"name"`
		Value   string   `xml:"value"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "application/xml", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<data><name>test</name><value>123</value></data>`))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	var result TestXMLData
	err = client.GetXML(context.Background(), server.URL, &result)
	require.NoError(t, err)

	assert.Equal(t, "test", result.Name)
	assert.Equal(t, "123", result.Value)
}

// TestClientPostXMLMethod проверяет POST запросы с XML
func TestClientPostXMLMethod(t *testing.T) {
	t.Parallel()

	type TestXMLData struct {
		XMLName xml.Name `xml:"data"`
		Name    string   `xml:"name"`
		Value   string   `xml:"value"`
	}

	requestData := TestXMLData{
		Name:  "test",
		Value: "456",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/xml", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`<data><name>created</name><value>789</value></data>`))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	var result TestXMLData
	err = client.PostXML(context.Background(), server.URL, requestData, &result)
	require.NoError(t, err)

	assert.Equal(t, "created", result.Name)
	assert.Equal(t, "789", result.Value)
}
