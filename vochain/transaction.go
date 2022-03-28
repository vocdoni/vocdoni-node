package vochain

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	tmtypes "github.com/tendermint/tendermint/types"
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

// ErrorAlreadyExistInCache is returned if the transaction has been already processed
// and stored in the vote cache
var ErrorAlreadyExistInCache = fmt.Errorf("vote already exist in cache")

// VochainTx is a wrapper around a protobuf transaction with some helpers
type VochainTx struct {
	Tx         *models.Tx
	SignedBody []byte
	Signature  []byte
	TxID       [32]byte
}

// Unmarshal unarshal the content of a bytes serialized transaction.
// Returns the transaction struct, the original bytes and the signature
// of those bytes.
func (tx *VochainTx) Unmarshal(content []byte, chainID string) error {
	stx := new(models.SignedTx)
	if err := proto.Unmarshal(content, stx); err != nil {
		return err
	}
	tx.Tx = new(models.Tx)
	if err := proto.Unmarshal(stx.GetTx(), tx.Tx); err != nil {
		return err
	}
	tx.Signature = stx.GetSignature()
	tx.TxID = TxKey(content)
	tx.SignedBody = ethereum.BuildVocdoniTransaction(stx.GetTx(), chainID)
	return nil
}

// TxKey computes the checksum of the tx
func TxKey(tx tmtypes.Tx) [32]byte {
	return sha256.Sum256(tx)
}

// AddTx check the validity of a transaction and adds it to the state if commit=true.
// It returns a bytes value which depends on the transaction type:
//  Tx_Vote: vote nullifier
//  default: []byte{}
func (app *BaseApplication) AddTx(vtx *VochainTx, commit bool) ([]byte, error) {
	if vtx.Tx == nil || app.State == nil || vtx.Tx.Payload == nil {
		return nil, fmt.Errorf("transaction, state, and/or transaction payload is nil")
	}
	switch vtx.Tx.Payload.(type) {
	case *models.Tx_Vote:
		// get VoteEnvelope from tx
		txVote := vtx.Tx.GetVote()
		v, err := app.VoteEnvelopeCheck(txVote, vtx.SignedBody, vtx.Signature, vtx.TxID, commit)
		if err != nil || v == nil {
			return nil, fmt.Errorf("voteTxCheck: %w", err)
		}
		if commit {
			return v.Nullifier, app.State.AddVote(v)
		}
		return v.Nullifier, nil
	case *models.Tx_Admin:
		if err := AdminTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State); err != nil {
			return nil, fmt.Errorf("adminTxChek: %w", err)
		}
		tx := vtx.Tx.GetAdmin()
		if commit {
			switch tx.Txtype {
			case models.TxType_ADD_ORACLE:
				return vtx.TxID[:], app.State.AddOracle(common.BytesToAddress(tx.Address))
			case models.TxType_REMOVE_ORACLE:
				return vtx.TxID[:], app.State.RemoveOracle(common.BytesToAddress(tx.Address))
			case models.TxType_ADD_VALIDATOR:
				pk, err := hexPubKeyToTendermintEd25519(fmt.Sprintf("%x", tx.PublicKey))
				if err == nil {
					if tx.Power == nil {
						return nil, fmt.Errorf("power not specified on addValidator transaction")
					}
					validator := &models.Validator{
						Address: pk.Address().Bytes(),
						PubKey:  pk.Bytes(),
						Power:   *tx.Power,
					}
					return vtx.TxID[:], app.State.AddValidator(validator)

				}
				return nil, fmt.Errorf("addValidator: %w", err)

			case models.TxType_REMOVE_VALIDATOR:
				return vtx.TxID[:], app.State.RemoveValidator(tx.Address)
			case models.TxType_ADD_PROCESS_KEYS:
				return vtx.TxID[:], app.State.AddProcessKeys(tx)
			case models.TxType_REVEAL_PROCESS_KEYS:
				return vtx.TxID[:], app.State.RevealProcessKeys(tx)
			}
		}

	case *models.Tx_NewProcess:
		if p, txSender, err := app.NewProcessTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State); err == nil {
			if commit {
				tx := vtx.Tx.GetNewProcess()
				if tx.Process == nil {
					return nil, fmt.Errorf("newProcess process is empty")
				}
				if err := app.State.AddProcess(p); err != nil {
					return nil, fmt.Errorf("newProcess: addProcess: %s", err)
				}
				return vtx.TxID[:], app.State.SubtractCostIncrementNonce(txSender, models.TxType_NEW_PROCESS)
			}
		} else {
			return nil, fmt.Errorf("newProcess: %w", err)
		}

	case *models.Tx_SetProcess:
		txSender, err := SetProcessTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("setProcess: %w", err)
		}
		if commit {
			tx := vtx.Tx.GetSetProcess()
			switch tx.Txtype {
			case models.TxType_SET_PROCESS_STATUS:
				if tx.GetStatus() == models.ProcessStatus_PROCESS_UNKNOWN {
					return nil, fmt.Errorf("set process status, status unknown")
				}
				if err := app.State.SetProcessStatus(tx.ProcessId, *tx.Status, true); err != nil {
					return nil, fmt.Errorf("setProcessStatus: %s", err)
				}
			case models.TxType_SET_PROCESS_RESULTS:
				if tx.GetResults() == nil {
					return nil, fmt.Errorf("set process results, results is nil")
				}
				if err := app.State.SetProcessResults(tx.ProcessId, tx.Results, true); err != nil {
					return nil, fmt.Errorf("setProcessResults: %s", err)
				}
			case models.TxType_SET_PROCESS_CENSUS:
				if tx.GetCensusRoot() == nil {
					return nil, fmt.Errorf("set process census, census root is nil")
				}
				if err := app.State.SetProcessCensus(tx.ProcessId, tx.CensusRoot, tx.GetCensusURI(), true); err != nil {
					return nil, fmt.Errorf("setProcessCensus: %s", err)
				}
			default:
				return nil, fmt.Errorf("unknown set process tx type")
			}
			return vtx.TxID[:], app.State.SubtractCostIncrementNonce(txSender, tx.Txtype)
		}

	case *models.Tx_RegisterKey:
		if err := app.State.RegisterKeyTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State,
			commit); err != nil {
			return nil, fmt.Errorf("registerKeyTx %w", err)
		}
		if commit {
			tx := vtx.Tx.GetRegisterKey()
			weight, ok := new(big.Int).SetString(tx.Weight, 10)
			if !ok {
				return nil, fmt.Errorf("cannot parse weight %s", weight)
			}
			return vtx.TxID[:], app.State.AddToRollingCensus(tx.ProcessId, tx.NewKey, weight)
		}

	case *models.Tx_SetAccountInfo:
		txValues, err := SetAccountInfoTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("setAccountInfoTxCheck: %w", err)
		}
		if commit {
			tx := vtx.Tx.GetSetAccountInfo()
			// create account
			if txValues.CreateAccount {
				// with faucet payload provided
				if txValues.CreateAccountWithFaucet {
					// create account
					if err := app.State.CreateAccount(
						txValues.TxSender,
						tx.GetInfoURI(),
						make([]common.Address, 0),
						0,
					); err != nil {
						return nil, fmt.Errorf("setAccountInfoTxCheck: createAccount %w", err)
					}
					// consume provided faucet payload
					return vtx.TxID[:], app.State.ConsumeFaucetPayload(
						txValues.FaucetPayloadSigner,
						&models.FaucetPayload{
							Identifier: tx.GetFaucetPackage().GetPayload().GetIdentifier(),
							To:         tx.GetFaucetPackage().GetPayload().GetTo(),
							Amount:     tx.GetFaucetPackage().GetPayload().GetAmount(),
						},
						models.TxType_SET_ACCOUNT_INFO,
					)
				}
				// any faucet payload provided, just create account
				return vtx.TxID[:], app.State.CreateAccount(txValues.TxSender,
					tx.GetInfoURI(),
					make([]common.Address, 0),
					0,
				)
			}
			// account already created, change infoURI
			if err := app.State.SetAccountInfoURI(
				txValues.Account,
				tx.GetInfoURI(),
			); err != nil {
				return nil, fmt.Errorf("setAccountInfoURI: %w", err)
			}
			// subtract tx costs and increment nonce
			return vtx.TxID[:], app.State.SubtractCostIncrementNonce(txValues.TxSender, models.TxType_SET_ACCOUNT_INFO)
		}

	case *models.Tx_SetTransactionCosts:
		cost, err := SetTransactionCostsTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("setTransactionCostsTx: %w", err)
		}
		if commit {
			if err := app.State.SetTxCost(vtx.Tx.GetSetTransactionCosts().Txtype, cost); err != nil {
				return nil, fmt.Errorf("setTransactionCosts: %w", err)
			}
			return vtx.TxID[:], app.State.IncrementTreasurerNonce()
		}

	case *models.Tx_MintTokens:
		address, amount, err := MintTokensTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("mintTokensTx: %w", err)
		}
		if commit {
			if err := app.State.MintBalance(address, amount); err != nil {
				return nil, fmt.Errorf("mintTokensTx: %w", err)
			}
			return vtx.TxID[:], app.State.IncrementTreasurerNonce()
		}
	case *models.Tx_SendTokens:
		txValues, err := SendTokensTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("sendTokensTxCheck: %w", err)
		}
		if commit {
			err := app.State.TransferBalance(txValues.From, txValues.To, txValues.Value)
			if err != nil {
				return nil, fmt.Errorf("sendTokensTx: transferBalance: %w", err)
			}
			// subtract tx costs and increment nonce
			return vtx.TxID[:], app.State.SubtractCostIncrementNonce(txValues.From, models.TxType_SEND_TOKENS)
		}
	case *models.Tx_SetAccountDelegateTx:
		txValues, err := SetAccountDelegateTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("setAccountDelegateTxCheck: %w", err)
		}
		if commit {
			switch vtx.Tx.GetSetAccountDelegateTx().Txtype {
			case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
				if err := app.State.SetAccountDelegate(
					txValues.From,
					txValues.Delegate,
					models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
				); err != nil {
					return nil, fmt.Errorf("setAccountDelegate: %w", err)
				}
				err = app.State.SubtractCostIncrementNonce(txValues.From, models.TxType_ADD_DELEGATE_FOR_ACCOUNT)
			case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
				if err := app.State.SetAccountDelegate(
					txValues.From,
					txValues.Delegate,
					models.TxType_DEL_DELEGATE_FOR_ACCOUNT,
				); err != nil {
					return nil, fmt.Errorf("setAccountDelegate: %w", err)
				}
				err = app.State.SubtractCostIncrementNonce(txValues.From, models.TxType_DEL_DELEGATE_FOR_ACCOUNT)
			default:
				return nil, fmt.Errorf("setAccountDelegate: invalid transaction type")
			}
			return vtx.TxID[:], err
		}
	case *models.Tx_CollectFaucet:
		fromAcc, err := CollectFaucetTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("collectFaucetTxCheck: %w", err)
		}
		if commit {
			txValues := vtx.Tx.GetCollectFaucet()
			if err := app.State.ConsumeFaucetPayload(
				*fromAcc,
				&models.FaucetPayload{
					Identifier: txValues.FaucetPackage.Payload.Identifier,
					To:         txValues.FaucetPackage.Payload.To,
					Amount:     txValues.FaucetPackage.Payload.Amount,
				},
				models.TxType_COLLECT_FAUCET,
			); err != nil {
				return nil, fmt.Errorf("collectFaucetTx: %w", err)
			}
			// subtract tx costs and increment nonce
			return vtx.TxID[:], app.State.SubtractCostIncrementNonce(*fromAcc, models.TxType_COLLECT_FAUCET)
		}
	default:
		return nil, fmt.Errorf("invalid transaction type")
	}
	return vtx.TxID[:], nil
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

		// if vote is in cache, lazy check
		if vote != nil {
			// if not forCommit, it is a mempool check,
			// reject it since we already processed the transaction before.
			if !forCommit {
				return nil, ErrorAlreadyExistInCache
			}

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

		// check if vote already exists
		if exist, err := app.State.EnvelopeExists(ve.ProcessId,
			ve.Nullifier, false); err != nil || exist {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("vote %x already exists", ve.Nullifier)
		}
		log.Debugf("new zk vote %x for process %x", ve.Nullifier, ve.ProcessId)

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
	} else { // Signature based voting
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
		// Warning: vote cache might change during the execution of this function
		vote = app.State.CacheGetCopy(txID)

		// if the vote exists in cache
		if vote != nil {
			// if not forCommit, it is a mempool check,
			// reject it since we already processed the transaction before.
			if !forCommit {
				return nil, fmt.Errorf("vote %x already exists in cache", vote.Nullifier)
			}

			// if we are on DeliverTx and the vote is in cache, lazy check
			defer app.State.CacheDel(txID)
			vote.Height = height // update vote height
			if exist, err := app.State.EnvelopeExists(vote.ProcessId,
				vote.Nullifier, false); err != nil || exist {
				if err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("vote %x already exists", vote.Nullifier)
			}
			if height > process.GetStartBlock()+process.GetBlockCount() ||
				process.GetStatus() != models.ProcessStatus_READY {
				return nil, fmt.Errorf("vote %x is not longer valid", vote.Nullifier)
			}
			return vote, nil
		}

		// if not in cache, full check
		// extract pubKey, generate nullifier and check census proof.
		// add the transaction in the cache
		vote = &models.Vote{
			Height:      height,
			ProcessId:   ve.ProcessId,
			VotePackage: ve.VotePackage,
		}

		// check proof is nil
		if ve.Proof == nil {
			return nil, fmt.Errorf("proof not found on transaction")
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
	}
	if !forCommit {
		// add the vote to cache
		app.State.CacheAdd(txID, vote)
	}
	return vote, nil
}

// AdminTxCheck is an abstraction of ABCI checkTx for an admin transaction
func AdminTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	if vtx == nil {
		return ErrNilTx
	}
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
	if tx == nil {
		return ErrNilTx
	}
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
	if tx == nil {
		return ErrNilTx
	}
	if process == nil {
		return fmt.Errorf("process is nil")
	}
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

// SetTransactionCostsTxCheck is an abstraction of ABCI checkTx for a SetTransactionCosts transaction
func SetTransactionCostsTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (uint64, error) {
	if vtx == nil {
		return 0, ErrNilTx
	}
	tx := vtx.GetSetTransactionCosts()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return 0, fmt.Errorf("missing signature and/or transaction")
	}
	// get treasurer
	treasurer, err := state.Treasurer(false)
	if err != nil {
		return 0, err
	}
	// check nonce
	if tx.Nonce != treasurer.Nonce {
		return 0, fmt.Errorf("invalid nonce %d, expected: %d", tx.Nonce, treasurer.Nonce)
	}
	// check valid tx type
	if _, ok := TxTypeCostToStateKey[tx.Txtype]; !ok {
		return 0, fmt.Errorf("tx type not supported")
	}
	// get address from signature
	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return 0, fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	sigAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return 0, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	// check signature recovered address
	if common.BytesToAddress(treasurer.Address) != sigAddress {
		return 0, fmt.Errorf("address recovered not treasurer: expected %s got %s", treasurer.String(), sigAddress.String())
	}
	return tx.Value, nil
}
