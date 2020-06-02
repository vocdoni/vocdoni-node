package vochaininfo

import (
	"fmt"
	"sync"
	"time"

	"gitlab.com/vocdoni/go-dvote/vochain"
)

// VochainInfo stores some metrics and information regarding the Vochain Blockchain
// Avg1/10/60/360 are the block time average for 1 minute, 10 minutes, 1 hour and 6 hours
type VochainInfo struct {
	sync            bool
	height          int64
	voteTreeSize    int64
	processTreeSize int64
	mempoolSize     int
	avg1            float32
	avg10           float32
	avg60           float32
	avg360          float32
	avg1440         float32
	vnode           *vochain.BaseApplication
	close           chan bool
	lock            sync.RWMutex
}

// NewVochainInfo creates a new VochainInfo type
func NewVochainInfo(node *vochain.BaseApplication) *VochainInfo {
	return &VochainInfo{
		vnode: node,
		close: make(chan bool),
	}
}

// Height returns the current number of blocks of the blockchain
func (vi *VochainInfo) Height() int64 {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return vi.height
}

// BlockTimes returns the average block time for 1, 10, 60, 360 and 1440 minutes
// Value 0 means there is not yet an average
func (vi *VochainInfo) BlockTimes() (float32, float32, float32, float32, float32) {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return vi.avg1, vi.avg10, vi.avg60, vi.avg360, vi.avg1440
}

// Sync returns true if the Vochain is considered up-to-date
// Dislaimer: this method is not 100% accurated. Use it just for non-critical operations
func (vi *VochainInfo) Sync() bool {
	return !vi.vnode.Node.ConsensusReactor().FastSync()
}

// TreeSizes returns the current size of the ProcessTree and VoteTree
// ProcessTree: total number of created voting processes in the blockchain
// VoteTree: total number of votes registered in the blockchain
func (vi *VochainInfo) TreeSizes() (int64, int64) {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return vi.processTreeSize, vi.voteTreeSize
}

// MempoolSize returns the current number of transactions waiting to be validated
func (vi *VochainInfo) MempoolSize() int {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return vi.mempoolSize
}

// Peers returns the current list of connected peers
func (vi *VochainInfo) Peers() (peers []string) {
	for _, p := range vi.vnode.Node.Switch().Peers().List() {
		peers = append(peers, fmt.Sprintf("%s", p.ID()))
	}
	return
}

// Start initializes the Vochain statistics recollection
func (vi *VochainInfo) Start(sleepSecs int64) {
	var duration time.Duration
	var pheight, height int64
	var h1, h10, h60, h360, h1440 int64
	var n1, n10, n60, n360, n1440 int64
	var a1, a10, a60, a360, a1440 float32
	var sync bool
	duration = time.Second * time.Duration(sleepSecs)
	for {
		select {
		case <-time.After(duration):
			// TODO(mvdan): this seems racy too
			if vi.vnode.Node == nil {
				continue
			}
			height = vi.vnode.Node.BlockStore().Height()

			// less than 2s per block it's not real. Consider blockchain is synchcing
			if pheight > 0 && sleepSecs/2 > (height-pheight) {
				sync = true
				n1++
				n10++
				n60++
				n360++
				n1440++
				h1 += (height - pheight)
				h10 += (height - pheight)
				h60 += (height - pheight)
				h360 += (height - pheight)
				h1440 += (height - pheight)

				if sleepSecs*n1 >= 60 && h1 > 0 {
					a1 = float32((n1 * sleepSecs) / h1)
					n1 = 0
					h1 = 0
				}
				if sleepSecs*n10 >= 600 && h10 > 0 {
					a10 = float32((n10 * sleepSecs) / h10)
					n10 = 0
					h10 = 0
				}
				if sleepSecs*n60 >= 3600 && h60 > 0 {
					a60 = float32((n60 * sleepSecs) / h60)
					n60 = 0
					h60 = 0
				}
				if sleepSecs*n360 >= 21600 && h360 > 0 {
					a360 = float32((n360 * sleepSecs) / h360)
					n360 = 0
					h360 = 0
				}
				if sleepSecs*n1440 >= 86400 && h1440 > 0 {
					a1440 = float32((n1440 * sleepSecs) / h1440)
					n1440 = 0
					h1440 = 0
				}
			} else {
				sync = false
			}
			pheight = height

			vi.lock.Lock()
			vi.height = height
			vi.sync = sync
			vi.avg1 = a1
			vi.avg10 = a10
			vi.avg60 = a60
			vi.avg360 = a360
			vi.avg1440 = a1440
			vi.vnode.State.RLock()
			vi.processTreeSize = vi.vnode.State.ProcessTree.Size()
			vi.voteTreeSize = vi.vnode.State.VoteTree.Size()
			vi.vnode.State.RUnlock()
			vi.mempoolSize = vi.vnode.Node.Mempool().Size()
			vi.lock.Unlock()

		case <-vi.close:
			return
		}
	}
}

// Close stops all started goroutines of VochainInfo
func (vi *VochainInfo) Close() {
	close(vi.close)
}
