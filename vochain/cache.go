package vochain

import (
	"time"

	"gitlab.com/vocdoni/go-dvote/types"
)

// VoteCacheAdd adds a new vote proof to the local cache
func (v *State) VoteCacheAdd(id string, vc *types.VoteProof) {
	if len(id) == 0 {
		return
	}
	v.voteCacheLock.Lock()
	defer v.voteCacheLock.Unlock()
	v.voteCache[id] = vc
}

// VoteCacheDel deletes an existing vote proof from the local cache
func (v *State) VoteCacheDel(id string) {
	if id == "" {
		return
	}
	v.voteCacheLock.Lock()
	defer v.voteCacheLock.Unlock()
	delete(v.voteCache, id)
}

// VoteCacheGet fetch an existing vote proof from the local cache
func (v *State) VoteCacheGet(id string) *types.VoteProof {
	if len(id) == 0 {
		return nil
	}
	v.voteCacheLock.RLock()
	defer v.voteCacheLock.RUnlock()
	return v.voteCache[id]
}

// VoteCachePurge removes the old cache saved votes
func (v *State) VoteCachePurge(height int64) {
	if height%6 != 0 {
		return
	}
	v.voteCacheLock.Lock()
	defer v.voteCacheLock.Unlock()
	for id, vp := range v.voteCache {
		if time.Since(vp.Created) > voteCachePurgeThreshold {
			delete(v.voteCache, id)
		}
	}
}

// VoteCacheSize returns the current size of the vote cache
func (v *State) VoteCacheSize() int {
	v.voteCacheLock.RLock()
	defer v.voteCacheLock.RUnlock()
	return len(v.voteCache)
}
