package results

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

// ComputeResults walks through the envelopes of a process and computes the results.
func ComputeResults(electionID []byte, st *state.State) (*Results, error) {
	if electionID == nil {
		return nil, fmt.Errorf("process is nil")
	}

	p, err := st.Process(electionID, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get process: %w", err)
	}

	if p.VoteOptions.MaxCount == 0 {
		p.VoteOptions.MaxCount = MaxQuestions
	}

	if p.VoteOptions.MaxCount > MaxQuestions {
		return nil, fmt.Errorf("maxCount overflow %d", p.VoteOptions.MaxCount)
	}
	results := &Results{
		Votes:        NewEmptyVotes(p.VoteOptions),
		ProcessID:    electionID,
		Weight:       new(types.BigInt).SetUint64(0),
		VoteOpts:     p.VoteOptions,
		EnvelopeType: p.EnvelopeType,
		BlockHeight:  st.CurrentHeight(),
	}

	lock := sync.Mutex{}
	startTime := time.Now()
	err = st.IterateVotes(electionID, true, func(vote *models.StateDBVote) bool {
		keys := []string{}
		if p.EnvelopeType.EncryptedVotes {
			for _, k := range vote.EncryptionKeyIndexes {
				if k < types.KeyKeeperMaxKeyIndex && k < uint32(len(p.EncryptionPrivateKeys)) {
					keys = append(keys, p.EncryptionPrivateKeys[k])
				} else {
					log.Warn("wrong vote: key index overflow or too many fields")
					return false
				}
			}
		}
		vp, err := unmarshalVote(vote.VotePackage, keys)
		if err != nil {
			log.Debugf("vote invalid: %v", err)
			return false
		}
		if err = results.AddVote(vp.Votes, new(big.Int).SetBytes(vote.Weight), &lock); err != nil {
			log.Debugf("addVote failed: %v", err)
			return false
		}
		return false
	})
	if errors.Is(err, statedb.ErrEmptyTree) {
		// no votes, return empty results
		return results, nil
	}
	if err != nil {
		return nil, fmt.Errorf("IterateVotes failed: %w", err)
	}

	log.Infow("computed results",
		"process", fmt.Sprintf("%x", electionID),
		"results", results.String(),
		"elapsed", time.Since(startTime).String(),
	)
	return results, nil
}

// unmarshalVote decodes the base64 payload to a VotePackage struct type.
// If the vochain.VotePackage is encrypted the list of keys to decrypt it should be provided.
// The order of the Keys must be as it was encrypted.
// The function will reverse the order and use the decryption keys starting from the
// last one provided.
func unmarshalVote(VotePackage []byte, keys []string) (*state.VotePackage, error) {
	var vote state.VotePackage
	rawVote := bytes.Clone(VotePackage)
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
