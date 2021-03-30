package vochaininfo

import (
	"sync"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain"
)

// VochainInfo stores some metrics and information regarding the Vochain Blockchain
// Avg1/10/60/360 are the block time average for 1 minute, 10 minutes, 1 hour and 6 hours
type VochainInfo struct {
	sync            bool
	height          int64
	voteTreeSize    uint64
	processTreeSize uint64
	mempoolSize     int
	voteCacheSize   int
	votesPerMinute  int
	avg1            int32
	avg10           int32
	avg60           int32
	avg360          int32
	avg1440         int32
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
func (vi *VochainInfo) BlockTimes() *[5]int32 {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return &[5]int32{vi.avg1, vi.avg10, vi.avg60, vi.avg360, vi.avg1440}
}

// Sync returns true if the Vochain is considered up-to-date
// Disclaimer: this method is not 100% accurated. Use it just for non-critical operations
func (vi *VochainInfo) Sync() bool {
	return !vi.vnode.Node.ConsensusReactor().WaitSync()
}

// TreeSizes returns the current size of the ProcessTree, VoteTree and the votes per minute
// ProcessTree: total number of created voting processes in the blockchain
// VoteTree: total number of votes registered in the blockchain
// VotesPerMinute: number of votes included in the last 60 seconds
func (vi *VochainInfo) TreeSizes() (uint64, uint64, int) {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return vi.processTreeSize, vi.voteTreeSize, vi.votesPerMinute
}

// MempoolSize returns the current number of transactions waiting to be validated
func (vi *VochainInfo) MempoolSize() int {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return vi.mempoolSize
}

// VoteCacheSize returns the current number of validated votes waiting to be included in the blockchain
func (vi *VochainInfo) VoteCacheSize() int {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return vi.voteCacheSize
}

// Peers returns the current list of connected peers
func (vi *VochainInfo) Peers() (peers []string) {
	for _, p := range vi.vnode.Node.Switch().Peers().List() {
		peers = append(peers, string(p.ID()))
	}
	return
}

// Start initializes the Vochain statistics recollection
func (vi *VochainInfo) Start(sleepSecs int64) {
	log.Infof("starting vochain info service every %d seconds", sleepSecs)
	var duration time.Duration
	var pheight, height int64
	var h1, h10, h60, h360, h1440 int64
	var n1, n10, n60, n360, n1440, vm int64
	var a1, a10, a60, a360, a1440 int32
	var sync bool
	var oldVoteTreeSize uint64
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
				vm++
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
					a1 = int32((n1 * sleepSecs * 1000) / h1)
					n1 = 0
					h1 = 0
				}
				if sleepSecs*n10 >= 600 && h10 > 0 {
					a10 = int32((n10 * sleepSecs * 1000) / h10)
					n10 = 0
					h10 = 0
				}
				if sleepSecs*n60 >= 3600 && h60 > 0 {
					a60 = int32((n60 * sleepSecs * 1000) / h60)
					n60 = 0
					h60 = 0
				}
				if sleepSecs*n360 >= 21600 && h360 > 0 {
					a360 = int32((n360 * sleepSecs * 1000) / h360)
					n360 = 0
					h360 = 0
				}
				if sleepSecs*n1440 >= 86400 && h1440 > 0 {
					a1440 = int32((n1440 * sleepSecs * 1000) / h1440)
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
			vi.processTreeSize = vi.vnode.State.Store.Tree(vochain.ProcessTree).Count()
			vi.voteTreeSize = vi.vnode.State.Store.Tree(vochain.VoteTree).Count()
			if sleepSecs*vm >= 60 {
				vi.votesPerMinute = int(vi.voteTreeSize) - int(oldVoteTreeSize)
				oldVoteTreeSize = vi.voteTreeSize
				vm = 0
			}
			vi.voteCacheSize = vi.vnode.State.CacheSize()
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
