package census

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

type censusImport struct {
	censusID, censusURI string
}

// importTree adds the raw (uncompressed) []byte tree to the cid namespace
func (m *Manager) importTree(tree []byte, cid string) error {
	var dump CensusDump
	if err := json.Unmarshal(tree, &dump); err != nil {
		return fmt.Errorf("retrieved census does not have a valid format: (%s)", err)
	}
	log.Debugf("retrieved census with rootHash %s and size %d bytes", dump.RootHash, len(tree))
	if fmt.Sprintf("%x", dump.RootHash) != util.TrimHex(cid) {
		return fmt.Errorf("dump root Hash and census ID root hash do not match, aborting import")
	}
	if len(dump.Data) == 0 {
		return fmt.Errorf("no claims found on the retreived census")
	}
	tr, err := m.AddNamespace(cid, []string{})
	if err == ErrNamespaceExist {
		return nil
	} else if err != nil {
		return fmt.Errorf("cannot create new census namespace: (%s)", err)
	}
	err = tr.ImportDump(dump.Data)
	if err != nil {
		return fmt.Errorf("error importing dump: %s", err)
	}
	if !bytes.Equal(tr.Root(), dump.RootHash) {
		if err := m.DelNamespace(cid); err != nil {
			log.Error(err)
		}
		return fmt.Errorf("root hash does not match on imported census, aborting import")
	}
	tr.Publish()
	log.Infof("census imported successfully, %d claims. Status is public:%t",
		len(dump.Data), tr.IsPublic())
	return nil
}

// ImportQueueSize returns the size of the import census queue
func (m *Manager) ImportQueueSize() int32 {
	return atomic.LoadInt32(&m.queueSize)
}

func (m *Manager) queueAdd(i int32) {
	atomic.AddInt32(&m.queueSize, i)
}

// ImportFailedQueue is the list of remote census imported that failed. Returns a safe copy.
func (m *Manager) ImportFailedQueue() map[string]string {
	m.failedQueueLock.RLock()
	defer m.failedQueueLock.RUnlock()
	fq := make(map[string]string, len(m.failedQueue))
	for k, v := range m.failedQueue {
		fq[k] = v
	}
	return fq
}

// ImportFailedQueueSize is the size of the list of remote census imported that failed
func (m *Manager) ImportFailedQueueSize() int {
	m.failedQueueLock.RLock()
	defer m.failedQueueLock.RUnlock()
	return len(m.failedQueue)
}

// AddToImportQueue adds a new census to the queue for being imported remotely
func (m *Manager) AddToImportQueue(censusID, censusURI string) {
	m.importQueue <- censusImport{censusID: censusID, censusURI: censusURI}
}

func (m *Manager) importFailedQueueDaemon() {
	log.Infof("starting import failed queue daemon")
	for {
		for cid, uri := range m.ImportFailedQueue() {
			log.Debugf("retrying census import %s %s", cid, uri)
			ctx, cancel := context.WithTimeout(context.Background(), ImportRetrieveTimeout*2)
			censusRaw, err := m.RemoteStorage.Retrieve(ctx, uri[len(m.RemoteStorage.URIprefix()):], 0)
			cancel()
			if err != nil {
				continue
			}
			censusRaw = m.decompressBytes(censusRaw)
			if err := m.importTree(censusRaw, cid); err != nil {
				log.Warnf("cannot import census %s: (%v)", cid, err)
			}
			m.failedQueueLock.Lock()
			delete(m.failedQueue, cid)
			m.failedQueueLock.Unlock()
		}
		time.Sleep(1 * time.Second)
	}
}

// ImportQueueDaemon fetches and imports remote census added via importQueue.
func (m *Manager) importQueueDaemon() {
	for imp := range m.importQueue {
		cid, uri := imp.censusID, imp.censusURI
		// TODO(mvdan): this lock is separate from the one
		// from AddNamespace below. The namespace might appear
		// in between the two pieces of code.
		m.TreesMu.RLock()
		exists := m.Exists(cid)
		m.TreesMu.RUnlock()
		if exists {
			log.Debugf("census %s already exists, skipping", cid)
			continue
		}
		log.Infof("retrieving remote census %s", uri)
		m.queueAdd(1)
		ctx, cancel := context.WithTimeout(context.Background(), ImportRetrieveTimeout)
		censusRaw, err := m.RemoteStorage.Retrieve(ctx, uri[len(m.RemoteStorage.URIprefix()):], 0)
		cancel()
		if err != nil {
			if os.IsTimeout(err) {
				log.Warnf("timeout importing census %s, adding it to failed queue for retry", uri)
				m.failedQueueLock.Lock()
				m.failedQueue[cid] = uri
				m.failedQueueLock.Unlock()
			} else {
				log.Warnf("cannot retrieve census %s: (%s)", cid, err)
			}
			m.queueAdd(-1)
			continue
		}
		censusRaw = m.decompressBytes(censusRaw)
		if err = m.importTree(censusRaw, cid); err != nil {
			log.Warnf("cannot import census %s: (%s)", cid, err)
		}
		m.queueAdd(-1)
	}
}
