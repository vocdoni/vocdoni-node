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

// AddTxResponse is the data returned by AddTx()
type AddTxResponse struct {
	TxHash []byte
	Data   []byte
	Log    string
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
func (app *BaseApplication) AddTx(vtx *VochainTx, commit bool) (*AddTxResponse, error) {
	if vtx.Tx == nil || app.State == nil || vtx.Tx.Payload == nil {
		return nil, fmt.Errorf("transaction, state, and/or transaction payload is nil")
	}
	response := &AddTxResponse{TxHash: vtx.TxID[:]}
	switch vtx.Tx.Payload.(type) {
	case *models.Tx_Vote:
		// get VoteEnvelope from tx
		txVote := vtx.Tx.GetVote()
		v, voterID, err := app.VoteEnvelopeCheck(txVote, vtx.SignedBody, vtx.Signature, vtx.TxID, commit)
		if err != nil || v == nil {
			return nil, fmt.Errorf("voteTxCheck: %w", err)
		}
		response.Data = v.Nullifier
		if commit {
			return response, app.State.AddVote(v, voterID)
		}

	case *models.Tx_Admin:
		_, err := AdminTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("adminTxCheck: %w", err)
		}
		if commit {
			tx := vtx.Tx.GetAdmin()
			switch tx.Txtype {
			case models.TxType_ADD_ORACLE:
				if err := app.State.AddOracle(common.BytesToAddress(tx.Address)); err != nil {
					return nil, fmt.Errorf("addOracle: %w", err)
				}
				return response, app.State.IncrementTreasurerNonce()
			case models.TxType_REMOVE_ORACLE:
				if err := app.State.RemoveOracle(common.BytesToAddress(tx.Address)); err != nil {
					return nil, fmt.Errorf("removeOracle: %w", err)
				}
				return response, app.State.IncrementTreasurerNonce()
			// TODO: @jordipainan No cost applied, no nonce increased
			case models.TxType_ADD_PROCESS_KEYS:
				if err := app.State.AddProcessKeys(tx); err != nil {
					return nil, fmt.Errorf("addProcessKeys: %w", err)
				}
			// TODO: @jordipainan No cost applied, no nonce increased
			case models.TxType_REVEAL_PROCESS_KEYS:
				if err := app.State.RevealProcessKeys(tx); err != nil {
					return nil, fmt.Errorf("revealProcessKeys: %w", err)
				}
			default:
				return nil, fmt.Errorf("tx not supported")
			}
		}

	case *models.Tx_NewProcess:
		p, txSender, err := app.NewProcessTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("newProcess: %w", err)
		}
		response.Data = p.ProcessId
		if commit {
			tx := vtx.Tx.GetNewProcess()
			if tx.Process == nil {
				return nil, fmt.Errorf("newProcess process is empty")
			}
			if err := app.State.AddProcess(p); err != nil {
				return nil, fmt.Errorf("newProcess: addProcess: %w", err)
			}
			entityAddr := common.BytesToAddress(p.EntityId)
			if err := app.State.IncrementAccountProcessIndex(entityAddr); err != nil {
				return nil, fmt.Errorf("newProcess: cannot increment process index: %w", err)
			}
			return response, app.State.BurnTxCostIncrementNonce(txSender, models.TxType_NEW_PROCESS)
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
				if err := app.State.SetProcessStatus(tx.ProcessId, tx.GetStatus(), true); err != nil {
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
			return response, app.State.BurnTxCostIncrementNonce(txSender, tx.Txtype)
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
			return response, app.State.AddToRollingCensus(tx.ProcessId, tx.NewKey, weight)
		}

	case *models.Tx_SetAccount:
		tx := vtx.Tx.GetSetAccount()
		switch tx.Txtype {
		case models.TxType_CREATE_ACCOUNT:
			err := CreateAccountTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
			if err != nil {
				return nil, fmt.Errorf("createAccount: %w", err)
			}

		case models.TxType_SET_ACCOUNT_INFO_URI:
			err := SetAccountInfoTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
			if err != nil {
				return nil, fmt.Errorf("setAccountInfoTxCheck: %w", err)
			}

		case models.TxType_ADD_DELEGATE_FOR_ACCOUNT, models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
			err := SetAccountDelegateTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
			if err != nil {
				return nil, fmt.Errorf("setAccountDelegateTxCheck: %w", err)
			}

		default:
			return nil, fmt.Errorf("setAccount: invalid transaction type")
		}

		if commit {
			switch tx.Txtype {
			case models.TxType_CREATE_ACCOUNT:
				txSenderAddress, err := ethereum.AddrFromSignature(vtx.SignedBody, vtx.Signature)
				if err != nil {
					return nil, fmt.Errorf("createAccountTx: txSenderAddress %w", err)
				}
				if err := app.State.CreateAccount(
					txSenderAddress,
					tx.GetInfoURI(),
					tx.GetDelegates(),
				); err != nil {
					return nil, fmt.Errorf("setAccountTx: createAccount %w", err)
				}
				if tx.FaucetPackage != nil {
					faucetIssuerAddress, err := ethereum.AddrFromSignature(tx.FaucetPackage.Payload, tx.FaucetPackage.Signature)
					if err != nil {
						return nil, fmt.Errorf("createAccountTx: faucetIssuerAddress %w", err)
					}
					txCost, err := app.State.TxCost(models.TxType_CREATE_ACCOUNT, false)
					if err != nil {
						return nil, fmt.Errorf("createAccountTx: txCost %w", err)
					}
					if txCost != 0 {
						if err := app.State.BurnTxCost(faucetIssuerAddress, txCost); err != nil {
							return nil, fmt.Errorf("setAccountTx: burnTxCost %w", err)
						}
					}
					faucetPayload := &models.FaucetPayload{}
					if err := proto.Unmarshal(tx.FaucetPackage.Payload, faucetPayload); err != nil {
						return nil, fmt.Errorf("createAccountTx: cannot unmarshal faucetPayload %w", err)
					}
					if err := app.State.ConsumeFaucetPayload(
						faucetIssuerAddress,
						faucetPayload,
					); err != nil {
						return nil, fmt.Errorf("setAccountTx: consumeFaucet %w", err)
					}
					// transfer balance from faucet package issuer to created account
					return response, app.State.TransferBalance(
						faucetIssuerAddress,
						txSenderAddress,
						faucetPayload.Amount,
					)
				}
				return response, nil

			case models.TxType_SET_ACCOUNT_INFO_URI:
				txSenderAddress, err := ethereum.AddrFromSignature(vtx.SignedBody, vtx.Signature)
				if err != nil {
					return nil, fmt.Errorf("createAccountTx: txSenderAddress %w", err)
				}
				// consume cost for setAccount
				if err := app.State.BurnTxCostIncrementNonce(
					txSenderAddress,
					models.TxType_SET_ACCOUNT_INFO_URI,
				); err != nil {
					return nil, fmt.Errorf("setAccountTx: burnCostIncrementNonce %w", err)
				}
				txAccount := common.BytesToAddress(tx.GetAccount())
				if txAccount != (common.Address{}) {
					return response, app.State.SetAccountInfoURI(
						txAccount,
						tx.GetInfoURI(),
					)
				}
				return response, app.State.SetAccountInfoURI(
					txSenderAddress,
					tx.GetInfoURI(),
				)

			case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
				txSenderAddress, err := ethereum.AddrFromSignature(vtx.SignedBody, vtx.Signature)
				if err != nil {
					return nil, fmt.Errorf("createAccountTx: txSenderAddress %w", err)
				}
				if err := app.State.BurnTxCostIncrementNonce(
					txSenderAddress,
					models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
				); err != nil {
					return nil, fmt.Errorf("setAccountDelegate: burnTxCostIncrementNonce %w", err)
				}
				if err := app.State.SetAccountDelegate(
					txSenderAddress,
					tx.Delegates,
					models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
				); err != nil {
					return nil, fmt.Errorf("setAccountDelegate: %w", err)
				}
			case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
				txSenderAddress, err := ethereum.AddrFromSignature(vtx.SignedBody, vtx.Signature)
				if err != nil {
					return nil, fmt.Errorf("createAccountTx: txSenderAddress %w", err)
				}
				if err := app.State.BurnTxCostIncrementNonce(
					txSenderAddress,
					models.TxType_DEL_DELEGATE_FOR_ACCOUNT,
				); err != nil {
					return nil, fmt.Errorf("setAccountDelegate: burnTxCostIncrementNonce %w", err)
				}
				if err := app.State.SetAccountDelegate(
					txSenderAddress,
					tx.Delegates,
					models.TxType_DEL_DELEGATE_FOR_ACCOUNT,
				); err != nil {
					return nil, fmt.Errorf("setAccountDelegate: %w", err)
				}
			default:
				return nil, fmt.Errorf("setAccount: invalid transaction type")
			}
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
			return response, app.State.IncrementTreasurerNonce()
		}

	case *models.Tx_MintTokens:
		err := MintTokensTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("mintTokensTx: %w", err)
		}
		if commit {
			tx := vtx.Tx.GetMintTokens()
			if err := app.State.MintBalance(common.BytesToAddress(tx.To), tx.Value); err != nil {
				return nil, fmt.Errorf("mintTokensTx: %w", err)
			}
			return response, app.State.IncrementTreasurerNonce()
		}

	case *models.Tx_SendTokens:
		err := SendTokensTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("sendTokensTxCheck: %w", err)
		}
		if commit {
			tx := vtx.Tx.GetSendTokens()
			from, to := common.BytesToAddress(tx.From), common.BytesToAddress(tx.To)
			err := app.State.BurnTxCostIncrementNonce(from, models.TxType_SEND_TOKENS)
			if err != nil {
				return nil, fmt.Errorf("sendTokensTx: burnTxCostIncrementNonce %w", err)
			}
			return response, app.State.TransferBalance(from, to, tx.Value)
		}

	case *models.Tx_CollectFaucet:
		err := CollectFaucetTxCheck(vtx.Tx, vtx.SignedBody, vtx.Signature, app.State)
		if err != nil {
			return nil, fmt.Errorf("collectFaucetTxCheck: %w", err)
		}
		if commit {
			tx := vtx.Tx.GetCollectFaucet()
			issuerAddress, err := ethereum.AddrFromSignature(tx.FaucetPackage.Payload, tx.FaucetPackage.Signature)
			if err != nil {
				return nil, fmt.Errorf("collectFaucetTx: cannot get issuerAddress %w", err)
			}
			if err := app.State.BurnTxCostIncrementNonce(issuerAddress, models.TxType_COLLECT_FAUCET); err != nil {
				return nil, fmt.Errorf("collectFaucetTx: burnTxCost %w", err)
			}
			faucetPayload := &models.FaucetPayload{}
			if err := proto.Unmarshal(tx.FaucetPackage.Payload, faucetPayload); err != nil {
				return nil, fmt.Errorf("could not unmarshal faucet package: %w", err)
			}
			if err := app.State.ConsumeFaucetPayload(
				issuerAddress,
				&models.FaucetPayload{
					Identifier: faucetPayload.Identifier,
					To:         faucetPayload.To,
					Amount:     faucetPayload.Amount,
				},
			); err != nil {
				return nil, fmt.Errorf("collectFaucetTx: %w", err)
			}
			return response, app.State.TransferBalance(issuerAddress,
				common.BytesToAddress(faucetPayload.To),
				faucetPayload.Amount,
			)
		}

	default:
		return nil, fmt.Errorf("invalid transaction type")
	}

	return response, nil
}

// VoteEnvelopeCheck is an abstraction of ABCI checkTx for submitting a vote
// All hexadecimal strings should be already sanitized (without 0x)
func (app *BaseApplication) VoteEnvelopeCheck(ve *models.VoteEnvelope, txBytes, signature []byte,
	txID [32]byte, forCommit bool) (*models.Vote, VoterID, error) {
	// Perform basic/general checks
	voterID := VoterID{}
	if ve == nil {
		return nil, voterID.Nil(), fmt.Errorf("vote envelope is nil")
	}
	process, err := app.State.Process(ve.ProcessId, false)
	if err != nil {
		return nil, voterID.Nil(), fmt.Errorf("cannot fetch processId: %w", err)
	}
	if process == nil || process.EnvelopeType == nil || process.Mode == nil {
		return nil, voterID.Nil(), fmt.Errorf("process %x malformed", ve.ProcessId)
	}
	height := app.State.CurrentHeight()
	endBlock := process.StartBlock + process.BlockCount

	if height < process.StartBlock {
		return nil, voterID.Nil(), fmt.Errorf("process %x starts at height %d, current height is %d", ve.ProcessId, process.StartBlock, height)
	} else if height > endBlock {
		return nil, voterID.Nil(), fmt.Errorf("process %x finished at height %d, current height is %d", ve.ProcessId, endBlock, height)
	}

	if process.Status != models.ProcessStatus_READY {
		return nil, voterID.Nil(), fmt.Errorf("process %x not in READY state - current state: %s", ve.ProcessId, process.Status.String())
	}

	// Check in case of keys required, they have been sent by some keykeeper
	if process.EnvelopeType.EncryptedVotes &&
		process.KeyIndex != nil &&
		*process.KeyIndex < 1 {
		return nil, voterID.Nil(), fmt.Errorf("no keys available, voting is not possible")
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
				return nil, voterID.Nil(), ErrorAlreadyExistInCache
			}

			vote.Height = height // update vote height
			defer app.State.CacheDel(txID)
			if exist, err := app.State.EnvelopeExists(vote.ProcessId,
				vote.Nullifier, false); err != nil || exist {
				if err != nil {
					return nil, voterID.Nil(), err
				}
				return nil, voterID.Nil(), fmt.Errorf("vote %x already exists", vote.Nullifier)
			}
			return vote, voterID.Nil(), nil
		}

		// Supports Groth16 proof generated from circom snark compatible
		// prover
		proofZkSNARK := ve.Proof.GetZkSnark()
		if proofZkSNARK == nil {
			return nil, voterID.Nil(), fmt.Errorf("zkSNARK proof is empty")
		}
		proof, _, err := zk.ProtobufZKProofToCircomProof(proofZkSNARK)
		if err != nil {
			return nil, voterID.Nil(), fmt.Errorf("failed on zk.ProtobufZKProofToCircomProof: %w", err)
		}

		// ve.Nullifier is encoded in little-endian
		nullifierBI := arbo.BytesToBigInt(ve.Nullifier)

		// check if vote already exists
		if exist, err := app.State.EnvelopeExists(ve.ProcessId,
			ve.Nullifier, false); err != nil || exist {
			if err != nil {
				return nil, voterID.Nil(), err
			}
			return nil, voterID.Nil(), fmt.Errorf("vote %x already exists", ve.Nullifier)
		}
		log.Debugf("new zk vote %x for process %x", ve.Nullifier, ve.ProcessId)

		if int(proofZkSNARK.CircuitParametersIndex) >= len(app.ZkVKs) ||
			int(proofZkSNARK.CircuitParametersIndex) < 0 {
			return nil, voterID.Nil(), fmt.Errorf("invalid CircuitParametersIndex: %d of %d", proofZkSNARK.CircuitParametersIndex, len(app.ZkVKs))
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
			return nil, voterID.Nil(), fmt.Errorf("zkSNARK proof verification failed")
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
				return nil, voterID.Nil(), fmt.Errorf("no key indexes provided on vote package")
			}
			vote.EncryptionKeyIndexes = ve.EncryptionKeyIndexes
		}
	} else { // Signature based voting
		if signature == nil {
			return nil, voterID.Nil(), fmt.Errorf("signature missing on voteTx")
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
				return nil, voterID.Nil(), fmt.Errorf("vote %x already exists in cache", vote.Nullifier)
			}

			// if we are on DeliverTx and the vote is in cache, lazy check
			defer app.State.CacheDel(txID)
			vote.Height = height // update vote height
			if exist, err := app.State.EnvelopeExists(vote.ProcessId,
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
			ProcessId:   ve.ProcessId,
			VotePackage: ve.VotePackage,
		}

		// check proof is nil
		if ve.Proof == nil {
			return nil, voterID.Nil(), fmt.Errorf("proof not found on transaction")
		}
		if ve.Proof.Payload == nil {
			return nil, voterID.Nil(), fmt.Errorf("invalid proof payload provided")
		}

		// If process encrypted, check the vote is encrypted (includes at least one key index)
		if process.EnvelopeType.EncryptedVotes {
			if len(ve.EncryptionKeyIndexes) == 0 {
				return nil, voterID.Nil(), fmt.Errorf("no key indexes provided on vote package")
			}
			vote.EncryptionKeyIndexes = ve.EncryptionKeyIndexes
		}
		pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
		if err != nil {
			return nil, voterID.Nil(), fmt.Errorf("cannot extract public key from signature: %w", err)
		}
		voterID = []byte{VoterIDTypeECDSA}
		voterID = append(voterID, pubKey...)
		addr, err := ethereum.AddrFromPublicKey(pubKey)
		if err != nil {
			return nil, voterID.Nil(), fmt.Errorf("cannot extract address from public key: %w", err)
		}
		// assign a nullifier
		vote.Nullifier = GenerateNullifier(addr, vote.ProcessId)

		// check if vote already exists
		if exist, err := app.State.EnvelopeExists(vote.ProcessId,
			vote.Nullifier, false); err != nil || exist {
			if err != nil {
				return nil, voterID.Nil(), err
			}
			return nil, voterID.Nil(), fmt.Errorf("vote %x already exists", vote.Nullifier)
		}
		log.Debugf("new vote %x for address %s and process %x", vote.Nullifier, addr.Hex(), ve.ProcessId)

		valid, weight, err := VerifyProof(process, ve.Proof,
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
		app.State.CacheAdd(txID, vote)
	}
	return vote, voterID, nil
}

// AdminTxCheck is an abstraction of ABCI checkTx for an admin transaction
func AdminTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (common.Address, error) {
	if vtx == nil {
		return common.Address{}, ErrNilTx
	}
	tx := vtx.GetAdmin()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return common.Address{}, fmt.Errorf("missing signature and/or admin transaction")
	}

	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	addr, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	log.Debugf("checking admin signed tx %s by addr %x", log.FormatProto(tx), addr)

	switch tx.Txtype {
	// TODO: @jordipainan make keykeeper independent of oracles
	case models.TxType_ADD_PROCESS_KEYS, models.TxType_REVEAL_PROCESS_KEYS:
		if tx.ProcessId == nil {
			return common.Address{}, fmt.Errorf("missing processId on AdminTxCheck")
		}
		// check process exists
		process, err := state.Process(tx.ProcessId, false)
		if err != nil {
			return common.Address{}, err
		}
		if process == nil {
			return common.Address{}, fmt.Errorf("process with id (%x) does not exist", tx.ProcessId)
		}
		// check process actually requires keys
		if !process.EnvelopeType.EncryptedVotes && !process.EnvelopeType.Anonymous {
			return common.Address{}, fmt.Errorf("process does not require keys")
		}
		// get oracles
		oracles, err := state.Oracles(false)
		if err != nil || len(oracles) == 0 {
			return common.Address{}, fmt.Errorf("cannot check authorization against a nil or empty oracle list")
		}
		// check if sender authorized
		authorized, addr, err := verifySignatureAgainstOracles(oracles, txBytes, signature)
		if err != nil {
			return common.Address{}, err
		}
		if !authorized {
			return common.Address{}, fmt.Errorf("unauthorized to perform an adminTx, address: %s", addr.Hex())
		}
		// check oracle account
		oracleAcc, err := state.GetAccount(addr, false)
		if err != nil {
			return common.Address{}, fmt.Errorf("cannot get oracle account: %w", err)
		}
		if oracleAcc == nil {
			return common.Address{}, ErrAccountNotExist
		}
		/* TODO: @jordipainan activate if cost and nonce on add or reveal process keys
		if oracleAcc.Nonce != tx.Nonce {
			return common.Address{}, ErrAccountNonceInvalid
		}
			cost, err := state.TxCost(tx.Txtype, false)
			if err != nil {
				return common.Address{}, fmt.Errorf("cannot get tx %s cost", tx.Txtype.String())
			}
			if oracleAcc.Balance < cost {
				return common.Address{}, ErrNotEnoughBalance
			}
		*/
		// get current height
		height := state.CurrentHeight()
		// Specific checks
		switch tx.Txtype {
		case models.TxType_ADD_PROCESS_KEYS:
			if tx.KeyIndex == nil {
				return common.Address{}, fmt.Errorf("missing keyIndex on AdminTxCheck")
			}
			// endblock is always greater than start block so that case is also included here
			if height > process.StartBlock {
				return common.Address{}, fmt.Errorf("cannot add process keys to a process that has started or finished status (%s)", process.Status.String())
			}
			// process is not canceled
			if process.Status == models.ProcessStatus_CANCELED ||
				process.Status == models.ProcessStatus_ENDED ||
				process.Status == models.ProcessStatus_RESULTS {
				return common.Address{}, fmt.Errorf("cannot add process keys to a %s process", process.Status)
			}
			if len(process.EncryptionPublicKeys[tx.GetKeyIndex()]) > 0 {
				return common.Address{}, fmt.Errorf("keys for process %x already revealed", tx.ProcessId)
			}
			// check included keys and keyindex are valid
			if err := checkAddProcessKeys(tx, process); err != nil {
				return common.Address{}, err
			}
		case models.TxType_REVEAL_PROCESS_KEYS:
			if tx.KeyIndex == nil {
				return common.Address{}, fmt.Errorf("missing keyIndex on AdminTxCheck")
			}
			// check process is finished
			if height < process.StartBlock+process.BlockCount &&
				!(process.Status == models.ProcessStatus_ENDED ||
					process.Status == models.ProcessStatus_CANCELED) {
				return common.Address{}, fmt.Errorf("cannot reveal keys before the process is finished")
			}
			if len(process.EncryptionPrivateKeys[tx.GetKeyIndex()]) > 0 {
				return common.Address{}, fmt.Errorf("keys for process %x already revealed", tx.ProcessId)
			}
			// check the keys are valid
			if err := checkRevealProcessKeys(tx, process); err != nil {
				return common.Address{}, err
			}
		}
	case models.TxType_ADD_ORACLE:
		err := state.VerifyTreasurer(addr, tx.Nonce)
		if err != nil {
			return common.Address{}, fmt.Errorf("tx sender not authorized: %w", err)
		}
		// check not empty, correct length and not 0x0 addr
		if (bytes.Equal(tx.Address, []byte{})) ||
			(len(tx.Address) != types.EthereumAddressSize) ||
			(bytes.Equal(tx.Address, common.Address{}.Bytes())) {
			return common.Address{}, fmt.Errorf("invalid oracle address: %x", tx.Address)
		}
		oracles, err := state.Oracles(false)
		if err != nil {
			return common.Address{}, fmt.Errorf("cannot get oracles")
		}
		for idx, oracle := range oracles {
			if oracle == common.BytesToAddress(tx.Address) {
				return common.Address{}, fmt.Errorf("oracle already added to oracle list at position %d", idx)
			}
		}
	case models.TxType_REMOVE_ORACLE:
		err := state.VerifyTreasurer(addr, tx.Nonce)
		if err != nil {
			return common.Address{}, fmt.Errorf("tx sender not authorized: %w", err)
		}
		// check not empty, correct length and not 0x0 addr
		if (bytes.Equal(tx.Address, []byte{})) ||
			(len(tx.Address) != types.EthereumAddressSize) ||
			(bytes.Equal(tx.Address, common.Address{}.Bytes())) {
			return common.Address{}, fmt.Errorf("invalid oracle address: %x", tx.Address)
		}
		oracles, err := state.Oracles(false)
		if err != nil {
			return common.Address{}, fmt.Errorf("cannot get oracles")
		}
		var found bool
		for _, oracle := range oracles {
			if oracle == common.BytesToAddress(tx.Address) {
				found = true
				break
			}
		}
		if !found {
			return common.Address{}, fmt.Errorf("cannot remove oracle, not found")
		}
	default:
		return common.Address{}, fmt.Errorf("tx not supported")
	}
	return addr, nil
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
		tx.GetKeyIndex() < 1 || tx.GetKeyIndex() > types.KeyKeeperMaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex is not already used
	if len(process.EncryptionPublicKeys[tx.GetKeyIndex()]) > 0 {
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
		tx.GetKeyIndex() < 1 || tx.GetKeyIndex() > types.KeyKeeperMaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex exists
	if len(process.EncryptionPublicKeys[tx.GetKeyIndex()]) < 1 {
		return fmt.Errorf("key index %d does not exist", tx.GetKeyIndex())
	}
	// check keys actually work
	if tx.EncryptionPrivateKey != nil {
		if priv, err := nacl.DecodePrivate(fmt.Sprintf("%x", tx.EncryptionPrivateKey)); err == nil {
			pub := priv.Public().Bytes()
			if fmt.Sprintf("%x", pub) != process.EncryptionPublicKeys[tx.GetKeyIndex()] {
				log.Debugf("%x != %s", pub, process.EncryptionPublicKeys[tx.GetKeyIndex()])
				return fmt.Errorf("the provided private key does not match "+
					"with the stored public key for index %d", tx.GetKeyIndex())
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
		return 0, fmt.Errorf("address recovered not treasurer: expected %s got %s", common.BytesToAddress(treasurer.Address), sigAddress.String())
	}
	return tx.Value, nil
}
