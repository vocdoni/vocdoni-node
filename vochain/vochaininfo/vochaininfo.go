package vochaininfo

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/state"
)

// VochainInfo stores some metrics and information regarding the Vochain Blockchain
// Avg1/10/60/360 are the block time average for 1 minute, 10 minutes, 1 hour and 6 hours
type VochainInfo struct {
	sync           bool
	votesPerMinute uint64
	avg1           uint64
	avg10          uint64
	avg60          uint64
	avg360         uint64
	avg1440        uint64
	vnode          *vochain.BaseApplication
	close          chan bool
	lock           sync.RWMutex
}

// NewVochainInfo creates a new VochainInfo type
func NewVochainInfo(node *vochain.BaseApplication) *VochainInfo {
	return &VochainInfo{
		vnode: node,
		close: make(chan bool),
	}
}

func (vi *VochainInfo) updateCounters() {
	height.Set(uint64(vi.vnode.Height()))

	pc, err := vi.vnode.State.CountProcesses(true)
	if err != nil {
		log.Errorf("cannot count processes: %s", err)
	}
	processTreeSize.Set(pc)

	vc, err := vi.vnode.State.CountTotalVotes()
	if err != nil {
		log.Errorf("cannot access vote count: %s", err)
	}
	voteCount.Set(vc)

	ac, err := vi.vnode.State.CountAccounts(true)
	if err != nil {
		log.Errorf("cannot count accounts: %s", err)
	}
	accountTreeSize.Set(ac)

	sc, err := vi.vnode.State.CountSIKs(true)
	if err != nil {
		log.Errorf("cannot count SIKs: %s", err)
	}
	sikTreeSize.Set(sc)

	voteCacheSize.Set(uint64(vi.vnode.State.CacheSize()))
	mempoolSize.Set(uint64(vi.vnode.MempoolSize()))
}

// Height returns the current number of blocks of the blockchain
func (vi *VochainInfo) Height() uint64 {
	return height.Get()
}

// BlockTimes returns the average block time in milliseconds for 1, 10, 60, 360 and 1440 minutes.
// Value 0 means there is not yet an average
func (vi *VochainInfo) BlockTimes() *[5]uint64 {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return &[5]uint64{vi.avg1, vi.avg10, vi.avg60, vi.avg360, vi.avg1440}
}

// EstimateBlockHeight provides an estimation time for a future blockchain height number.
func (vi *VochainInfo) EstimateBlockHeight(target time.Time) (uint64, error) {
	currentTime := time.Now()
	// diff time in seconds
	diffTime := target.Unix() - currentTime.Unix()

	// block time in ms
	times := vi.BlockTimes()
	getMaxTimeFrom := func(i int) uint64 {
		for ; i >= 0; i-- {
			if times[i] != 0 {
				return uint64(times[i])
			}
		}
		return 10000 // fallback
	}
	inPast := diffTime < 0
	absDiff := diffTime
	// check diff is not too big
	if absDiff > math.MaxUint64/1000 {
		return 0, fmt.Errorf("target time %v is too far in the future", target)
	}
	if inPast {
		absDiff = -absDiff
	}
	t := uint64(0)
	switch {
	// if less than around 15 minutes missing
	case absDiff < 900:
		t = getMaxTimeFrom(1)
	// if less than around 6 hours missing
	case absDiff < 21600:
		t = getMaxTimeFrom(3)
	// if more than around 6 hours missing
	default:
		t = getMaxTimeFrom(4)
	}
	// Multiply by 1000 because t is represented in seconds, not ms.
	// Dividing t first can floor the integer, leading to divide-by-zero
	currentHeight := uint64(vi.Height())
	blockDiff := uint64(absDiff*1000) / t
	if inPast {
		if blockDiff > currentHeight {
			return 0, fmt.Errorf("target time %v is before origin", target)
		}
		return currentHeight - blockDiff, nil
	}
	return currentHeight + blockDiff, nil
}

// HeightTime estimates the UTC time for a future height or returns the
// block timestamp if height is in the past.
func (vi *VochainInfo) HeightTime(height uint64) time.Time {
	times := vi.BlockTimes()
	currentHeight := vi.Height()
	diffHeight := int64(height - currentHeight)

	if diffHeight < 0 {
		blk := vi.vnode.GetBlockByHeight(int64(height))
		if blk == nil {
			log.Errorf("cannot get block height %d", height)
			return time.Time{}
		}
		return blk.Header.Time
	}

	getMaxTimeFrom := func(i int) uint64 {
		for ; i >= 0; i-- {
			if times[i] != 0 {
				return times[i]
			}
		}
		return 10000 // fallback
	}

	t := uint64(0)
	switch {
	// if less than around 15 minutes missing
	case diffHeight < 100:
		t = getMaxTimeFrom(1)
	// if less than around 6 hours missing
	case diffHeight < 1000:
		t = getMaxTimeFrom(3)
	// if less than around 6 hours missing
	case diffHeight >= 1000:
		t = getMaxTimeFrom(4)
	}
	return time.Now().Add(time.Duration(diffHeight*int64(t)) * time.Millisecond)
}

// TreeSizes returns the current size of the ProcessTree, VoteTree and the votes per minute
// ProcessTree: total number of created voting processes in the blockchain
// VoteTree: total number of votes registered in the blockchain
// VotesPerMinute: number of votes included in the last 60 seconds
func (vi *VochainInfo) TreeSizes() (uint64, uint64, uint64) {
	vi.lock.RLock()
	defer vi.lock.RUnlock()
	return processTreeSize.Get(), voteCount.Get(), vi.votesPerMinute
}

// MempoolSize returns the current number of transactions waiting to be validated
func (vi *VochainInfo) MempoolSize() uint64 {
	return mempoolSize.Get()
}

// VoteCacheSize returns the current number of validated votes waiting to be
// included in the blockchain
func (vi *VochainInfo) VoteCacheSize() uint64 {
	return voteCacheSize.Get()
}

// AccountTreeSize returns the current number of validated votes waiting to be
// included in the blockchain
func (vi *VochainInfo) AccountTreeSize() uint64 {
	return accountTreeSize.Get()
}

// SIKTreeSize returns the current number of validated votes waiting to be
// included in the blockchain
func (vi *VochainInfo) SIKTreeSize() uint64 {
	return sikTreeSize.Get()
}

// TokensBurned returns the current balance of the burn address
func (vi *VochainInfo) TokensBurned() uint64 {
	acc, err := vi.vnode.State.GetAccount(state.BurnAddress, true)
	if err != nil || acc == nil {
		return 0
	}
	return acc.Balance
}

// NetInfo returns network info (mainly, peers)
func (vi *VochainInfo) NetInfo() (*coretypes.ResultNetInfo, error) {
	if vi.vnode.NodeClient != nil {
		return vi.vnode.NodeClient.NetInfo(context.Background())
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

// Start initializes the Vochain statistics recollection.
// TODO: use time.Duration instead of int64
func (vi *VochainInfo) Start(sleepSecs uint64) {
	log.Infof("starting vochain info service every %d seconds", sleepSecs)

	metrics.NewGauge("vochain_tokens_burned",
		func() float64 { return float64(vi.TokensBurned()) })

	var duration time.Duration
	var pheight, height uint64
	var h1, h10, h60, h360, h1440 uint64
	var n1, n10, n60, n360, n1440, vm uint64
	var a1, a10, a60, a360, a1440 uint64
	var sync bool
	var oldVoteTreeSize uint64
	duration = time.Second * time.Duration(sleepSecs)
	for {
		select {
		case <-time.After(duration):
			vi.updateCounters()

			height = uint64(vi.vnode.Height())

			// less than 2s per block it's not real. Consider blockchain is synchcing
			if pheight > 0 {
				sync = true
				vm++
				n1++
				n10++
				n60++
				n360++
				n1440++
				h1 += height - pheight
				h10 += height - pheight
				h60 += height - pheight
				h360 += height - pheight
				h1440 += height - pheight

				if sleepSecs*n1 >= 60 && h1 > 0 {
					a1 = uint64((n1 * sleepSecs * 1000) / h1)
					n1 = 0
					h1 = 0
				}
				if sleepSecs*n10 >= 600 && h10 > 0 {
					a10 = uint64((n10 * sleepSecs * 1000) / h10)
					n10 = 0
					h10 = 0
				}
				if sleepSecs*n60 >= 3600 && h60 > 0 {
					a60 = uint64((n60 * sleepSecs * 1000) / h60)
					n60 = 0
					h60 = 0
				}
				if sleepSecs*n360 >= 21600 && h360 > 0 {
					a360 = uint64((n360 * sleepSecs * 1000) / h360)
					n360 = 0
					h360 = 0
				}
				if sleepSecs*n1440 >= 86400 && h1440 > 0 {
					a1440 = uint64((n1440 * sleepSecs * 1000) / h1440)
					n1440 = 0
					h1440 = 0
				}
			} else {
				sync = false
			}
			pheight = height

			vi.lock.Lock()
			vi.sync = sync
			vi.avg1 = a1
			vi.avg10 = a10
			vi.avg60 = a60
			vi.avg360 = a360
			vi.avg1440 = a1440

			if sleepSecs*vm >= 60 {
				vi.votesPerMinute = voteCount.Get() - oldVoteTreeSize
				oldVoteTreeSize = voteCount.Get()
				vm = 0
			}
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
