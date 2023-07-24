package transaction

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	snarkTypes "github.com/vocdoni/go-snark/types"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/vochain/ist"
	"go.vocdoni.io/dvote/vochain/state"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

var (
	// ErrNilTx is returned if the transaction is nil.
	ErrNilTx = fmt.Errorf("nil transaction")
	// ErrInvalidURILength is returned if the entity URI length is invalid.
	ErrInvalidURILength = fmt.Errorf("invalid URI length")
	// ErrorAlreadyExistInCache is returned if the transaction has been already processed
	// and stored in the vote cache.
	ErrorAlreadyExistInCache = fmt.Errorf("vote already exist in cache")
)

// TransactionResponse is the response of a transaction check.
type TransactionResponse struct {
	TxHash []byte
	Data   []byte
	Log    string
}

// TransactionHandler holds the methods for checking the correctness of a transaction.
type TransactionHandler struct {
	// state is the state of the vochain
	state *vstate.State
	// istc is the internal state transition controller
	istc *ist.Controller
	// dataDir is the path for storing some files
	dataDir string
	// ZkVKs contains the VerificationKey for each circuit parameters index
	ZkVKs []*snarkTypes.Vk

	ZkCircuit *circuit.ZkCircuit
}

// NewTransactionHandler creates a new TransactionHandler.
func NewTransactionHandler(state *vstate.State, istc *ist.Controller, dataDir string) (*TransactionHandler, error) {
	return &TransactionHandler{
		state:   state,
		dataDir: dataDir,
		istc:    istc,
	}, nil
}

func (t *TransactionHandler) LoadZkCircuit(configTag string) error {
	circuit, err := circuit.LoadZkCircuitByTag(configTag)
	if err != nil {
		return fmt.Errorf("could not load zk verification keys: %w", err)
	}
	t.ZkCircuit = circuit
	return nil
}

// CheckTx check the validity of a transaction and adds it to the state if forCommit=true.
// It returns a bytes value which depends on the transaction type:
//
//	Tx_Vote: vote nullifier
//	default: []byte{}
func (t *TransactionHandler) CheckTx(vtx *vochaintx.Tx, forCommit bool) (*TransactionResponse, error) {
	if vtx.Tx == nil || vtx.Tx.Payload == nil {
		return nil, fmt.Errorf("transaction is empty")
	}
	response := &TransactionResponse{
		TxHash: vtx.TxID[:],
	}
	switch vtx.Tx.Payload.(type) {
	case *models.Tx_Vote:
		v, err := t.VoteTxCheck(vtx, forCommit)
		if err != nil || v == nil {
			return nil, fmt.Errorf("voteTx: %w", err)
		}
		response.Data = v.Nullifier
		if forCommit {
			return response, t.state.AddVote(v)
		}

	case *models.Tx_Admin:
		_, err := t.AdminTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("adminTx: %w", err)
		}
		if forCommit {
			tx := vtx.Tx.GetAdmin()
			switch tx.Txtype {
			// TODO: @jordipainan No cost applied, no nonce increased
			case models.TxType_ADD_PROCESS_KEYS:
				if err := t.state.AddProcessKeys(tx); err != nil {
					return nil, fmt.Errorf("addProcessKeys: %w", err)
				}
			// TODO: @jordipainan No cost applied, no nonce increased
			case models.TxType_REVEAL_PROCESS_KEYS:
				if err := t.state.RevealProcessKeys(tx); err != nil {
					return nil, fmt.Errorf("revealProcessKeys: %w", err)
				}
			default:
				return nil, fmt.Errorf("tx not supported")
			}
		}

	case *models.Tx_NewProcess:
		p, txSender, err := t.NewProcessTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("newProcessTx: %w", err)
		}
		response.Data = p.ProcessId
		if forCommit {
			tx := vtx.Tx.GetNewProcess()
			if tx.Process == nil {
				return nil, fmt.Errorf("newProcess process is empty")
			}
			if err := t.state.AddProcess(p); err != nil {
				return nil, fmt.Errorf("newProcessTx: addProcess: %w", err)
			}
			entityAddr := common.BytesToAddress(p.EntityId)
			if err := t.state.IncrementAccountProcessIndex(entityAddr); err != nil {
				return nil, fmt.Errorf("newProcessTx: cannot increment process index: %w", err)
			}
			// schedule end process on the ISTC
			if err := t.istc.Schedule(p.StartBlock+p.BlockCount, p.ProcessId, ist.Action{
				ID:         ist.ActionEndProcess,
				ElectionID: p.ProcessId,
			}); err != nil {
				return nil, fmt.Errorf("newProcessTx: cannot schedule end process: %w", err)
			}
			return response, t.state.BurnTxCostIncrementNonce(
				common.Address(txSender),
				models.TxType_NEW_PROCESS,
				t.txElectionCostFromProcess(p),
			)
		}

	case *models.Tx_SetProcess:
		txSender, err := t.SetProcessTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("setProcessTx: %w", err)
		}
		if forCommit {
			tx := vtx.Tx.GetSetProcess()
			switch tx.Txtype {
			case models.TxType_SET_PROCESS_STATUS:
				if tx.GetStatus() == models.ProcessStatus_PROCESS_UNKNOWN {
					return nil, fmt.Errorf("set process status, status unknown")
				}
				if err := t.state.SetProcessStatus(tx.ProcessId, tx.GetStatus(), true); err != nil {
					return nil, fmt.Errorf("setProcessStatus: %s", err)
				}
				if tx.GetStatus() == models.ProcessStatus_ENDED {
					// schedule results computations on the ISTC
					if err := t.istc.Schedule(t.state.CurrentHeight()+1, tx.ProcessId, ist.Action{
						ID:         ist.ActionComputeResults,
						ElectionID: tx.ProcessId,
					}); err != nil {
						return nil, fmt.Errorf("setProcessTx: cannot schedule end process: %w", err)
					}
				}

			case models.TxType_SET_PROCESS_CENSUS:
				if tx.GetCensusRoot() == nil {
					return nil, fmt.Errorf("set process census, census root is nil")
				}
				if err := t.state.SetProcessCensus(tx.ProcessId, tx.CensusRoot, tx.GetCensusURI(), true); err != nil {
					return nil, fmt.Errorf("setProcessCensus: %s", err)
				}
			default:
				return nil, fmt.Errorf("unknown set process tx type")
			}
			return response, t.state.BurnTxCostIncrementNonce(common.Address(txSender), tx.Txtype, 0)
		}

	case *models.Tx_RegisterKey:
		if err := t.RegisterKeyTxCheck(vtx, forCommit); err != nil {
			return nil, fmt.Errorf("registerKeyTx: %w", err)
		}
		if forCommit {
			tx := vtx.Tx.GetRegisterKey()
			weight, ok := new(big.Int).SetString(tx.Weight, 10)
			if !ok {
				return nil, fmt.Errorf("cannot parse weight %s", weight)
			}
			return response, t.state.AddToRollingCensus(tx.ProcessId, tx.NewKey, weight)
		}

	case *models.Tx_SetAccount:
		tx := vtx.Tx.GetSetAccount()

		var err error
		var sik state.SIK
		switch tx.Txtype {
		case models.TxType_CREATE_ACCOUNT:
			if sik, err = t.CreateAccountTxCheck(vtx); err != nil {
				return nil, fmt.Errorf("createAccountTx: %w", err)
			}

		case models.TxType_SET_ACCOUNT_INFO_URI:
			if err := t.SetAccountInfoTxCheck(vtx); err != nil {
				return nil, fmt.Errorf("setAccountInfoTx: %w", err)
			}

		case models.TxType_ADD_DELEGATE_FOR_ACCOUNT, models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
			if err := t.SetAccountDelegateTxCheck(vtx); err != nil {
				return nil, fmt.Errorf("setAccountDelegateTx: %w", err)
			}

		default:
			return nil, fmt.Errorf("setAccount: invalid transaction type")
		}

		if forCommit {
			switch tx.Txtype {
			case models.TxType_CREATE_ACCOUNT:
				txSenderAddress, err := ethereum.AddrFromSignature(vtx.SignedBody, vtx.Signature)
				if err != nil {
					return nil, fmt.Errorf("createAccountTx: txSenderAddress %w", err)
				}
				if err := t.state.CreateAccount(
					txSenderAddress,
					tx.GetInfoURI(),
					tx.GetDelegates(),
					0,
				); err != nil {
					return nil, fmt.Errorf("setAccountTx: createAccount %w", err)
				}
				// if the CreateAccountTxCheck returns a valid sik, try to
				// persist it in the state.
				if sik != nil {
					if err := t.state.SetAddressSIK(txSenderAddress, sik); err != nil {
						return nil, fmt.Errorf("setAccountTx: setSik %w", err)
					}
				}
				if tx.FaucetPackage != nil {
					faucetIssuerAddress, err := ethereum.AddrFromSignature(tx.FaucetPackage.Payload, tx.FaucetPackage.Signature)
					if err != nil {
						return nil, fmt.Errorf("createAccountTx: faucetIssuerAddress %w", err)
					}
					txCost, err := t.state.TxBaseCost(models.TxType_CREATE_ACCOUNT, false)
					if err != nil {
						return nil, fmt.Errorf("createAccountTx: txCost %w", err)
					}
					if txCost != 0 {
						if err := t.state.BurnTxCost(faucetIssuerAddress, txCost); err != nil {
							return nil, fmt.Errorf("setAccountTx: burnTxCost %w", err)
						}
					}
					faucetPayload := &models.FaucetPayload{}
					if err := proto.Unmarshal(tx.FaucetPackage.Payload, faucetPayload); err != nil {
						return nil, fmt.Errorf("createAccountTx: cannot unmarshal faucetPayload %w", err)
					}
					if err := t.state.ConsumeFaucetPayload(
						faucetIssuerAddress,
						faucetPayload,
					); err != nil {
						return nil, fmt.Errorf("setAccountTx: consumeFaucet %w", err)
					}
					if err := t.state.TransferBalance(&vochaintx.TokenTransfer{
						FromAddress: faucetIssuerAddress,
						ToAddress:   txSenderAddress,
						Amount:      faucetPayload.Amount,
						TxHash:      vtx.TxID[:],
					}, false); err != nil {
						return nil, fmt.Errorf("setAccountTx: transferBalance %w", err)
					}
					// transfer balance from faucet package issuer to created account
					return response, nil
				}
				return response, nil

			case models.TxType_SET_ACCOUNT_INFO_URI:
				txSenderAddress, err := ethereum.AddrFromSignature(vtx.SignedBody, vtx.Signature)
				if err != nil {
					return nil, fmt.Errorf("createAccountTx: txSenderAddress %w", err)
				}
				// consume cost for setAccount
				if err := t.state.BurnTxCostIncrementNonce(
					txSenderAddress,
					models.TxType_SET_ACCOUNT_INFO_URI,
					0,
				); err != nil {
					return nil, fmt.Errorf("setAccountTx: burnCostIncrementNonce %w", err)
				}
				txAccount := common.BytesToAddress(tx.GetAccount())
				if txAccount != (common.Address{}) {
					return response, t.state.SetAccountInfoURI(
						txAccount,
						tx.GetInfoURI(),
					)
				}
				return response, t.state.SetAccountInfoURI(
					txSenderAddress,
					tx.GetInfoURI(),
				)

			case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
				txSenderAddress, err := ethereum.AddrFromSignature(vtx.SignedBody, vtx.Signature)
				if err != nil {
					return nil, fmt.Errorf("createAccountTx: txSenderAddress %w", err)
				}
				if err := t.state.BurnTxCostIncrementNonce(
					txSenderAddress,
					models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
					0,
				); err != nil {
					return nil, fmt.Errorf("setAccountDelegateTx: burnTxCostIncrementNonce %w", err)
				}
				if err := t.state.SetAccountDelegate(
					txSenderAddress,
					tx.Delegates,
					models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
				); err != nil {
					return nil, fmt.Errorf("setAccountDelegateTx: %w", err)
				}
			case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
				txSenderAddress, err := ethereum.AddrFromSignature(vtx.SignedBody, vtx.Signature)
				if err != nil {
					return nil, fmt.Errorf("createAccountTx: txSenderAddress %w", err)
				}
				if err := t.state.BurnTxCostIncrementNonce(
					txSenderAddress,
					models.TxType_DEL_DELEGATE_FOR_ACCOUNT,
					0,
				); err != nil {
					return nil, fmt.Errorf("setAccountDelegate: burnTxCostIncrementNonce %w", err)
				}
				if err := t.state.SetAccountDelegate(
					txSenderAddress,
					tx.Delegates,
					models.TxType_DEL_DELEGATE_FOR_ACCOUNT,
				); err != nil {
					return nil, fmt.Errorf("setAccountDelegateTx: %w", err)
				}
			default:
				return nil, fmt.Errorf("setAccount: invalid transaction type")
			}
		}

	case *models.Tx_SetTransactionCosts:
		cost, err := t.SetTransactionCostsTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("setTransactionCostsTx: %w", err)
		}
		if forCommit {
			if err := t.state.SetTxBaseCost(vtx.Tx.GetSetTransactionCosts().Txtype, cost); err != nil {
				return nil, fmt.Errorf("setTransactionCostsTx: %w", err)
			}
			return response, t.state.IncrementTreasurerNonce()
		}

	case *models.Tx_MintTokens:
		err := t.MintTokensTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("mintTokensTx: %w", err)
		}
		if forCommit {
			tx := vtx.Tx.GetMintTokens()
			treasurer, err := t.state.Treasurer(true)
			if err != nil {
				return nil, fmt.Errorf("mintTokensTx: %w", err)
			}
			if err := t.state.MintBalance(&vochaintx.TokenTransfer{
				FromAddress: common.BytesToAddress(treasurer.Address),
				ToAddress:   common.BytesToAddress(tx.To),
				Amount:      tx.Value,
				TxHash:      vtx.TxID[:],
			}); err != nil {
				return nil, fmt.Errorf("mintTokensTx: %w", err)
			}
			return response, t.state.IncrementTreasurerNonce()
		}

	case *models.Tx_SendTokens:
		err := t.SendTokensTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("sendTokensTx: %w", err)
		}
		if forCommit {
			tx := vtx.Tx.GetSendTokens()
			from, to := common.BytesToAddress(tx.From), common.BytesToAddress(tx.To)
			err := t.state.BurnTxCostIncrementNonce(from, models.TxType_SEND_TOKENS, 0)
			if err != nil {
				return nil, fmt.Errorf("sendTokensTx: burnTxCostIncrementNonce %w", err)
			}
			if err := t.state.TransferBalance(&vochaintx.TokenTransfer{
				FromAddress: from,
				ToAddress:   to,
				Amount:      tx.Value,
				TxHash:      vtx.TxID[:],
			}, false); err != nil {
				return nil, fmt.Errorf("sendTokensTx: %w", err)
			}
			return response, nil
		}

	case *models.Tx_CollectFaucet:
		err := t.CollectFaucetTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("collectFaucetTxCheck: %w", err)
		}
		if forCommit {
			tx := vtx.Tx.GetCollectFaucet()
			issuerAddress, err := ethereum.AddrFromSignature(tx.FaucetPackage.Payload, tx.FaucetPackage.Signature)
			if err != nil {
				return nil, fmt.Errorf("collectFaucetTx: cannot get issuerAddress %w", err)
			}
			if err := t.state.BurnTxCostIncrementNonce(issuerAddress, models.TxType_COLLECT_FAUCET, 0); err != nil {
				return nil, fmt.Errorf("collectFaucetTx: burnTxCost %w", err)
			}
			faucetPayload := &models.FaucetPayload{}
			if err := proto.Unmarshal(tx.FaucetPackage.Payload, faucetPayload); err != nil {
				return nil, fmt.Errorf("could not unmarshal faucet package: %w", err)
			}
			if err := t.state.ConsumeFaucetPayload(
				issuerAddress,
				&models.FaucetPayload{
					Identifier: faucetPayload.Identifier,
					To:         faucetPayload.To,
					Amount:     faucetPayload.Amount,
				},
			); err != nil {
				return nil, fmt.Errorf("collectFaucetTx: %w", err)
			}
			if err := t.state.TransferBalance(&vochaintx.TokenTransfer{
				FromAddress: issuerAddress,
				ToAddress:   common.BytesToAddress(faucetPayload.To),
				Amount:      faucetPayload.Amount,
				TxHash:      vtx.TxID[:],
			}, false); err != nil {
				return nil, fmt.Errorf("collectFaucetTx: %w", err)
			}
			return response, nil
		}

	case *models.Tx_SetSik:
		addr, newSik, err := t.SetSikTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("setSikTx: %w", err)
		}
		if forCommit {
			if err := t.state.SetAddressSIK(addr, newSik); err != nil {
				return nil, fmt.Errorf("setSikTx: %w", err)
			}
		}
		return response, nil

	case *models.Tx_DelSik:
		address, err := t.DelSikTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("setSikTx: %w", err)
		}
		if forCommit {
			if err := t.state.InvalidateSIK(address); err != nil {
				return nil, fmt.Errorf("setSikTx: %w", err)
			}
		}
		return response, nil

	default:
		return nil, fmt.Errorf("invalid transaction type")
	}

	return response, nil
}
