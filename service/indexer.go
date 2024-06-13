package service

import (
	"path/filepath"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/snapshot"
	"go.vocdoni.io/dvote/vochain/indexer"
)

// VochainIndexer creates the vochain indexer service.
func (vs *VocdoniService) VochainIndexer() error {
	log.Info("creating vochain indexer service")
	var err error
	vs.Indexer, err = indexer.New(vs.App, indexer.Options{
		DataDir:           filepath.Join(vs.Config.DataDir, "indexer"),
		IgnoreLiveResults: vs.Config.Indexer.IgnoreLiveResults,
		// During StateSync, IndexerDB will be restored, so enable ExpectBackupRestore in that case
		ExpectBackupRestore: vs.Config.StateSyncEnabled,
	})
	if err != nil {
		return err
	}
	// launch the indexer after sync routine (executed when the blockchain is ready)
	go vs.Indexer.AfterSyncBootstrap(false)

	snapshot.SetFnImportIndexer(vs.Indexer.ImportBackup)
	snapshot.SetFnExportIndexer(vs.Indexer.ExportBackup)

	return nil
}
