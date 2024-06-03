package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

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

	snapshot.SetFnImportIndexer(func(r io.Reader) error {
		log.Debugf("restoring indexer backup")

		file, err := os.CreateTemp("", "indexer.sqlite3")
		if err != nil {
			return fmt.Errorf("creating tmpfile: %w", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				log.Warnw("error closing tmpfile", "path", file.Name(), "err", err)
			}
			if err := os.Remove(file.Name()); err != nil {
				log.Warnw("error removing tmpfile", "path", file.Name(), "err", err)
			}
		}()

		if _, err := io.Copy(file, r); err != nil {
			return fmt.Errorf("writing tmpfile: %w", err)
		}

		return vs.Indexer.RestoreBackup(file.Name())
	})

	snapshot.SetFnExportIndexer(func(w io.Writer) error {
		log.Debugf("saving indexer backup")

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		data, err := vs.Indexer.ExportBackupAsBytes(ctx)
		if err != nil {
			return fmt.Errorf("creating indexer backup: %w", err)
		}
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("writing data: %w", err)
		}
		return nil
	})

	return nil
}
