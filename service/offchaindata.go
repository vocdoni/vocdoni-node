package service

import (
	"fmt"
	"path/filepath"
	"time"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/data/downloader"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/snapshot"
	"go.vocdoni.io/dvote/vochain/offchaindatahandler"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
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
	vs.OffChainData = offchaindatahandler.NewOffChainDataHandler(
		vs.App,
		vs.DataDownloader,
		vs.CensusDB,
		vs.Config.SkipPreviousOffchainData,
	)

	snapshot.SetFnImportOffChainData(func(s *state.State) error {
		log.Debugf("importing offchain data after snapshot restore")

		// IPFS is not restored during StateSync, so:
		// * fetch metadata from all accounts
		// * fetch metadata for all elections
		// * fetch censuses from all open elections
		// all of these actions triggers pinning and repopulates IPFS

		// accounts
		accts, err := s.ListAccounts(true)
		if err != nil {
			return fmt.Errorf("couldn't list accounts: %w", err)
		}
		for addr, acct := range accts {
			vs.OffChainData.OnSetAccount(addr.Bytes(), acct)
		}

		// elections
		pids, err := s.ListProcessIDs(true)
		if err != nil {
			return fmt.Errorf("couldn't list process IDs: %w", err)
		}
		for _, pid := range pids {
			p, err := s.Process(pid, true)
			if err != nil {
				log.Errorf("couldn't fetch process %x", pid)
			}
			if !(p.GetStatus() == models.ProcessStatus_READY ||
				p.GetStatus() == models.ProcessStatus_PAUSED) {
				// we want to download only censuses from open elections
				// and skip downloading censuses from past elections, so if not READY or PAUSED
				// set a nil CensusURI before passing to OffChainData.OnProcess()
				p.CensusURI = nil
			}
			vs.OffChainData.OnProcess(p, 0)
		}

		if len(accts) > 0 || len(pids) > 0 {
			if err := vs.OffChainData.Commit(0); err != nil {
				log.Errorw(err, "offchaindata.Commit returned error")
			}
		}

		return nil
	})

	return nil
}
