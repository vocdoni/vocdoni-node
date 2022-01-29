package vochain

import (
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	cacheNullPrefix = "n_"
)

// cacheGetNullifierKey appends the cacheNullPrefix to nullifier
func cacheGetNullifierKey(nullifier []byte) string {
	return cacheNullPrefix + string(nullifier)
}

func (v *State) cacheDisabled() bool {
	return v.DisableVoteCache.Load().(bool)
}

// CacheAdd adds a new vote proof to the local cache
func (v *State) CacheAdd(id [32]byte, vote *models.Vote) {
	if v.cacheDisabled() {
		return
	}
	if vote == nil || vote.Nullifier == nil {
		return
	}
	v.voteCache.Add(id, vote)
	v.voteCache.Add(cacheGetNullifierKey(vote.Nullifier), nil)
}

// CacheDel deletes an existing vote proof from the local cache
func (v *State) CacheDel(id [32]byte) {
	if v.cacheDisabled() {
		return
	}
	vote := v.CacheGet(id)
	if vote != nil {
		v.voteCache.Remove(cacheGetNullifierKey(vote.Nullifier))
	}
	v.voteCache.Remove(id)
}

// CacheGet fetch an existing vote from the local cache and returns a copy
func (v *State) CacheGet(id [32]byte) *models.Vote {
	if v.cacheDisabled() {
		return nil
	}
	record, ok := v.voteCache.Get(id)
	if !ok || record == nil {
		return nil
	}
	return record.(*models.Vote)
}

// CacheGetCopy fetch an existing vote from the local cache and returns a copy
// which is thread-safe for writing.
func (v *State) CacheGetCopy(id [32]byte) *models.Vote {
	if v.cacheDisabled() {
		return nil
	}
	if vote := v.CacheGet(id); vote != nil {
		return proto.Clone(vote).(*models.Vote)
	}
	return nil
}

// CacheHasNullifier fetch an existing vote from the local cache
func (v *State) CacheHasNullifier(nullifier []byte) bool {
	if v.cacheDisabled() {
		return false
	}
	_, ok := v.voteCache.Get(cacheGetNullifierKey(nullifier))
	return ok
}

// CachePurge removes the old cache saved votes
func (v *State) CachePurge(height uint32) {
	if v.cacheDisabled() {
		return
	}
	// Purge only every 18 blocks (3 minute)
	if height%18 != 0 { // TODO(pau): do not use height but time
		return
	}
	keys := v.voteCache.Keys()
	removed := 0
	for _, id := range keys {
		record, ok := v.voteCache.Get(id)
		if !ok {
			// vote have been already deleted?
			continue
		}
		vote, ok := record.(*models.Vote)
		if !ok {
			continue
		}
		if !ok {
			log.Warn("vote cache index is not [32]byte")
			continue
		}
		if vote.Height+voteCachePurgeThreshold >= height {
			v.voteCache.Remove(cacheGetNullifierKey(vote.Nullifier))
			v.voteCache.Remove(id)
			removed++
		}
	}
	log.Debugf("removed %d votes from cache", removed)
}

// SetCacheSize sets the size for the vote LRU cache.
func (v *State) SetCacheSize(size int) {
	v.voteCache.Resize(size)
}

// CacheSize returns the current size of the vote cache
func (v *State) CacheSize() int {
	if v.cacheDisabled() {
		return 0
	}
	return v.voteCache.Len()
}
