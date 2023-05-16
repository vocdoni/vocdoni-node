package state

import (
	"go.vocdoni.io/dvote/log"
)

const (
	cacheNullPrefix = "n_"
)

// cacheGetNullifierKey appends the cacheNullPrefix to nullifier
func cacheGetNullifierKey(nullifier []byte) string {
	return cacheNullPrefix + string(nullifier)
}

func (v *State) cacheDisabled() bool {
	return v.DisableVoteCache.Load()
}

// CacheAdd adds a new vote proof to the local cache
func (v *State) CacheAdd(id [32]byte, vote *Vote) {
	if v.cacheDisabled() {
		return
	}
	if vote == nil || vote.Nullifier == nil {
		return
	}
	v.voteCache.Add(string(id[:]), vote)
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
	v.voteCache.Remove(string(id[:]))
}

// CacheGet fetch an existing vote from the local cache and returns it.
func (v *State) CacheGet(id [32]byte) *Vote {
	if v.cacheDisabled() {
		return nil
	}
	vote, _ := v.voteCache.Get(string(id[:]))
	return vote
}

// CacheGetCopy fetch an existing vote from the local cache and returns a copy
// which is thread-safe for writing.
func (v *State) CacheGetCopy(id [32]byte) *Vote {
	if v.cacheDisabled() {
		return nil
	}
	if vote := v.CacheGet(id); vote != nil {
		// return a deep copy of vote
		return vote.DeepCopy()
	}
	return nil
}

// CacheHasNullifier fetch an existing vote from the local cache.
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
		vote, ok := v.voteCache.Get(id)
		if !ok {
			// vote have been already deleted?
			continue
		}
		if vote == nil {
			// each vote is already deleted with the nullifier key as well
			continue
		}
		if height >= vote.Height+voteCachePurgeThreshold {
			v.voteCache.Remove(cacheGetNullifierKey(vote.Nullifier))
			v.voteCache.Remove(id)
			removed++
		}
	}
	if removed != 0 {
		log.Debugf("removed %d votes from cache", removed)
	}
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
