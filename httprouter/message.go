package httprouter

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"go.vocdoni.io/dvote/log"
)

// Message is a wrapper for messages for a RouterNamespace implementation.
// Data is set by the namespace and can be of any type (implementation details
// should be check). In order to send a reply, Context.Send() should be called.
type Message struct {
	Data      interface{}
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
func (h *HTTPContext) Send(msg []byte) error {
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("recovered http send panic: %v", r)
		}
	}()
	defer close(h.sent)
	defer h.Request.Body.Close()

	if h.Request.Context().Err() != nil {
		// The connection was closed, so don't try to write to it.
		return fmt.Errorf("connection is closed")
	}
	h.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg)+1))
	h.Writer.Header().Set("Content-Type", "application/json")
	if _, err := h.Writer.Write(msg); err != nil {
		return err
	}
	// Ensure we end the response with a newline, to be nice.
	_, err := h.Writer.Write([]byte("\n"))
	return err
}
