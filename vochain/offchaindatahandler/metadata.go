package offchaindatahandler

import (
	"strings"

	"go.vocdoni.io/dvote/log"
)

// enqueueMetadata enqueue a election metadata for download.
// (safe for concurrent use, simply pushes an item to a channel)
func (d *OffChainDataHandler) enqueueMetadata(item importItem) {
	if !strings.HasPrefix(item.uri, d.storage.RemoteStorage.URIprefix()) {
		log.Warnf("metadata URI not valid: %s", item.uri)
		return
	}
	d.storage.AddToQueue(item.uri, func(s string, b []byte) {
		log.Infof("metadata downloaded successfully from %s (%d bytes)", s, len(b))
		if item.itemType == itemTypeElectionMetadata && len(item.pid) > 0 {
			d.indexer.UpdateProcessMetadata(d.storage.RemoteStorage, item.pid, item.uri)
		}
	}, true)
}
