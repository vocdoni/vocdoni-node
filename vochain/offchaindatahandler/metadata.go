package offchaindatahandler

import (
	"strings"

	"go.vocdoni.io/dvote/log"
)

// enqueueMetadata enqueue a election metadata for download
func (d *OffChainDataHandler) enqueueMetadata(uri string) {
	if !strings.HasPrefix(uri, d.storage.RemoteStorage.URIprefix()) {
		log.Warnf("metadata URI not valid: %s", uri)
		return
	}
	d.storage.AddToQueue(uri, func(s string, b []byte) {
		log.Infof("metadata downloaded successfully from %s (%d bytes)", s, len(b))
	}, true)
}
