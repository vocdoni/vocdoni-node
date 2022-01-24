package vochain

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/arbo"
	"github.com/vocdoni/go-snark/verifier"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// AddTx check the validity of a transaction and adds it to the state if commit=true.
// It returns a bytes value which depends on the transaction type:
//  Tx_Vote: vote nullifier
//  default: []byte{}
func (app *BaseApplication) AddTx(vtx *models.Tx, txBytes, signature []byte,
	txID [32]byte, commit bool) ([]byte, error) {
	if vtx == nil || app.State == nil || vtx.Payload == nil {
		return nil, fmt.Errorf("transaction, state, and/or transaction payload is nil")
	}
	switch vtx.Payload.(type) {
	case *models.Tx_Vote:
		// get VoteEnvelope from tx
		txVote := vtx.GetVote()
		v, err := app.VoteEnvelopeCheck(txVote, txBytes, signature, txID, commit)
		if err != nil || v == nil {
			return []byte{}, fmt.Errorf("voteTxCheck: %w", err)
		}
		if commit {
			return v.Nullifier, app.State.AddVote(v)
		}
		return v.Nullifier, nil
	case *models.Tx_Admin:
		if err := AdminTxCheck(vtx, txBytes, signature, app.State); err != nil {
			return []byte{}, fmt.Errorf("adminTxChek: %w", err)
		}
		tx := vtx.GetAdmin()
		if commit {
			switch tx.Txtype {
			case models.TxType_ADD_ORACLE:
				return []byte{}, app.State.AddOracle(common.BytesToAddress(tx.Address))
			case models.TxType_REMOVE_ORACLE:
				return []byte{}, app.State.RemoveOracle(common.BytesToAddress(tx.Address))
			case models.TxType_ADD_VALIDATOR:
				pk, err := hexPubKeyToTendermintEd25519(fmt.Sprintf("%x", tx.PublicKey))
				if err == nil {
					if tx.Power == nil {
						return []byte{}, fmt.Errorf("power not specified on addValidator transaction")
					}
					validator := &models.Validator{
						Address: pk.Address().Bytes(),
						PubKey:  pk.Bytes(),
						Power:   *tx.Power,
					}
					return []byte{}, app.State.AddValidator(validator)

				}
				return []byte{}, fmt.Errorf("addValidator: %w", err)

			case models.TxType_REMOVE_VALIDATOR:
				return []byte{}, app.State.RemoveValidator(tx.Address)
			case models.TxType_ADD_PROCESS_KEYS:
				return []byte{}, app.State.AddProcessKeys(tx)
			case models.TxType_REVEAL_PROCESS_KEYS:
				return []byte{}, app.State.RevealProcessKeys(tx)
			}
		}

	case *models.Tx_NewProcess:
		if p, err := app.NewProcessTxCheck(vtx, txBytes, signature, app.State); err == nil {
			if commit {
				tx := vtx.GetNewProcess()
				if tx.Process == nil {
					return []byte{}, fmt.Errorf("newProcess process is empty")
				}
				return []byte{}, app.State.AddProcess(p)
			}
		} else {
			return []byte{}, fmt.Errorf("newProcess: %w", err)
		}

	case *models.Tx_SetProcess:
		if err := SetProcessTxCheck(vtx, txBytes, signature, app.State); err != nil {
			return []byte{}, fmt.Errorf("setProcess: %w", err)
		}
		if commit {
			tx := vtx.GetSetProcess()
			switch tx.Txtype {
			case models.TxType_SET_PROCESS_STATUS:
				if tx.GetStatus() == models.ProcessStatus_PROCESS_UNKNOWN {
					return []byte{}, fmt.Errorf("set process status, status unknown")
				}
				return []byte{}, app.State.SetProcessStatus(tx.ProcessId, *tx.Status, true)
			case models.TxType_SET_PROCESS_RESULTS:
				if tx.GetResults() == nil {
					return []byte{}, fmt.Errorf("set process results, results is nil")
				}
				return []byte{}, app.State.SetProcessResults(tx.ProcessId, tx.Results, true)
			case models.TxType_SET_PROCESS_CENSUS:
				if tx.GetCensusRoot() == nil {
					return []byte{}, fmt.Errorf("set process census, census root is nil")
				}
				return []byte{}, app.State.SetProcessCensus(tx.ProcessId, tx.CensusRoot, tx.GetCensusURI(), true)
			default:
				return []byte{}, fmt.Errorf("unknown set process tx type")
			}
		}

	case *models.Tx_RegisterKey:
		if err := app.State.RegisterKeyTxCheck(vtx, txBytes, signature, app.State,
			commit); err != nil {
			return []byte{}, fmt.Errorf("registerKeyTx %w", err)
		}
		if commit {
			tx := vtx.GetRegisterKey()
			weight, ok := new(big.Int).SetString(tx.Weight, 10)
			if !ok {
				return []byte{}, fmt.Errorf("cannot parse weight %s", weight)
			}
			return []byte{}, app.State.AddToRollingCensus(tx.ProcessId, tx.NewKey, weight)
		}

	case *models.Tx_SetAccountInfo:
		txValues, err := SetAccountInfoTxCheck(vtx, txBytes, signature, app.State)
		if err != nil {
			return []byte{}, fmt.Errorf("cannot set account: %w", err)
		}
		if commit {
			if txValues.Create {
				return []byte{}, app.State.CreateAccount(txValues.Account, vtx.GetSetAccountInfo().GetInfoURI(), make([]common.Address, 0), 0)
			} else {
				return []byte{}, app.State.SetAccountInfoURI(txValues.Account, txValues.TxSender, vtx.GetSetAccountInfo().InfoURI)
			}
		}

	case *models.Tx_MintTokens:
		address, amount, err := MintTokensTxCheck(vtx, txBytes, signature, app.State)
		if err != nil {
			return []byte{}, fmt.Errorf("mintTokensTx: %w", err)
		}
		if commit {
			if err := app.State.MintBalance(address, amount); err != nil {
				return []byte{}, fmt.Errorf("mintTokensTx: %w", err)
			}
			return []byte{}, app.State.incrementTreasurerNonce()
		}

	case *models.Tx_SetAccountDelegateTx:
		accountAddr, delegate, err := SetAccountDelegateTxCheck(vtx, txBytes, signature, app.State)
		if err != nil {
			return []byte{}, fmt.Errorf("setAccountDelegateTx: %w", err)
		}
		if commit {
			tx := vtx.GetSetAccountDelegateTx()
			switch tx.Txtype {
			case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
				return []byte{}, app.State.SetDelegate(accountAddr, delegate, models.TxType_ADD_DELEGATE_FOR_ACCOUNT)
			case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
				return []byte{}, app.State.SetDelegate(accountAddr, delegate, models.TxType_DEL_DELEGATE_FOR_ACCOUNT)
			default:
				return []byte{}, fmt.Errorf("unknown set account delegate tx type")
			}
		}

	case *models.Tx_SendTokens:
		txValues, err := SendTokensTxCheck(vtx, txBytes, signature, app.State)
		if err != nil {
			return []byte{}, fmt.Errorf("sendTokensTx: %w", err)
		}
		if commit {
			if txValues != nil {
				return []byte{}, app.State.TransferBalance(txValues.From, txValues.To, txValues.Value, uint64(txValues.Nonce))
			}
			return []byte{}, fmt.Errorf("sendTokensTx: tx data is invalid")
		}

	case *models.Tx_CollectFaucet:
		from, err := CollectFaucetTxCheck(vtx, txBytes, signature, app.State)
		if err != nil {
			return []byte{}, fmt.Errorf("collectFaucetTx: %w", err)
		}
		if commit {
			tx := vtx.GetCollectFaucet()
			return []byte{}, app.State.CollectFaucet(
				from,
				common.BytesToAddress(tx.FaucetPackage.Payload.To),
				tx.FaucetPackage.Payload.Amount,
				tx.FaucetPackage.Payload.Identifier,
			)
		}

	default:
		return []byte{}, fmt.Errorf("invalid transaction type")
	}
	return []byte{}, nil
}

// UnmarshalTx unarshal the content of a bytes serialized transaction.
// Returns the transaction struct, the original bytes and the signature
// of those bytes.
func UnmarshalTx(content []byte) (*models.Tx, []byte, []byte, error) {
	stx := new(models.SignedTx)
	if err := proto.Unmarshal(content, stx); err != nil {
		return nil, nil, nil, err
	}
	vtx := new(models.Tx)
	return vtx, stx.GetTx(), stx.GetSignature(), proto.Unmarshal(stx.GetTx(), vtx)
}

// VoteEnvelopeCheck is an abstraction of ABCI checkTx for submitting a vote
// All hexadecimal strings should be already sanitized (without 0x)
func (app *BaseApplication) VoteEnvelopeCheck(ve *models.VoteEnvelope, txBytes, signature []byte,
	txID [32]byte, forCommit bool) (*models.Vote, error) {

	// Perform basic/general checks
	if ve == nil {
		return nil, fmt.Errorf("vote envelope is nil")
	}
	process, err := app.State.Process(ve.ProcessId, false)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch processId: %w", err)
	}
	if process == nil || process.EnvelopeType == nil || process.Mode == nil {
		return nil, fmt.Errorf("process %x malformed", ve.ProcessId)
	}
	height := app.State.CurrentHeight()
	endBlock := process.StartBlock + process.BlockCount

	if height < process.StartBlock || height > endBlock {
		return nil, fmt.Errorf("process %x not started or finished", ve.ProcessId)
	}

	if process.Status != models.ProcessStatus_READY {
		return nil, fmt.Errorf("process %x not in READY state", ve.ProcessId)
	}

	// Check in case of keys required, they have been sent by some keykeeper
	if process.EnvelopeType.EncryptedVotes &&
		process.KeyIndex != nil &&
		*process.KeyIndex < 1 {
		return nil, fmt.Errorf("no keys available, voting is not possible")
	}

	var vote *models.Vote
	if process.EnvelopeType.Anonymous {
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
		vote = app.State.CacheGetCopy(txID)

		// if vote is in cache, lazy check and remove it from cache
		if forCommit && vote != nil {
			vote.Height = height // update vote height
			defer app.State.CacheDel(txID)
			if exist, err := app.State.EnvelopeExists(vote.ProcessId,
				vote.Nullifier, false); err != nil || exist {
				if err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("vote %x already exists", vote.Nullifier)
			}
			return vote, nil
		}

		// if not forCommit, it is a mempool check,
		// reject it since we already processed the transaction before.
		if !forCommit && vote != nil {
			return nil, fmt.Errorf("vote %x already exists in cache", vote.Nullifier)
		}

		// Supports Groth16 proof generated from circom snark compatible
		// prover
		proofZkSNARK := ve.Proof.GetZkSnark()
		if proofZkSNARK == nil {
			return nil, fmt.Errorf("zkSNARK proof is empty")
		}
		proof, _, err := zk.ProtobufZKProofToCircomProof(proofZkSNARK)
		if err != nil {
			return nil, fmt.Errorf("failed on zk.ProtobufZKProofToCircomProof: %w", err)
		}

		// ve.Nullifier is encoded in little-endian
		nullifierBI := arbo.BytesToBigInt(ve.Nullifier)

		// check that nullifier does not exist in cache already, this avoids
		// processing multiple transactions with same nullifier.
		if app.State.CacheHasNullifier(ve.Nullifier) {
			return nil, fmt.Errorf("nullifier %x already exists in cache", ve.Nullifier)
		}
		// check if vote already exists
		if exist, err := app.State.EnvelopeExists(ve.ProcessId,
			ve.Nullifier, false); err != nil || exist {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("vote %x already exists", ve.Nullifier)
		}
		log.Debugf("new vote %x for process %x", ve.Nullifier, ve.ProcessId)

		if int(proofZkSNARK.CircuitParametersIndex) >= len(app.ZkVKs) {
			return nil, fmt.Errorf("invalid CircuitParametersIndex: %d of %d", proofZkSNARK.CircuitParametersIndex, len(app.ZkVKs))
		}
		verificationKey := app.ZkVKs[proofZkSNARK.CircuitParametersIndex]

		// prepare the publicInputs that are defined by the process.
		// publicInputs contains: processId0, processId1, censusRoot,
		// nullifier, voteHash0, voteHash1.
		processId0BI := arbo.BytesToBigInt(process.ProcessId[:16])
		processId1BI := arbo.BytesToBigInt(process.ProcessId[16:])
		censusRootBI := arbo.BytesToBigInt(process.RollingCensusRoot)
		// voteHash from the user voteValue to the publicInputs
		voteValueHash := sha256.Sum256(ve.VotePackage)
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
			return nil, fmt.Errorf("zkSNARK proof verification failed")
		}

		// TODO the next 12 lines of code are the same than a little
		// further down. TODO: maybe move them before the 'switch', as
		// is a logic that must be done even if
		// process.EnvelopeType.Anonymous==true or not
		vote = &models.Vote{
			Height:      height,
			ProcessId:   ve.ProcessId,
			VotePackage: ve.VotePackage,
			Nullifier:   ve.Nullifier,
			// Anonymous Voting doesn't support weighted voting, so
			// we assing always 1 to each vote.
			Weight: big.NewInt(1).Bytes(),
		}
		// If process encrypted, check the vote is encrypted (includes at least one key index)
		if process.EnvelopeType.EncryptedVotes {
			if len(ve.EncryptionKeyIndexes) == 0 {
				return nil, fmt.Errorf("no key indexes provided on vote package")
			}
			vote.EncryptionKeyIndexes = ve.EncryptionKeyIndexes
		}

		// add the vote to cache
		app.State.CacheAdd(txID, vote)
	} else {
		// Signature based voting
		if signature == nil {
			return nil, fmt.Errorf("signature missing on voteTx")
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
		vote = app.State.CacheGetCopy(txID)

		// if vote is in cache, lazy check and remove it from cache
		if forCommit && vote != nil {
			vote.Height = height // update vote height
			defer app.State.CacheDel(txID)
			if exist, err := app.State.EnvelopeExists(vote.ProcessId,
				vote.Nullifier, false); err != nil || exist {
				if err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("vote %x already exists", vote.Nullifier)
			}
			return vote, nil
		}

		// if not forCommit, it is a mempool check,
		// reject it since we already processed the transaction before.
		if !forCommit && vote != nil {
			return nil, fmt.Errorf("vote %x already exists in cache", vote.Nullifier)
		}

		// if not in cache, full check
		// extract pubKey, generate nullifier and check census proof.
		// add the transaction in the cache
		if ve.Proof == nil {
			return nil, fmt.Errorf("proof not found on transaction")
		}

		vote = &models.Vote{
			Height:      height,
			ProcessId:   ve.ProcessId,
			VotePackage: ve.VotePackage,
		}
		// If process encrypted, check the vote is encrypted (includes at least one key index)
		if process.EnvelopeType.EncryptedVotes {
			if len(ve.EncryptionKeyIndexes) == 0 {
				return nil, fmt.Errorf("no key indexes provided on vote package")
			}
			vote.EncryptionKeyIndexes = ve.EncryptionKeyIndexes
		}
		pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
		if err != nil {
			return nil, fmt.Errorf("cannot extract public key from signature: %w", err)
		}
		addr, err := ethereum.AddrFromPublicKey(pubKey)
		if err != nil {
			return nil, fmt.Errorf("cannot extract address from public key: %w", err)
		}

		// assign a nullifier
		vote.Nullifier = GenerateNullifier(addr, vote.ProcessId)

		// check that nullifier does not exist in cache already, this avoids
		// processing multiple transactions with same nullifier.
		if app.State.CacheHasNullifier(vote.Nullifier) {
			return nil, fmt.Errorf("nullifier %x already exists in cache", vote.Nullifier)
		}

		// check if vote already exists
		if exist, err := app.State.EnvelopeExists(vote.ProcessId,
			vote.Nullifier, false); err != nil || exist {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("vote %x already exists", vote.Nullifier)
		}
		log.Debugf("new vote %x for address %s and process %x", vote.Nullifier, addr.Hex(), ve.ProcessId)

		valid, weight, err := VerifyProof(process, ve.Proof,
			process.CensusOrigin, process.CensusRoot, process.ProcessId,
			pubKey, addr)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, fmt.Errorf("proof not valid")
		}
		vote.Weight = weight.Bytes()

		// add the vote to cache
		app.State.CacheAdd(txID, vote)
	}
	return vote, nil
}

// AdminTxCheck is an abstraction of ABCI checkTx for an admin transaction
func AdminTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	tx := vtx.GetAdmin()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return fmt.Errorf("missing signature and/or admin transaction")
	}
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}

	if authorized, addr, err := verifySignatureAgainstOracles(
		oracles, txBytes, signature); err != nil {
		return err
	} else if !authorized {
		return fmt.Errorf("unauthorized to perform an adminTx, address: %s", addr.Hex())
	}

	switch tx.Txtype {
	case models.TxType_ADD_PROCESS_KEYS, models.TxType_REVEAL_PROCESS_KEYS:
		if tx.ProcessId == nil {
			return fmt.Errorf("missing processId on AdminTxCheck")
		}
		// check process exists
		process, err := state.Process(tx.ProcessId, false)
		if err != nil {
			return err
		}
		if process == nil {
			return fmt.Errorf("process with id (%x) does not exist", tx.ProcessId)
		}
		// check process actually requires keys
		if !process.EnvelopeType.EncryptedVotes && !process.EnvelopeType.Anonymous {
			return fmt.Errorf("process does not require keys")
		}

		height := state.CurrentHeight()
		// Specific checks
		switch tx.Txtype {
		case models.TxType_ADD_PROCESS_KEYS:
			if tx.KeyIndex == nil {
				return fmt.Errorf("missing keyIndex on AdminTxCheck")
			}
			// endblock is always greater than start block so that case is also included here
			if height > process.StartBlock {
				return fmt.Errorf("cannot add process keys to a process that has started or finished")
			}
			// process is not canceled
			if process.Status == models.ProcessStatus_CANCELED ||
				process.Status == models.ProcessStatus_ENDED ||
				process.Status == models.ProcessStatus_RESULTS {
				return fmt.Errorf("cannot add process keys to a %s process", process.Status)
			}
			if len(process.EncryptionPublicKeys[*tx.KeyIndex]) > 0 {
				return fmt.Errorf("keys for process %x already revealed", tx.ProcessId)
			}
			// check included keys and keyindex are valid
			if err := checkAddProcessKeys(tx, process); err != nil {
				return err
			}
		case models.TxType_REVEAL_PROCESS_KEYS:
			if tx.KeyIndex == nil {
				return fmt.Errorf("missing keyIndex on AdminTxCheck")
			}
			// check process is finished
			if height < process.StartBlock+process.BlockCount &&
				!(process.Status == models.ProcessStatus_ENDED ||
					process.Status == models.ProcessStatus_CANCELED) {
				return fmt.Errorf("cannot reveal keys before the process is finished")
			}
			if len(process.EncryptionPrivateKeys[*tx.KeyIndex]) > 0 {
				return fmt.Errorf("keys for process %x already revealed", tx.ProcessId)
			}
			// check the keys are valid
			if err := checkRevealProcessKeys(tx, process); err != nil {
				return err
			}
		}
	case models.TxType_ADD_ORACLE:
		// check not empty, correct length and not 0x0 addr
		if (bytes.Equal(tx.Address, []byte{})) ||
			(len(tx.Address) != types.EthereumAddressSize) ||
			(bytes.Equal(tx.Address, common.Address{}.Bytes())) {
			return fmt.Errorf("invalid oracle address: %x", tx.Address)
		}
		for idx, oracle := range oracles {
			if oracle == common.BytesToAddress(tx.Address) {
				return fmt.Errorf("oracle already added to oracle list at position %d", idx)
			}
		}
	case models.TxType_REMOVE_ORACLE:
		// check not empty, correct length and not 0x0 addr
		if (bytes.Equal(tx.Address, []byte{})) ||
			(len(tx.Address) != types.EthereumAddressSize) ||
			(bytes.Equal(tx.Address, common.Address{}.Bytes())) {
			return fmt.Errorf("invalid oracle address: %x", tx.Address)
		}
		var found bool
		for _, oracle := range oracles {
			if oracle == common.BytesToAddress(tx.Address) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("cannot remove oracle, not found")
		}
	}
	return nil
}

func checkAddProcessKeys(tx *models.AdminTx, process *models.Process) error {
	if tx.KeyIndex == nil {
		return fmt.Errorf("key index is nil")
	}
	// check if at leat 1 key is provided and the keyIndex do not over/under flow
	if (tx.EncryptionPublicKey == nil) ||
		*tx.KeyIndex < 1 || *tx.KeyIndex > types.KeyKeeperMaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex is not already used
	if len(process.EncryptionPublicKeys[*tx.KeyIndex]) > 0 {
		return fmt.Errorf("key index %d already exists", tx.KeyIndex)
	}
	// TBD check that provided keys are correct (ed25519 for encryption and size for Commitment)
	return nil
}

func checkRevealProcessKeys(tx *models.AdminTx, process *models.Process) error {
	if tx.KeyIndex == nil {
		return fmt.Errorf("key index is nil")
	}
	// check if at leat 1 key is provided and the keyIndex do not over/under flow
	if (tx.EncryptionPrivateKey == nil) ||
		*tx.KeyIndex < 1 || *tx.KeyIndex > types.KeyKeeperMaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex exists
	if len(process.EncryptionPublicKeys[*tx.KeyIndex]) < 1 {
		return fmt.Errorf("key index %d does not exist", *tx.KeyIndex)
	}
	// check keys actually work
	if tx.EncryptionPrivateKey != nil {
		if priv, err := nacl.DecodePrivate(fmt.Sprintf("%x", tx.EncryptionPrivateKey)); err == nil {
			pub := priv.Public().Bytes()
			if fmt.Sprintf("%x", pub) != process.EncryptionPublicKeys[*tx.KeyIndex] {
				log.Debugf("%x != %s", pub, process.EncryptionPublicKeys[*tx.KeyIndex])
				return fmt.Errorf("the provided private key does not match "+
					"with the stored public key for index %d", *tx.KeyIndex)
			}
		} else {
			return err
		}
	}
	return nil
}
