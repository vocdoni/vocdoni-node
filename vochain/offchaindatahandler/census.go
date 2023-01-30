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
	if err := d.census.ImportAsPublic(data); err != nil {
		log.Warnf("cannot import census from %s: %v", uri, err)
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

// importRollingCensus imports a rolling census (zkIndexed) from a remote URI into the censusDB storage.
func (d *OffChainDataHandler) importRollingCensus(pid []byte) {
	rcensus, err := d.vochain.State.DumpRollingCensus(pid)
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
	if err := d.census.ImportAsPublic(dump); err != nil {
		if errors.Is(err, censusdb.ErrCensusAlreadyExists) {
			// If namespace exists it means the census is already loaded, so
			// no need to show an error message.
			return
		}
		log.Warnf("could not import census with pid %x: %v", pid, err)
	}
}
