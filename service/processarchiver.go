package service

import (
	"fmt"

	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/processarchive"
)

func (vs *VocdoniService) ProcessArchiver() error {
	// Process Archiver
	if vs.Indexer == nil {
		return fmt.Errorf("process archive needs indexer enabled")
	}
	ipfs, ok := vs.Storage.(*ipfs.Handler)
	if !ok {
		return fmt.Errorf("ipfsStorage is not IPFS")
	}
	log.Infof("starting process archiver on %s", vs.Config.ProcessArchiveDataDir)
	processarchive.NewProcessArchive(
		vs.Indexer,
		ipfs,
		vs.Config.ProcessArchiveDataDir,
		vs.Config.ProcessArchiveKey,
	)
	return nil
}
