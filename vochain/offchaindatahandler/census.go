package offchaindatahandler

import (
	"errors"
	"strings"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/log"
)

// importExternalCensus imports a census from a remote URI into the censusDB storage.
func (d *OffChainDataHandler) importExternalCensus(uri string, data []byte) {
	if len(data) == 0 {
		log.Warnf("cannot import empty census from %s", uri)
		return
	}
	if err := d.census.ImportTreeAsPublic(data); err != nil {
		if !errors.Is(err, censusdb.ErrCensusAlreadyExists) {
			log.Warnf("cannot import census from %s: %v", uri, err)
		}
	}
}

// enqueueOffchainCensus enqueue a census for download and imports it into the censusDB storage.
func (d *OffChainDataHandler) enqueueOffchainCensus(root, uri string) {
	if !strings.HasPrefix(uri, d.storage.RemoteStorage.URIprefix()) ||
		len(root) == 0 || len(uri) <= len(d.storage.RemoteStorage.URIprefix()) {
		log.Warnf("census URI or root not valid: (%s,%s)", uri, root)
		return
	}
	d.storage.AddToQueue(uri, d.importExternalCensus, true)
}
