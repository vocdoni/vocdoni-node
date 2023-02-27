package offchaindatahandler

import (
	"errors"
	"strings"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
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
// TODO: Adapt this function to a non-indexed census.
func (d *OffChainDataHandler) importRollingCensus(pid []byte) {
	rcensus, err := d.vochain.State.DumpRollingCensus(pid)
	if err != nil {
		log.Errorf("cannot dump census with pid %x: %v", pid, err)
		return
	}
	maxLevels := censustree.DefaultMaxLevels
	if rcensus.Type == models.Census_ARBO_POSEIDON {
		maxLevels = d.vochain.TransactionHandler.ZkCircuit.Config.Levels
	}
	log.Infof("snapshoting rolling census %s", rcensus.CensusID)
	dump, err := censusdb.BuildExportDump(rcensus.DumpRoot, rcensus.DumpData, rcensus.Type, maxLevels)
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
