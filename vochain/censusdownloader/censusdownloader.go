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

// CensusOriginsToDownload is the list of census that should be downloaded by the censusdownloader
var CensusOriginsToDownload = map[models.CensusOrigin]bool{
	models.CensusOrigin_OFF_CHAIN_TREE:          true,
	models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED: true,
}

// TBD: A startup process for importing on-going processe census
// TBD: a mechanism for removing alyready finished census?

// CensusDownloader is a Vochain event handler aimed to fetch and import census when a new process is created
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
func NewCensusDownloader(v *vochain.BaseApplication, c *census.Manager, importOnlyNew bool) *CensusDownloader {
	cd := CensusDownloader{vochain: v, census: c, importOnlyNew: importOnlyNew}
	cd.queue = make(map[string]string)
	v.State.AddEventListener(&cd)
	return &cd
}

// importcensus imports remote census
func (c *CensusDownloader) importCensus(root, uri string) {
	if !strings.HasPrefix(uri, c.census.RemoteStorage.URIprefix()) || len(root) == 0 || len(uri) <= len(c.census.RemoteStorage.URIprefix()) {
		log.Warnf("census URI or root not valid: (%s,%s)", uri, root)
		return
	}
	go c.census.AddToImportQueue(root, uri)
}

func (c *CensusDownloader) Rollback() {
	c.queueLock.Lock()
	c.queue = make(map[string]string)
	c.isFastSync = c.vochain.Node.ConsensusReactor().WaitSync()
	c.queueLock.Unlock()
}

func (c *CensusDownloader) Commit(height int64) {
	c.queueLock.Lock()
	defer c.queueLock.Unlock()
	for k, v := range c.queue {
		log.Debugf("importing remote census %s", v)
		c.importCensus(k, v)
	}
}

func (c *CensusDownloader) OnProcess(pid, eid []byte, mkroot, mkuri string) {
	mkroot = util.TrimHex(mkroot)
	c.queueLock.Lock()
	defer c.queueLock.Unlock()
	if !c.importOnlyNew || !c.isFastSync {
		p, err := c.vochain.State.Process(pid, false)
		if err != nil || p == nil {
			log.Errorf("censusDownloader cannot get process from state: (%v)", err)
			return
		}
		if CensusOriginsToDownload[p.CensusOrigin] {
			c.queue[mkroot] = mkuri
		}
	}
}

// NOT USED but required for implementing the interface
func (c *CensusDownloader) OnCancel(pid []byte)                                           {}
func (c *CensusDownloader) OnVote(v *models.Vote)                                         {}
func (c *CensusDownloader) OnProcessKeys(pid []byte, pub, com string)                     {}
func (c *CensusDownloader) OnRevealKeys(pid []byte, priv, rev string)                     {}
func (c *CensusDownloader) OnProcessStatusChange(pid []byte, status models.ProcessStatus) {}
