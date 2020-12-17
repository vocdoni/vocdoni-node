package censusdownloader

import (
	"strings"
	"sync"

	"github.com/vocdoni/dvote-protobuf/build/go/models"
	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
)

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
		c.queue[mkroot] = mkuri
	}
}

func (c *CensusDownloader) OnCancel(pid []byte)                                           {}
func (c *CensusDownloader) OnVote(v *models.Vote)                                         {}
func (c *CensusDownloader) OnProcessKeys(pid []byte, pub, com string)                     {}
func (c *CensusDownloader) OnRevealKeys(pid []byte, priv, rev string)                     {}
func (c *CensusDownloader) OnProcessStatusChange(pid []byte, status models.ProcessStatus) {}
