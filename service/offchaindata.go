package service

import (
	"io"
	"path/filepath"
	"time"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/data/downloader"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/snapshot"
	"go.vocdoni.io/dvote/vochain/offchaindatahandler"
)

// OffChainDataHandler creates the offchain data downloader handler service and a censusDB.
func (vs *VocdoniService) OffChainDataHandler() error {
	log.Infof("creating offchain data downloader service")
	if vs.DataDownloader == nil {
		vs.DataDownloader = downloader.NewDownloader(vs.Storage)
		vs.DataDownloader.Start()
		go vs.DataDownloader.PrintLogInfo(time.Second * 120)
	}
	if vs.CensusDB == nil {
		db, err := metadb.New(vs.Config.DBType, filepath.Join(vs.Config.DataDir, "censusdb"))
		if err != nil {
			return err
		}
		vs.CensusDB = censusdb.NewCensusDB(db)
	}

	snapshot.SetFnImportCensusDB(func(r io.Reader) error {
		log.Debugf("restoring censusdb backup")
		return vs.CensusDB.ImportCensusDB(r)
	})

	snapshot.SetFnExportCensusDB(func(w io.Writer) error {
		log.Debugf("saving censusdb backup")
		return vs.CensusDB.ExportCensusDB(w)
	})

	vs.OffChainData = offchaindatahandler.NewOffChainDataHandler(
		vs.App,
		vs.DataDownloader,
		vs.CensusDB,
		vs.Config.SkipPreviousOffchainData,
	)
	return nil
}
