package service

import (
	"path/filepath"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/indexer"
)

// VochainIndexer creates the vochain indexer service.
func (vs *VocdoniService) VochainIndexer() error {
	log.Info("creating vochain indexer service")
	var err error
	vs.Indexer, err = indexer.NewIndexer(
		filepath.Join(vs.Config.DataDir, "indexer"),
		vs.App,
		!vs.Config.Indexer.IgnoreLiveResults,
	)
	if err != nil {
		return err
	}
	// launch the indexer after sync routine (executed when the blockchain is ready)
	go vs.Indexer.AfterSyncBootstrap(false)

	if vs.Config.Indexer.ArchiveURL != "" {
		log.Infow("starting archive retrieval", "path", vs.Config.Indexer.ArchiveURL)
		go vs.Indexer.StartArchiveRetrieval(vs.DataDownloader, vs.Config.Indexer.ArchiveURL)
	}

	return nil
}
