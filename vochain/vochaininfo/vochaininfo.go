package vochaininfo

import (
	"sync"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain"
)

// VochainInfo stores some metrics and information regarding the Vochain Blockchain
type VochainInfo struct {
	height         int64
	voteCount      uint64
	processCount   uint64
	mempoolSize    int
	voteCacheSize  int
	votesPerMinute int
	avgBlockTimes  map[time.Duration]time.Duration
	vnode          *vochain.BaseApplication
	close          chan bool
	lock           sync.RWMutex
}

// time windows for calculating a moving average of the block time (avgBlockTimes)
var intervals = [...]time.Duration{
	1 * time.Minute,
	10 * time.Minute,
	1 * time.Hour,
	6 * time.Hour,
	24 * time.Hour,
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

// BlockTimes returns the average block time in milliseconds for 1, 10, 60, 360 and 1440 minutes.
// Value 0 means there is not yet an average
func (vi *VochainInfo) BlockTimes() *[5]int32 {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return &[5]int32{
		int32(vi.avgBlockTimes[intervals[0]].Milliseconds()),
		int32(vi.avgBlockTimes[intervals[1]].Milliseconds()),
		int32(vi.avgBlockTimes[intervals[2]].Milliseconds()),
		int32(vi.avgBlockTimes[intervals[3]].Milliseconds()),
		int32(vi.avgBlockTimes[intervals[4]].Milliseconds()),
	}
}

// HeightTime estimates the UTC time for a future height or returns the
// block timestamp if height is in the past.
func (vi *VochainInfo) HeightTime(height int64) time.Time {
	times := vi.BlockTimes()
	currentHeight := vi.Height()
	diffHeight := height - currentHeight

	// if height is in the past
	if diffHeight < 0 {
		blk := vi.vnode.GetBlockByHeight(height)
		if blk == nil {
			log.Errorf("cannot get block height %d", height)
			return time.Time{}
		}
		return blk.Header.Time
	}

	// else, height is in the future
	getMaxTimeFrom := func(i int) int64 {
		for ; i >= 0; i-- {
			if times[i] != 0 {
				return int64(times[i])
			}
		}
		return 10000 // fallback
	}

	t := int64(0)
	switch {
	// if less than around 15 minutes missing
	case diffHeight < 100:
		t = getMaxTimeFrom(1)
	// if less than around 6 hours missing
	case diffHeight < 1000:
		t = getMaxTimeFrom(3)
	// if more than around 6 hours missing
	case diffHeight >= 1000:
		t = getMaxTimeFrom(4)
	}
	return time.Now().Add(time.Duration(diffHeight*t) * time.Millisecond)
}

// Sync returns true if the Vochain is considered up-to-date
// Disclaimer: this method is not 100% accurate. Use it just for non-critical operations
func (vi *VochainInfo) Sync() bool {
	return !vi.vnode.IsSynchronizing()
}

// TreeSizes returns the current size of the ProcessTree, VoteTree and the votes per minute
// ProcessTree: total number of created voting processes in the blockchain
// VoteTree: total number of votes registered in the blockchain
// VotesPerMinute: number of votes included in the last 60 seconds
func (vi *VochainInfo) TreeSizes() (uint64, uint64, int) {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return vi.processCount, vi.voteCount, vi.votesPerMinute
}

// MempoolSize returns the current number of transactions waiting to be validated
func (vi *VochainInfo) MempoolSize() int {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return vi.mempoolSize
}

// VoteCacheSize returns the current number of validated votes waiting to be
// included in the blockchain
func (vi *VochainInfo) VoteCacheSize() int {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return vi.voteCacheSize
}

/* TENDERMINT 0.35
// NetInfo returns network info (mainly, peers)
func (vi *VochainInfo) NetInfo() (*coretypes.ResultNetInfo, error) {
	if vi.vnode.Node != nil {
		return vi.vnode.Node.NetInfo(context.Background())
	}
	return nil, fmt.Errorf("vochain node not ready")
}

// NPeers returns the whole list of peers, including ID and URL
func (vi *VochainInfo) NPeers() int {
	ni, err := vi.NetInfo()
	if err != nil || ni == nil {
		return 0
	}
	return ni.NPeers
}
*/

// NPeers returns the number of peers connected
func (vi *VochainInfo) NPeers() int {
	peers := []string{}
	if vi.vnode.Node != nil {
		for _, p := range vi.vnode.Node.Switch().Peers().List() {
			peers = append(peers, string(p.ID()))
		}
	}
	return len(peers)
}

// Start initializes the Vochain statistics recollection.
// statsInterval defines how often the (cached) calculations are updated.
func (vi *VochainInfo) Start(statsInterval time.Duration) {
	log.Infof("starting vochain info service every %s", statsInterval)
	var err error

	// avgBlockTimes is updated at intervals different than statsInterval
	vi.avgBlockTimes = make(map[time.Duration]time.Duration)
	for _, i := range intervals {
		go vi.trackAvgBlockTimes(i)
	}

	t := time.NewTicker(statsInterval)
	defer t.Stop()
	for {
		vi.lock.Lock()
		vi.height = int64(vi.vnode.Height())
		vi.processCount, err = vi.vnode.State.CountProcesses(true)
		if err != nil {
			log.Errorf("cannot count processes: %s", err)
		}
		vi.voteCount, err = vi.vnode.State.VoteCount(true)
		if err != nil {
			log.Errorf("cannot access vote count: %s", err)
		}
		vi.voteCacheSize = vi.vnode.State.CacheSize()
		vi.mempoolSize = vi.vnode.MempoolSize()
		vi.lock.Unlock()

		go vi.updateVotesPerMinute()

		select {
		case <-t.C:
			continue
		case <-vi.close:
			return
		}
	}
}

// trackAvgBlockTimes continously calculates the average block time
// during the specified interval and updates vi.avgBlockTimes[interval].
//
// Safe for concurrent use
func (vi *VochainInfo) trackAvgBlockTimes(interval time.Duration) {
	// ticker t runs at shorter intervals for better sampling
	// so (interval / 3) means the avgBlockTime[1m] is calculated/updated every 20 seconds
	// and the avgBlockTime[24h] is updated every 8 hours. This spawns 3x goroutines tho.
	t := time.NewTicker(interval / 3)
	defer t.Stop()
	for {
		go vi.updateAvgBlockTimes(interval)
		select {
		case <-t.C:
			continue
		case <-vi.close:
			return
		}
	}
}

// updateAvgBlockTimes sleeps for interval time and then updates vi.avgBlockTimes[interval].
// (i.e. call it as a goroutine)
func (vi *VochainInfo) updateAvgBlockTimes(interval time.Duration) {
	// during fastsync, stats are totally skewed: don't even try.
	if !vi.Sync() {
		return
	}
	oldHeight := vi.vnode.Height()
	select {
	case <-time.After(interval):
		vi.lock.Lock()
		vi.avgBlockTimes[interval] = time.Duration(int64(interval) / int64(vi.vnode.Height()-oldHeight))
		vi.lock.Unlock()
	case <-vi.close:
		return
	}
}

// updateVotesPerMinute sleeps for 60 seconds and updates vi.votesPerMinute.
// (i.e. call it as a goroutine)
func (vi *VochainInfo) updateVotesPerMinute() {
	vi.lock.RLock()
	oldVoteCount := vi.voteCount
	vi.lock.RUnlock()
	select {
	case <-time.After(time.Minute):
		vi.lock.Lock()
		vi.votesPerMinute = int(vi.voteCount) - int(oldVoteCount)
		vi.lock.Unlock()
	case <-vi.close:
		return
	}
}

// Close stops all started goroutines of VochainInfo
func (vi *VochainInfo) Close() {
	close(vi.close)
}
