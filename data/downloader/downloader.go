package downloader

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
)

const (
	// ImportQueueRoutines is the number of parallel routines processing the
	// remote file download queue
	ImportQueueRoutines = 10

	// ImportRetrieveTimeout the maximum duration the import queue will wait
	// for retreiving a remote file
	ImportRetrieveTimeout = 3 * time.Minute

	importQueueBuffer = 32
)

// Downloader is a remote file downloader that uses queues.
type Downloader struct {
	RemoteStorage data.Storage

	importQueue     chan DownloadItem
	queueSize       int32
	failedQueueLock sync.RWMutex
	failedQueue     map[string]*DownloadItem
	cancel          context.CancelFunc
	wgQueueDaemons  sync.WaitGroup
}

type DownloadItem struct {
	URI      string
	AddedAt  time.Time
	Callback func(URI string, data []byte)
}

// NewDownloader returns a new Downloader
func NewDownloader(remoteStorage data.Storage) *Downloader {
	d := &Downloader{
		RemoteStorage: remoteStorage,
		importQueue:   make(chan DownloadItem, importQueueBuffer),
		failedQueue:   make(map[string]*DownloadItem),
	}
	return d
}

// Start starts the import queue daemons
func (d *Downloader) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	for i := 0; i < ImportQueueRoutines; i++ {
		d.wgQueueDaemons.Add(1)
		go d.importQueueDaemon(ctx)
	}
	d.wgQueueDaemons.Add(1)
	go d.importFailedQueueDaemon(ctx)
}

// Stop stops the import queue daemons
func (d *Downloader) Stop() {
	d.cancel()
	d.wgQueueDaemons.Wait()
}

// Close closes the import queue
func (d *Downloader) Close() {
	close(d.importQueue)
}

// AddToImportQueue adds a new URI to the queue for being imported remotely. Once
// the file is downloaded, the callback is called with the URI as argument.
func (d *Downloader) AddToQueue(URI string, callback func(string, []byte)) {
	d.importQueue <- DownloadItem{AddedAt: time.Now(), URI: URI, Callback: callback}
}

// QueueSize returns the size of the import census queue
func (d *Downloader) QueueSize() int32 {
	return atomic.LoadInt32(&d.queueSize)
}

// ImportFailedQueueSize is the size of the list of remote census imported that failed
func (d *Downloader) ImportFailedQueueSize() int {
	d.failedQueueLock.RLock()
	defer d.failedQueueLock.RUnlock()
	return len(d.failedQueue)
}

// queueAddDelta adds or substracts a delta to the queue size.
func (d *Downloader) queueAddDelta(i int32) {
	atomic.AddInt32(&d.queueSize, i)
}

// importFailedQueue is the list of remote census imported that failed. Returns a safe copy.
func (d *Downloader) importFailedQueue() map[string]*DownloadItem {
	d.failedQueueLock.RLock()
	defer d.failedQueueLock.RUnlock()
	fq := make(map[string]*DownloadItem, len(d.failedQueue))
	for k, v := range d.failedQueue {
		fq[k] = v
	}
	return fq
}

// handleImportFailedQueue tries to import files that failed.
func (d *Downloader) handleImportFailedQueue() {
	for cid, item := range d.importFailedQueue() {
		log.Debugf("retrying file download %s %s", cid, item.URI)
		ctx, cancel := context.WithTimeout(context.Background(), ImportRetrieveTimeout*2)
		data, err := d.RemoteStorage.Retrieve(ctx, item.URI[len(d.RemoteStorage.URIprefix()):], 0)
		cancel()
		if err != nil {
			continue
		}
		d.failedQueueLock.Lock()
		delete(d.failedQueue, cid)
		d.failedQueueLock.Unlock()
		if item.Callback != nil {
			go item.Callback(item.URI, data)
		}
	}
}

// importFailedQueueDaemon is a daemon that retries to import files that failed.
func (d *Downloader) importFailedQueueDaemon(ctx context.Context) {
	d.handleImportFailedQueue()
	for {
		select {
		case <-time.NewTimer(1 * time.Second).C:
			d.handleImportFailedQueue()
		case <-ctx.Done():
			d.wgQueueDaemons.Done()
			return
		}
	}
}

// handleImport fetches and imports a remote file. If the download fails, the file
// is added to a secondary queue for retrying.
func (d *Downloader) handleImport(i *DownloadItem) {
	log.Infof("retrieving remote file %q", i.URI)
	d.queueAddDelta(1)
	ctx, cancel := context.WithTimeout(context.Background(), ImportRetrieveTimeout)
	data, err := d.RemoteStorage.Retrieve(ctx, i.URI[len(d.RemoteStorage.URIprefix()):], 0)
	cancel()
	if err != nil {
		if os.IsTimeout(err) {
			log.Warnf("timeout importing file %q, adding it to failed queue for retry", i.URI)
			d.failedQueueLock.Lock()
			d.failedQueue[i.URI] = i
			d.failedQueueLock.Unlock()
		} else {
			log.Warnf("cannot retrieve file %q: (%v)", i.URI, err)
		}
	}
	d.queueAddDelta(-1)
	if i.Callback != nil {
		go i.Callback(i.URI, data)
	}
}

// importQueueDaemon fetches and imports remote files added via importQueue.
func (d *Downloader) importQueueDaemon(ctx context.Context) {
	for {
		select {
		case item := <-d.importQueue:
			d.handleImport(&item)
		case <-ctx.Done():
			d.wgQueueDaemons.Done()
			return
		}
	}
}
