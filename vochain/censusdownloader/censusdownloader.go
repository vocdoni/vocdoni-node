package censusdownloader

import (
	"strings"
	"sync"

	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
)

// TBD: A startup process for importing on-going process census
// TBD: a mechanism for removing already finished census?

// CensusDownloader is a Vochain event handler aimed to fetch and import census
// when a new process is created
type CensusDownloader struct {
	vochain       *vochain.BaseApplication
	census        *census.Manager
	queue         map[string]string
	queueLock     sync.RWMutex
	importOnlyNew bool
	isFastSync    bool
}

// NewCensusDownloader creates a new instance of the census downloader daemon.
// It will subscribe to Vochain events and perform the census import.
func NewCensusDownloader(v *vochain.BaseApplication,
	c *census.Manager, importOnlyNew bool) *CensusDownloader {
	cd := CensusDownloader{vochain: v, census: c, importOnlyNew: importOnlyNew}
	cd.queue = make(map[string]string)
	v.State.AddEventListener(&cd)
	return &cd
}

// importcensus imports remote census
func (c *CensusDownloader) importCensus(root, uri string) {
	if !strings.HasPrefix(uri, c.census.RemoteStorage.URIprefix()) ||
		len(root) == 0 || len(uri) <= len(c.census.RemoteStorage.URIprefix()) {
		log.Warnf("census URI or root not valid: (%s,%s)", uri, root)
		return
	}
	go c.census.AddToImportQueue(root, uri)
}

func (c *CensusDownloader) Rollback() {
	c.queueLock.Lock()
	c.queue = make(map[string]string)
	c.isFastSync = c.vochain.IsSynchronizing()
	c.queueLock.Unlock()
}

func (c *CensusDownloader) Commit(height uint32) error {
	c.queueLock.Lock()
	defer c.queueLock.Unlock()
	for k, v := range c.queue {
		log.Infof("importing remote census %s", v)
		c.importCensus(k, v)
	}
	return nil
}

func (c *CensusDownloader) OnProcess(pid, eid []byte, censusRoot, censusURI string, txindex int32) {
	censusRoot = util.TrimHex(censusRoot)
	c.queueLock.Lock()
	defer c.queueLock.Unlock()
	if !c.importOnlyNew || !c.isFastSync {
		p, err := c.vochain.State.Process(pid, false)
		if err != nil || p == nil {
			log.Errorf("censusDownloader cannot get process from state: (%v)", err)
			return
		}
		if vochain.CensusOrigins[p.CensusOrigin].NeedsDownload && len(censusURI) > 0 {
			c.queue[censusRoot] = censusURI
		}
	}
}

// NOT USED but required for implementing the vochain.EventListener interface
func (c *CensusDownloader) OnCancel(pid []byte, txindex int32)                  {}
func (c *CensusDownloader) OnVote(v *models.Vote, txindex int32)                {}
func (c *CensusDownloader) OnNewTx(blockHeight uint32, txIndex int32)           {}
func (c *CensusDownloader) OnProcessKeys(pid []byte, pub string, txindex int32) {}
func (c *CensusDownloader) OnRevealKeys(pid []byte, priv string, txindex int32) {}
func (c *CensusDownloader) OnProcessStatusChange(pid []byte,
	status models.ProcessStatus, txindex int32) {
}

func (c *CensusDownloader) OnProcessResults(pid []byte,
	results *models.ProcessResult, txindex int32) error {
	return nil
}

func (s *CensusDownloader) OnProcessesStart(pids [][]byte) {
	for _, pid := range pids {
		process, err := s.vochain.State.Process(pid, true)
		if err != nil {
			log.Fatalf("cannot find process with pid %x: %v", pid, err)
		}
		if process.Mode.PreRegister && process.EnvelopeType.Anonymous {
			census, err := s.vochain.State.DumpRollingCensus(pid)
			if err != nil {
				log.Fatalf("cannot dump census with pid %x: %v", pid, err)
			}
			if _, err := s.census.ImportDump(census.CensusID,
				census.Type, census.DumpRoot, census.DumpData); err != nil {
				log.Fatalf("cannot import census with pid %x: %v", pid, err)
			}
		}
	}
}
