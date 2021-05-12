package vochain

import (
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

// CacheAdd adds a new vote proof to the local cache
func (v *State) CacheAdd(id [32]byte, vote *models.Vote) {
	if len(id) == 0 {
		return
	}
	v.voteCache.Add(id, vote)
}

// CacheDel deletes an existing vote proof from the local cache
func (v *State) CacheDel(id [32]byte) {
	v.voteCache.Remove(id)
}

// CacheGet fetch an existing vote from the local cache
func (v *State) CacheGet(id [32]byte) *models.Vote {
	record, ok := v.voteCache.Get(id)
	if !ok || record == nil {
		return nil
	}
	return record.(*models.Vote)
}

// CachePurge removes the old cache saved votes
func (v *State) CachePurge(height uint32) {
	// Purge only every 6 blocks (3 minute)
	if height%18 != 0 {
		return
	}
	start := time.Now()
	purged := 0
	v.voteCache.Keys()
	for _, id := range v.voteCache.Keys() {
		record, ok := v.voteCache.Get(id)
		if !ok {
			// vote have been already deleted?
			continue
		}
		vote, ok := record.(*models.Vote)
		if !ok {
			log.Warn("vote cache is not models.Vote")
			continue
		}
		vid, ok := id.([32]byte)
		if !ok {
			log.Warn("vote cache index is not [32]byte")
			continue
		}
		if vote.Height+voteCachePurgeThreshold >= height {
			v.voteCache.Remove(vid)
			if v.MemPoolRemoveTxKey != nil {
				v.MemPoolRemoveTxKey(vid, true)
				purged++
			}
		}
	}
	if purged > 0 {
		log.Infof("[txcache] purged %d transactions, took %s", purged, time.Since(start))
	}
}

// CacheSize returns the current size of the vote cache
func (v *State) CacheSize() int {
	return v.voteCache.Len()
}
