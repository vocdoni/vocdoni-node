package offchaindatahandler

import (
	"errors"
	"strings"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/log"
)

// importExternalCensus imports a census from a remote URI into the censusDB storage.
func (c *OffChainDataHandler) importExternalCensus(uri string, data []byte) {
	if len(data) == 0 {
		log.Warnf("cannot import empty census from %s", uri)
		return
	}
	if err := c.census.ImportAsPublic(data); err != nil {
		log.Warnf("cannot import census from %s: %v", uri, err)
	}
}

// enqueueOffchainCensus enqueue a census for download and imports it into the censusDB storage.
func (c *OffChainDataHandler) enqueueOffchainCensus(root, uri string) {
	if !strings.HasPrefix(uri, c.storage.RemoteStorage.URIprefix()) ||
		len(root) == 0 || len(uri) <= len(c.storage.RemoteStorage.URIprefix()) {
		log.Warnf("census URI or root not valid: (%s,%s)", uri, root)
		return
	}
	c.storage.AddToQueue(uri, c.importExternalCensus, true)
}

// importRollingCensus imports a rolling census (zkIndexed) from a remote URI into the censusDB storage.
func (c *OffChainDataHandler) importRollingCensus(pid []byte) {
	rcensus, err := c.vochain.State.DumpRollingCensus(pid)
	if err != nil {
		log.Errorf("cannot dump census with pid %x: %v", pid, err)
		return
	}
	log.Infof("snapshoting rolling census %s", rcensus.CensusID)
	dump, err := censusdb.BuildExportDump(rcensus.DumpRoot, rcensus.DumpData, rcensus.Type, true)
	if err != nil {
		log.Errorf("cannot build census dump for process %x: %v", pid, err)
		return
	}
	if err := c.census.ImportAsPublic(dump); err != nil {
		if errors.Is(err, censusdb.ErrCensusAlreadyExists) {
			// If namespace exists it means the census is already loaded, so
			// no need to show an error message.
			return
		}
		log.Warnf("could not import census with pid %x: %v", pid, err)
	}
}
