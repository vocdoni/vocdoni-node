package httprouter

import (
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"go.vocdoni.io/dvote/log"
)

// Message is a wrapper for messages for a RouterNamespace implementation.
// Data is set by the namespace and can be of any type (implementation details
// should be checked). In order to send a reply, Context.Send() should be called.
type Message struct {
	Data      any
	TimeStamp time.Time
	Path      []string
	Context   *HTTPContext
}

// HTTPContext is the Context for an HTTP request.
type HTTPContext struct {
	writer  http.ResponseWriter
	Request *http.Request

	contentType string
	sent        chan struct{}
}

// SetResponseContentType sets the content type for the response (the default content type is used if not defined).
func (h *HTTPContext) SetResponseContentType(contentType string) {
	h.contentType = contentType
}

// SetHeader sets a header in the response. For content type use SetResponseContentType().
func (h *HTTPContext) SetHeader(key, value string) {
	h.writer.Header().Set(key, value)
}

// URLParam is a wrapper around go-chi to get a URL parameter (specified in the path pattern as {key})
func (h *HTTPContext) URLParam(key string) string {
	return chi.URLParam(h.Request, key)
}

// Send replies the request with the provided message.
func (h *HTTPContext) Send(msg []byte, httpStatusCode int) error {
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("recovered http send panic: %v", r)
		}
	}()
	defer close(h.sent)

	if httpStatusCode < 100 || httpStatusCode >= 600 {
		return fmt.Errorf("http status code %d not supported", httpStatusCode)
	}
	if h.Request.Context().Err() != nil {
		// The connection was closed, so don't try to write to it.
		return fmt.Errorf("connection is closed")
	}
	// Set the content type if not set to default application/json
	if h.contentType == "" {
		h.writer.Header().Set("Content-Type", DefaultContentType)
	} else {
		h.writer.Header().Set("Content-Type", h.contentType)
	}

	// Special handling for no content, reset content, and not modified.
	if httpStatusCode == http.StatusNoContent ||
		httpStatusCode == http.StatusResetContent ||
		httpStatusCode == http.StatusNotModified ||
		(httpStatusCode >= 100 && httpStatusCode < 200) {
		// Don't set Content-Length, don't try to write a body.
		h.writer.WriteHeader(httpStatusCode)
		log.Debugw("http response", "status", httpStatusCode)
		return nil
	}

	// Content length will be message length plus newline character
	h.writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg)+1))
	h.writer.WriteHeader(httpStatusCode)

	// Log the response, but only if it's not binary data.
	var isBinaryData bool
	if len(msg) > 32 {
		isBinaryData = !utf8.Valid(msg[:32])
	} else {
		isBinaryData = !utf8.Valid(msg)
	}
	if !isBinaryData {
		log.Debugw("http response", "size", len(msg), "status", httpStatusCode, "data", func() string {
			if len(msg) > 512 {
				return string(msg[:512]) + "..."
			}
			return string(msg)
		}())
	} else {
		log.Debugw("http response", "size", len(msg), "status", httpStatusCode, "data", "binary")
	}

	// Write the response body.
	if _, err := h.writer.Write(msg); err != nil {
		return err
	}
	// Ensure we end the response with a newline, to be nice.
	_, err := h.writer.Write([]byte("\n"))
	return err
}
