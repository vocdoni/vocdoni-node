package results

import (
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

// ComputeResults walks through the envelopes of a process and computes the results.
func ComputeResults(electionID []byte, st *state.State) (*Results, error) {
	if electionID == nil {
		return nil, fmt.Errorf("process is nil")
	}

	p, err := st.Process(electionID, true)
	if err != nil {
		return nil, fmt.Errorf("cannot get process: %w", err)
	}

	if p.VoteOptions.MaxCount == 0 || p.VoteOptions.MaxValue == 0 {
		return nil, fmt.Errorf("maxCount and/or maxValue is zero")
	}
	if p.VoteOptions.MaxCount > MaxQuestions || p.VoteOptions.MaxValue > MaxOptions {
		return nil, fmt.Errorf("maxCount and/or maxValue overflows hardcoded maximum")
	}
	results := &Results{
		Votes:        NewEmptyVotes(int(p.VoteOptions.MaxCount), int(p.VoteOptions.MaxValue)+1),
		ProcessID:    electionID,
		Weight:       new(types.BigInt).SetUint64(0),
		Final:        true,
		VoteOpts:     p.VoteOptions,
		EnvelopeType: p.EnvelopeType,
		BlockHeight:  st.CurrentHeight(),
	}

	var nvotes atomic.Uint64
	lock := sync.Mutex{}
	starTime := time.Now()

	if err = st.IterateVotes(electionID, true, func(vote *models.StateDBVote) bool {
		var vp *state.VotePackage
		var err error
		if p.EnvelopeType.EncryptedVotes {
			if len(p.EncryptionPrivateKeys) < len(vote.EncryptionKeyIndexes) {
				log.Warn("wrong vote: encryptionKeyIndexes has too many fields")
				return false
			}
			keys := []string{}
			for _, k := range vote.EncryptionKeyIndexes {
				if k >= types.KeyKeeperMaxKeyIndex {
					log.Warn("wrong vote: key index overflow")
					return false
				}
				keys = append(keys, p.EncryptionPrivateKeys[k])
			}
			if len(keys) == 0 {
				log.Warn("wrong vote: no keys provided or wrong index")
				return false
			}
			vp, err = unmarshalVote(vote.VotePackage, keys)
		} else {
			vp, err = unmarshalVote(vote.VotePackage, []string{})
		}
		if err != nil {
			log.Debugf("vote invalid: %v", err)
			return false
		}

		if err = results.AddVote(vp.Votes, new(big.Int).SetBytes(vote.Weight), &lock); err != nil {
			log.Warnf("addVote failed: %v", err)
			return false
		}
		nvotes.Add(1)
		return false
	}); err == nil {
		log.Infow("computed results",
			"process", fmt.Sprintf("%x", electionID),
			"votes", nvotes.Load(),
			"results", results.String(),
			"elapsed", time.Since(starTime).String(),
		)
	}
	results.EnvelopeHeight = nvotes.Load()
	return results, err
}

// unmarshalVote decodes the base64 payload to a VotePackage struct type.
// If the vochain.VotePackage is encrypted the list of keys to decrypt it should be provided.
// The order of the Keys must be as it was encrypted.
// The function will reverse the order and use the decryption keys starting from the
// last one provided.
func unmarshalVote(VotePackage []byte, keys []string) (*state.VotePackage, error) {
	var vote state.VotePackage
	rawVote := make([]byte, len(VotePackage))
	copy(rawVote, VotePackage)
	// if encryption keys, decrypt the vote
	if len(keys) > 0 {
		for i := len(keys) - 1; i >= 0; i-- {
			priv, err := nacl.DecodePrivate(keys[i])
			if err != nil {
				return nil, fmt.Errorf("cannot create private key cipher: (%s)", err)
			}
			if rawVote, err = priv.Decrypt(rawVote); err != nil {
				return nil, fmt.Errorf("cannot decrypt vote with index key %d: %w", i, err)
			}
		}
	}
	if err := vote.Decode(rawVote); err != nil {
		return nil, fmt.Errorf("cannot unmarshal vote: %w", err)
	}
	return &vote, nil
}
