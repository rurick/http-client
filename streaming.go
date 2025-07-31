package httpclient

import (
	"io"
	"net/http"
)

// StreamResponseImpl implements the StreamResponse interface
type StreamResponseImpl struct {
	response *http.Response
}

// NewStreamResponse creates a new streaming response wrapper
func NewStreamResponse(resp *http.Response) StreamResponse {
	return &StreamResponseImpl{
		response: resp,
	}
}

// Body returns the response body as a ReadCloser for streaming
func (sr *StreamResponseImpl) Body() io.ReadCloser {
	return sr.response.Body
}

// Header returns the response headers
func (sr *StreamResponseImpl) Header() http.Header {
	return sr.response.Header
}

// StatusCode returns the HTTP status code
func (sr *StreamResponseImpl) StatusCode() int {
	return sr.response.StatusCode
}

// Close closes the response body
func (sr *StreamResponseImpl) Close() error {
	if sr.response.Body != nil {
		return sr.response.Body.Close()
	}
	return nil
}

// StreamingClient provides streaming capabilities for HTTP requests
type StreamingClient struct {
	client HTTPClient
}

// NewStreamingClient creates a new streaming client
func NewStreamingClient(client HTTPClient) *StreamingClient {
	return &StreamingClient{
		client: client,
	}
}

// StreamReader provides a streaming reader for HTTP responses
type StreamReader struct {
	body       io.ReadCloser
	buffer     []byte
	bufferSize int
}

// NewStreamReader creates a new stream reader with a specified buffer size
func NewStreamReader(body io.ReadCloser, bufferSize int) *StreamReader {
	if bufferSize <= 0 {
		bufferSize = 4096 // Default 4KB buffer
	}

	return &StreamReader{
		body:       body,
		buffer:     make([]byte, bufferSize),
		bufferSize: bufferSize,
	}
}

// Read implements the io.Reader interface for streaming reads
func (sr *StreamReader) Read(p []byte) (n int, err error) {
	return sr.body.Read(p)
}

// ReadChunk reads a chunk of data with the configured buffer size
func (sr *StreamReader) ReadChunk() ([]byte, error) {
	n, err := sr.body.Read(sr.buffer)
	if n > 0 {
		// Return a copy of the data to avoid buffer reuse issues
		chunk := make([]byte, n)
		copy(chunk, sr.buffer[:n])
		return chunk, nil
	}
	return nil, err
}

// Close closes the underlying body reader
func (sr *StreamReader) Close() error {
	return sr.body.Close()
}

// StreamWriter provides a streaming writer for HTTP request bodies
type StreamWriter struct {
	writer io.Writer
}

// NewStreamWriter creates a new stream writer
func NewStreamWriter(writer io.Writer) *StreamWriter {
	return &StreamWriter{
		writer: writer,
	}
}

// Write implements the io.Writer interface
func (sw *StreamWriter) Write(p []byte) (n int, err error) {
	return sw.writer.Write(p)
}

// WriteChunk writes a chunk of data
func (sw *StreamWriter) WriteChunk(data []byte) error {
	_, err := sw.writer.Write(data)
	return err
}

// ChunkedReader reads HTTP responses in chunks
type ChunkedReader struct {
	reader  *StreamReader
	chunkCh chan []byte
	errorCh chan error
	done    chan struct{}
}

// NewChunkedReader creates a new chunked reader that reads in background
func NewChunkedReader(body io.ReadCloser, bufferSize int) *ChunkedReader {
	reader := NewStreamReader(body, bufferSize)

	cr := &ChunkedReader{
		reader:  reader,
		chunkCh: make(chan []byte, 10), // Buffer 10 chunks
		errorCh: make(chan error, 1),
		done:    make(chan struct{}),
	}

	// Start reading in background
	go cr.readChunks()

	return cr
}

// readChunks reads chunks in background goroutine
func (cr *ChunkedReader) readChunks() {
	defer close(cr.chunkCh)
	defer close(cr.errorCh)
	defer cr.reader.Close()

	for {
		select {
		case <-cr.done:
			return
		default:
			chunk, err := cr.reader.ReadChunk()
			if err != nil {
				if err != io.EOF {
					cr.errorCh <- err
				}
				return
			}

			if len(chunk) > 0 {
				select {
				case cr.chunkCh <- chunk:
				case <-cr.done:
					return
				}
			}
		}
	}
}

// NextChunk returns the next chunk of data
func (cr *ChunkedReader) NextChunk() ([]byte, error) {
	select {
	case chunk, ok := <-cr.chunkCh:
		if !ok {
			// Check for errors
			select {
			case err := <-cr.errorCh:
				return nil, err
			default:
				return nil, io.EOF
			}
		}
		return chunk, nil
	case err := <-cr.errorCh:
		return nil, err
	}
}

// Close stops the background reading and closes resources
func (cr *ChunkedReader) Close() error {
	close(cr.done)
	return nil
}

// AllChunks returns all remaining chunks as a slice
func (cr *ChunkedReader) AllChunks() ([][]byte, error) {
	var chunks [][]byte

	for {
		chunk, err := cr.NextChunk()
		if err != nil {
			if err == io.EOF {
				break
			}
			return chunks, err
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}
