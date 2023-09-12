package httprouter

import (
	"fmt"
	"net/http"
	"time"

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
	Writer  http.ResponseWriter
	Request *http.Request

	sent chan struct{}
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
	defer h.Request.Body.Close()

	if httpStatusCode < 100 || httpStatusCode >= 600 {
		return fmt.Errorf("http status code %d not supported", httpStatusCode)
	}
	if h.Request.Context().Err() != nil {
		// The connection was closed, so don't try to write to it.
		return fmt.Errorf("connection is closed")
	}
	h.Writer.Header().Set("Content-Type", "application/json")

	if httpStatusCode == http.StatusNoContent {
		// For 204 status, don't set Content-Length, don't try to write a body.
		h.Writer.WriteHeader(httpStatusCode)
		log.Debugf("response: (%d)", httpStatusCode)
		return nil
	}

	// Content length will be message length plus newline character
	h.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg)+1))
	h.Writer.WriteHeader(httpStatusCode)

	log.Debugf("response: (%d) %s", httpStatusCode, msg)

	if _, err := h.Writer.Write(msg); err != nil {
		return err
	}
	// Ensure we end the response with a newline, to be nice.
	_, err := h.Writer.Write([]byte("\n"))
	return err
}
