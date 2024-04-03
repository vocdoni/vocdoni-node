package offchaindatahandler

import (
	"encoding/hex"
	"fmt"
	"sync"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/data/downloader"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	itemTypeExternalCensus = iota
	itemTypeOrganizationMetadata
	itemTypeElectionMetadata
	itemTypeAccountMetadata
)

type importItem struct {
	itemType   int
	uri        string
	censusRoot string
}

var itemTypesToString = map[int]string{
	itemTypeExternalCensus:       "external census",
	itemTypeOrganizationMetadata: "organization metadata",
	itemTypeElectionMetadata:     "election metadata",
	itemTypeAccountMetadata:      "account metadata",
}

// TBD: A startup process for importing on-going process census
// TBD: a mechanism for removing already finished census?

// OffChainDataHandler is a Vochain event handler aimed to fetch
// offchain data (usually on IPFS).
type OffChainDataHandler struct {
	vochain       *vochain.BaseApplication
	census        *censusdb.CensusDB
	storage       *downloader.Downloader
	queue         []importItem
	queueLock     sync.RWMutex
	importOnlyNew bool
	isSynced      bool
}

// NewOffChainDataHandler creates a new instance of the off chain data downloader daemon.
// It will subscribe to Vochain events and perform data import.
func NewOffChainDataHandler(v *vochain.BaseApplication, d *downloader.Downloader,
	c *censusdb.CensusDB, importOnlyNew bool) *OffChainDataHandler {
	od := OffChainDataHandler{
		vochain:       v,
		census:        c,
		storage:       d,
		importOnlyNew: importOnlyNew,
		queue:         make([]importItem, 0),
	}
	v.State.AddEventListener(&od)
	return &od
}

// Rollback is called when a new block is reverted, so we revert the import actions.
func (d *OffChainDataHandler) Rollback() {
	d.queueLock.Lock()
	d.queue = make([]importItem, 0)
	d.isSynced = d.vochain.IsSynced()
	d.queueLock.Unlock()
}

// Commit is called when a new block is committed, so we execute the import actions
// enqueued by the event handlers (else the queues are reverted by calling Rollback).
func (d *OffChainDataHandler) Commit(_ uint32) error {
	d.queueLock.Lock()
	defer d.queueLock.Unlock()
	for _, item := range d.queue {
		switch item.itemType {
		case itemTypeExternalCensus:
			log.Infow("importing data", "type", itemTypesToString[item.itemType], "uri", item.uri)
			// AddToQueue() writes to a channel that might be full, so we don't want to block the main thread.
			go d.enqueueOffchainCensus(item.censusRoot, item.uri)
		case itemTypeElectionMetadata, itemTypeAccountMetadata:
			log.Infow("importing data", "type", itemTypesToString[item.itemType], "uri", item.uri)
			go d.enqueueMetadata(item.uri)
		default:
			log.Errorf("unknown import item %d", item.itemType)
		}
	}
	return nil
}

// OnProcess is triggered when a new election is created. It checks if the election contains offchain data
// that needs to be imported and enqueues it for being handled by Commit.
func (d *OffChainDataHandler) OnProcess(p *models.Process, _ int32) {
	d.queueLock.Lock()
	defer d.queueLock.Unlock()
	if d.importOnlyNew && !d.isSynced {
		return
	}
	// enqueue for import election metadata information
	if m := p.GetMetadata(); m != "" {
		d.queue = append(d.queue, importItem{
			uri:      m,
			itemType: itemTypeElectionMetadata,
		})
	}
	// enqueue for download external census if needs to be imported
	if state.CensusOrigins[p.CensusOrigin].NeedsDownload && len(p.GetCensusURI()) > 0 {
		d.queue = append(d.queue, importItem{
			censusRoot: util.TrimHex(fmt.Sprintf("%x", p.CensusRoot)),
			uri:        p.GetCensusURI(),
			itemType:   itemTypeExternalCensus,
		})
	}
}

// OnCensusUpdate is triggered when the census is updated during an election.
func (d *OffChainDataHandler) OnCensusUpdate(pid, censusRoot []byte, censusURI string) {
	if d.importOnlyNew && !d.isSynced {
		return
	}
	p, err := d.vochain.State.Process(pid, false)
	if err != nil || p == nil {
		log.Errorw(err, "onCensusUpdate: could get process from state")
		return
	}
	if state.CensusOrigins[p.CensusOrigin].NeedsDownload && len(censusURI) > 0 {
		d.queue = append(d.queue, importItem{
			censusRoot: hex.EncodeToString(censusRoot),
			uri:        censusURI,
			itemType:   itemTypeExternalCensus,
		})
	}

}

// OnProcessesStart is triggered when a process starts. Does nothing.
func (*OffChainDataHandler) OnProcessesStart(_ [][]byte) {
}

// OnSetAccount is triggered when a new account is created or modified. If metadata info is present, it is enqueued.
func (d *OffChainDataHandler) OnSetAccount(_ []byte, account *state.Account) {
	d.queueLock.Lock()
	defer d.queueLock.Unlock()
	if d.importOnlyNew && !d.isSynced {
		return
	}
	// enqueue for import account metadata information
	if m := account.GetInfoURI(); m != "" {
		log.Debugf("adding account info metadata %s to queue", m)
		d.queue = append(d.queue, importItem{
			uri:      m,
			itemType: itemTypeAccountMetadata,
		})
	}
}

// NOT USED but required for implementing the vochain.EventListener interface
func (*OffChainDataHandler) OnCancel(_ []byte, _ int32)                                      {}
func (*OffChainDataHandler) OnVote(_ *state.Vote, _ int32)                                   {}
func (*OffChainDataHandler) OnNewTx(_ *vochaintx.Tx, _ uint32, _ int32)                      {}
func (*OffChainDataHandler) OnBeginBlock(state.BeginBlock)                                   {}
func (*OffChainDataHandler) OnProcessKeys(_ []byte, _ string, _ int32)                       {}
func (*OffChainDataHandler) OnRevealKeys(_ []byte, _ string, _ int32)                        {}
func (*OffChainDataHandler) OnProcessStatusChange(_ []byte, _ models.ProcessStatus, _ int32) {}
func (*OffChainDataHandler) OnTransferTokens(_ *vochaintx.TokenTransfer)                     {}
func (*OffChainDataHandler) OnProcessResults(_ []byte, _ *models.ProcessResult, _ int32)     {}
func (*OffChainDataHandler) OnSpendTokens(_ []byte, _ models.TxType, _ uint64, _ string)     {}
