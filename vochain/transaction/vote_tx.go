package transaction

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/vocdoni/arbo"
	"github.com/vocdoni/go-snark/verifier"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/log"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

// VoteTxCheck performs basic checks on a vote transaction.
func (t *TransactionHandler) VoteTxCheck(vtx *vochaintx.VochainTx, forCommit bool) (*models.Vote, vstate.VoterID, error) {
	voteEnvelope := vtx.Tx.GetVote()
	if voteEnvelope == nil {
		return nil, vstate.VoterID{}, fmt.Errorf("vote envelope is nil")
	}

	// Perform basic checks
	voterID := vstate.VoterID{}
	if voteEnvelope == nil {
		return nil, voterID.Nil(), fmt.Errorf("vote envelope is nil")
	}
	process, err := t.state.Process(voteEnvelope.ProcessId, false)
	if err != nil {
		return nil, voterID.Nil(), fmt.Errorf("cannot fetch processId: %w", err)
	}
	if process == nil || process.EnvelopeType == nil || process.Mode == nil {
		return nil, voterID.Nil(), fmt.Errorf("process %x malformed", voteEnvelope.ProcessId)
	}
	height := t.state.CurrentHeight()
	endBlock := process.StartBlock + process.BlockCount

	if height < process.StartBlock {
		return nil, voterID.Nil(), fmt.Errorf(
			"process %x starts at height %d, current height is %d",
			voteEnvelope.ProcessId, process.StartBlock, height)
	} else if height > endBlock {
		return nil, voterID.Nil(), fmt.Errorf(
			"process %x finished at height %d, current height is %d",
			voteEnvelope.ProcessId, endBlock, height)
	}

	if process.Status != models.ProcessStatus_READY {
		return nil, voterID.Nil(), fmt.Errorf(
			"process %x not in READY state - current state: %s",
			voteEnvelope.ProcessId, process.Status.String())
	}

	// Check in case of keys required, they have been sent by some keykeeper
	if process.EnvelopeType.EncryptedVotes &&
		process.KeyIndex != nil &&
		*process.KeyIndex < 1 {
		return nil, voterID.Nil(), fmt.Errorf("no keys available, voting is not possible")
	}

	var vote *models.Vote
	if process.EnvelopeType.Anonymous {
		if t.ZkVKs == nil || len(t.ZkVKs) == 0 {
			return nil, voterID.Nil(), fmt.Errorf("anonymous voting not supported, missing zk verification keys")
		}
		// In order to avoid double vote check (on checkTx and deliverTx), we use a memory vote cache.
		// An element can only be added to the vote cache during checkTx.
		// Every N seconds the old votes which are not yet in the blockchain will be removed from cache.
		// If the same vote (but different transaction) is send to the mempool, the cache will detect it
		// and vote will be discarted.
		// We use CacheGetCopy because we will modify the vote to set
		// the Height.  If we don't work with a copy we are racing with
		// concurrent reads to the votes in the cache which happen in
		// in State.CachePurge run via a goroutine in
		// started in BaseApplication.BeginBlock.
		vote = t.state.CacheGetCopy(vtx.TxID)

		// if vote is in cache, lazy check
		if vote != nil {
			// if not forCommit, it is a mempool check,
			// reject it since we already processed the transaction before.
			if !forCommit {
				return nil, voterID.Nil(), ErrorAlreadyExistInCache
			}

			vote.Height = height // update vote height
			defer t.state.CacheDel(vtx.TxID)
			if exist, err := t.state.EnvelopeExists(vote.ProcessId,
				vote.Nullifier, false); err != nil || exist {
				if err != nil {
					return nil, voterID.Nil(), err
				}
				return nil, voterID.Nil(), fmt.Errorf("vote %x already exists", vote.Nullifier)
			}
			return vote, voterID.Nil(), nil
		}

		// Supports Groth16 proof generated from circom snark compatible
		// provoteEnveloper
		proofZkSNARK := voteEnvelope.Proof.GetZkSnark()
		if proofZkSNARK == nil {
			return nil, voterID.Nil(), fmt.Errorf("zkSNARK proof is empty")
		}
		proof, _, err := zk.ProtobufZKProofToCircomProof(proofZkSNARK)
		if err != nil {
			return nil, voterID.Nil(), fmt.Errorf("failed on zk.ProtobufZKProofToCircomProof: %w", err)
		}

		// voteEnvelope.Nullifier is encoded in little-endian
		nullifierBI := arbo.BytesToBigInt(voteEnvelope.Nullifier)

		// check if vote already exists
		if exist, err := t.state.EnvelopeExists(voteEnvelope.ProcessId,
			voteEnvelope.Nullifier, false); err != nil || exist {
			if err != nil {
				return nil, voterID.Nil(), err
			}
			return nil, voterID.Nil(), fmt.Errorf("vote %x already exists", voteEnvelope.Nullifier)
		}
		log.Debugf("new zk vote %x for process %x", voteEnvelope.Nullifier, voteEnvelope.ProcessId)

		if int(proofZkSNARK.CircuitParametersIndex) >= len(t.ZkVKs) ||
			int(proofZkSNARK.CircuitParametersIndex) < 0 {
			return nil, voterID.Nil(), fmt.Errorf("invalid CircuitParametersIndex: %d of %d",
				proofZkSNARK.CircuitParametersIndex, len(t.ZkVKs))
		}
		verificationKey := t.ZkVKs[proofZkSNARK.CircuitParametersIndex]

		// prepare the publicInputs that are defined by the process.
		// publicInputs contains: processId0, processId1, censusRoot,
		// nullifier, voteHash0, voteHash1.
		processId0BI := arbo.BytesToBigInt(process.ProcessId[:16])
		processId1BI := arbo.BytesToBigInt(process.ProcessId[16:])
		censusRootBI := arbo.BytesToBigInt(process.RollingCensusRoot)
		// voteHash from the user voteValue to the publicInputs
		voteValueHash := sha256.Sum256(voteEnvelope.VotePackage)
		voteHash0 := arbo.BytesToBigInt(voteValueHash[:16])
		voteHash1 := arbo.BytesToBigInt(voteValueHash[16:])
		publicInputs := []*big.Int{
			processId0BI,
			processId1BI,
			censusRootBI,
			nullifierBI,
			voteHash0,
			voteHash1,
		}

		// check zkSnark proof
		if !verifier.Verify(verificationKey, proof, publicInputs) {
			return nil, voterID.Nil(), fmt.Errorf("zkSNARK proof verification failed")
		}

		// TODO the next 12 lines of code are the same than a little
		// further down. TODO: maybe movoteEnvelope them before the 'switch', as
		// is a logic that must be done evoteEnvelopen if
		// process.EnvoteEnvelopelopeType.Anonymous==true or not
		vote = &models.Vote{
			Height:      height,
			ProcessId:   voteEnvelope.ProcessId,
			VotePackage: voteEnvelope.VotePackage,
			Nullifier:   voteEnvelope.Nullifier,
			// Anonymous Voting doesn't support weighted voting, so
			// we assing always 1 to each vote.
			Weight: big.NewInt(1).Bytes(),
		}
		// If process encrypted, check the vote is encrypted (includes at least one key index)
		if process.EnvelopeType.EncryptedVotes {
			if len(voteEnvelope.EncryptionKeyIndexes) == 0 {
				return nil, voterID.Nil(), fmt.Errorf("no key indexes provided on vote package")
			}
			vote.EncryptionKeyIndexes = voteEnvelope.EncryptionKeyIndexes
		}
	} else { // Signature based voting
		if vtx.Signature == nil {
			return nil, voterID.Nil(), fmt.Errorf("signature missing on voteTx")
		}
		// In order to avoid double vote check (on checkTx and delivoteEnveloperTx), we use a memory vote cache.
		// An element can only be added to the vote cache during checkTx.
		// EvoteEnvelopery N seconds the old votes which are not yet in the blockchain will be removoteEnveloped from cache.
		// If the same vote (but different transaction) is send to the mempool, the cache will detect it
		// and vote will be discarted.
		// We use CacheGetCopy because we will modify the vote to set
		// the Height.  If we don't work with a copy we are racing with
		// concurrent reads to the votes in the cache which happen in
		// in State.CachePurge run via a goroutine in
		// started in BaseApplication.BeginBlock.
		// Warning: vote cache might change during the execution of this function
		vote = t.state.CacheGetCopy(vtx.TxID)

		// if the vote exists in cache
		if vote != nil {
			// if not forCommit, it is a mempool check,
			// reject it since we already processed the transaction before.
			if !forCommit {
				return nil, voterID.Nil(), fmt.Errorf("vote %x already exists in cache", vote.Nullifier)
			}

			// if we are on DelivoteEnveloperTx and the vote is in cache, lazy check
			defer t.state.CacheDel(vtx.TxID)
			vote.Height = height // update vote height
			if exist, err := t.state.EnvelopeExists(vote.ProcessId,
				vote.Nullifier, false); err != nil || exist {
				if err != nil {
					return nil, voterID.Nil(), err
				}
				return nil, voterID.Nil(), fmt.Errorf("vote %x already exists", vote.Nullifier)
			}
			if height > process.GetStartBlock()+process.GetBlockCount() ||
				process.GetStatus() != models.ProcessStatus_READY {
				return nil, voterID.Nil(), fmt.Errorf("vote %x is not longer valid", vote.Nullifier)
			}
			return vote, voterID.Nil(), nil
		}

		// if not in cache, full check
		// extract pubKey, generate nullifier and check census proof.
		// add the transaction in the cache
		vote = &models.Vote{
			Height:      height,
			ProcessId:   voteEnvelope.ProcessId,
			VotePackage: voteEnvelope.VotePackage,
		}

		// check proof is nil
		if voteEnvelope.Proof == nil {
			return nil, voterID.Nil(), fmt.Errorf("proof not found on transaction")
		}
		if voteEnvelope.Proof.Payload == nil {
			return nil, voterID.Nil(), fmt.Errorf("invalid proof payload provided")
		}

		// If process encrypted, check the vote is encrypted (includes at least one key index)
		if process.EnvelopeType.EncryptedVotes {
			if len(voteEnvelope.EncryptionKeyIndexes) == 0 {
				return nil, voterID.Nil(), fmt.Errorf("no key indexes provided on vote package")
			}
			vote.EncryptionKeyIndexes = voteEnvelope.EncryptionKeyIndexes
		}
		pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
		if err != nil {
			return nil, voterID.Nil(), fmt.Errorf("cannot extract public key from signature: %w", err)
		}
		voterID = []byte{vstate.VoterIDTypeECDSA}
		voterID = append(voterID, pubKey...)
		addr, err := ethereum.AddrFromPublicKey(pubKey)
		if err != nil {
			return nil, voterID.Nil(), fmt.Errorf("cannot extract address from public key: %w", err)
		}
		// assign a nullifier
		vote.Nullifier = vstate.GenerateNullifier(addr, vote.ProcessId)

		// check if vote already exists
		if exist, err := t.state.EnvelopeExists(vote.ProcessId,
			vote.Nullifier, false); err != nil || exist {
			if err != nil {
				return nil, voterID.Nil(), err
			}
			return nil, voterID.Nil(), fmt.Errorf("vote %x already exists", vote.Nullifier)
		}
		log.Debugf("new vote %x for address %s and process %x", vote.Nullifier, addr.Hex(), voteEnvelope.ProcessId)

		valid, weight, err := VerifyProof(process, voteEnvelope.Proof,
			process.CensusOrigin, process.CensusRoot, process.ProcessId,
			pubKey, addr)
		if err != nil {
			return nil, voterID.Nil(), err
		}
		if !valid {
			return nil, voterID.Nil(), fmt.Errorf("proof not valid")
		}
		vote.Weight = weight.Bytes()
	}
	if !forCommit {
		// add the vote to cache
		t.state.CacheAdd(vtx.TxID, vote)
	}
	return vote, voterID, nil
}