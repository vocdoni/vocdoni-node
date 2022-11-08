package downloader

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
)

const (
	// ImportQueueRoutines is the number of parallel routines processing the
	// remote file download queue.
	ImportQueueRoutines = 10
	// ImportRetrieveTimeout the maximum duration the import queue will wait
	// for retreiving a remote file.
	ImportRetrieveTimeout = 5 * time.Minute
	// ImportQueueTimeout is the maximum duration the import queue will wait
	// for pining a remote file.
	ImportPinTimeout = 2 * time.Minute
	// MaxFileSize is the maximum size of a file that can be imported.
	MaxFileSize = 100 * 1024 * 1024 // 100MB

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
	addedItems      int32
}

type DownloadItem struct {
	URI      string
	Callback func(URI string, data []byte)
	Pin      bool
}

// NewDownloader returns a new Downloader. After creating a new instance,
// the process should be started by calling "Start()"
func NewDownloader(remoteStorage data.Storage) *Downloader {
	d := &Downloader{
		RemoteStorage: remoteStorage,
		importQueue:   make(chan DownloadItem, importQueueBuffer),
		failedQueue:   make(map[string]*DownloadItem),
	}
	return d
}

// Start starts the import queue daemons. This is a non-blocking method.
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

// PrintLogInfo prints the current status of the downloader. This method is blocking.
func (d *Downloader) PrintLogInfo(period time.Duration) {
	for {
		time.Sleep(period)
		log.Infof("[downloader info] total:%d enqueued:%d retrying:%d",
			d.TotalItemsAdded(),
			d.QueueSize(),
			d.ImportFailedQueueSize(),
		)
	}
}

// Stop stops the import queue daemons.
func (d *Downloader) Stop() {
	d.cancel()
	d.wgQueueDaemons.Wait()
}

// AddToQueue adds a new URI to the queue for being imported remotely. Once
// the file is downloaded, the callback is called with the URI as argument.
func (d *Downloader) AddToQueue(URI string, callback func(string, []byte), pin bool) {
	d.importQueue <- DownloadItem{URI: URI, Callback: callback, Pin: pin}
}

// QueueSize returns the size of the import census queue.
func (d *Downloader) QueueSize() int32 {
	return atomic.LoadInt32(&d.queueSize)
}

// ImportFailedQueueSize is the size of the list of remote census imported that failed.
func (d *Downloader) ImportFailedQueueSize() int {
	d.failedQueueLock.RLock()
	defer d.failedQueueLock.RUnlock()
	return len(d.failedQueue)
}

// TotalItemsAdded is the number of items that has been added to the queue on this instance.
func (d *Downloader) TotalItemsAdded() int32 {
	return atomic.LoadInt32(&d.addedItems)
}

// handleImport fetches and imports a remote file. If the download fails, the file
// is added to a secondary queue for retrying.
func (d *Downloader) handleImport(item *DownloadItem) {
	log.Infof("pining remote file %q", item.URI)
	d.queueAddDelta(1)
	defer d.queueAddDelta(-1)
	ctx, cancel := context.WithTimeout(context.Background(), ImportRetrieveTimeout)
	err := d.RemoteStorage.Pin(ctx, item.URI)
	cancel()
	if err != nil {
		if os.IsTimeout(err) {
			log.Warnf("timeout importing file %q, adding it to failed queue for retry", item.URI)
			d.failedQueueLock.Lock()
			d.failedQueue[item.URI] = item
			d.failedQueueLock.Unlock()
		} else {
			log.Warnf("cannot pin file %q: (%v)", item.URI, err)
		}
		return
	}
	ctx, cancel = context.WithTimeout(context.Background(), ImportPinTimeout)
	defer cancel()
	data, err := d.RemoteStorage.Retrieve(ctx, item.URI, 0)
	if err != nil {
		log.Warnf("could not retrieve file %q: %v", item.URI, err)
	}
	if item.Callback != nil {
		go item.Callback(item.URI, data)
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

// queueAddDelta adds or substracts a delta to the queue size.
func (d *Downloader) queueAddDelta(i int32) {
	atomic.AddInt32(&d.queueSize, i)
	if i > 0 {
		atomic.AddInt32(&d.addedItems, i)
	}
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
		log.Debugf("retrying download %s", cid)
		ctx, cancel := context.WithTimeout(context.Background(), ImportRetrieveTimeout)
		err := d.RemoteStorage.Pin(ctx, strings.TrimPrefix(item.URI, d.RemoteStorage.URIprefix()))
		cancel()
		if err != nil {
			continue
		}
		d.failedQueueLock.Lock()
		delete(d.failedQueue, cid)
		d.failedQueueLock.Unlock()
		//item := item // copy the range variable as it is continuously modified
		ctx, cancel = context.WithTimeout(context.Background(), ImportPinTimeout)
		defer cancel()
		data, err := d.RemoteStorage.Retrieve(ctx, strings.TrimPrefix(item.URI, d.RemoteStorage.URIprefix()), 0)
		if err != nil {
			log.Warnf("could not pin file %q: %v", item.URI, err)
		}
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
