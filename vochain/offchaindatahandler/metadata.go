package offchaindatahandler

import (
	"strings"

	"go.vocdoni.io/dvote/log"
)

// enqueueMetadata enqueue a election metadata for download.
// (safe for concurrent use, simply pushes an item to a channel)
func (c *OffChainDataHandler) enqueueMetadata(uri string) {
	if !strings.HasPrefix(uri, c.storage.RemoteStorage.URIprefix()) {
		log.Warnf("metadata URI not valid: %s", uri)
		return
	}
	c.storage.AddToQueue(uri, func(s string, b []byte) {
		log.Infof("metadata downloaded successfully from %s (%d bytes)", s, len(b))
	}, true)
}
