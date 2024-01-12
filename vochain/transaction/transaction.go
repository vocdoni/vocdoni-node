package transaction

import (
	"encoding/hex"
	"fmt"

	cometCrypto256k1 "github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/ist"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	// newValidatorPower is the default power of a new validator
	newValidatorPower = 5
)

var (
	// ErrNilTx is returned if the transaction is nil.
	ErrNilTx = fmt.Errorf("nil transaction")
	// ErrInvalidURILength is returned if the entity URI length is invalid.
	ErrInvalidURILength = fmt.Errorf("invalid URI length")
	// ErrorAlreadyExistInCache is returned if the transaction has been already processed
	// and stored in the vote cache.
	ErrorAlreadyExistInCache = fmt.Errorf("transaction already exist in cache")
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
}

// NewTransactionHandler creates a new TransactionHandler.
func NewTransactionHandler(state *vstate.State, istc *ist.Controller, dataDir string) *TransactionHandler {
	return &TransactionHandler{
		state:   state,
		dataDir: dataDir,
		istc:    istc,
	}
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
	if forCommit {
		if err := t.checkAccountNonce(vtx); err != nil {
			return nil, fmt.Errorf("checkAccountNonce: %w", err)
		}
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
			if err := t.istc.Schedule(ist.Action{
				TypeID:     ist.ActionEndProcess,
				ElectionID: p.ProcessId,
				ID:         p.ProcessId,
				TimeStamp:  p.StartTime + p.Duration,
			}); err != nil {
				return nil, fmt.Errorf("newProcessTx: cannot schedule end process: %w", err)
			}
			return response, t.state.BurnTxCostIncrementNonce(
				common.Address(txSender),
				models.TxType_NEW_PROCESS,
				t.txElectionCostFromProcess(p),
				hex.EncodeToString(p.GetProcessId()),
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
					return nil, fmt.Errorf("setProcessStatus: status unknown")
				}
				if err := t.state.SetProcessStatus(tx.ProcessId, tx.GetStatus(), true); err != nil {
					return nil, fmt.Errorf("setProcessStatus: %s", err)
				}
				if tx.GetStatus() == models.ProcessStatus_ENDED {
					// purge RegisterSIKTx counter if it exists
					if err := t.state.PurgeRegisterSIK(tx.ProcessId); err != nil {
						return nil, fmt.Errorf("setProcessStatus: cannot purge RegisterSIKTx counter: %w", err)
					}
					// schedule results computations on the ISTC
					if err := t.istc.Remove(tx.ProcessId); err != nil {
						log.Errorw(err, "setProcessStatus: cannot remove IST action")
					}
					if err := t.istc.Schedule(ist.Action{
						TypeID:     ist.ActionCommitResults,
						ElectionID: tx.ProcessId,
						ID:         tx.ProcessId,
						Height:     t.state.CurrentHeight() + 1,
					}); err != nil {
						return nil, fmt.Errorf("setProcessStatus: cannot schedule commit results: %w", err)
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
			return response, t.state.BurnTxCostIncrementNonce(common.Address(txSender), tx.Txtype, 0, hex.EncodeToString(tx.ProcessId))
		}

	case *models.Tx_SetAccount:
		tx := vtx.Tx.GetSetAccount()

		switch tx.Txtype {
		case models.TxType_CREATE_ACCOUNT:
			if err := t.CreateAccountTxCheck(vtx); err != nil {
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
		case models.TxType_SET_ACCOUNT_VALIDATOR:
			if err := t.SetAccountValidatorTxCheck(vtx); err != nil {
				return nil, fmt.Errorf("setAccountValidatorTx: %w", err)
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
				// if the tx includes a sik try to persist it in the state
				if sik := tx.GetSIK(); sik != nil {
					if err := t.state.SetAddressSIK(txSenderAddress, sik); err != nil {
						return nil, fmt.Errorf("setAccountTx: SetAddressSIK %w", err)
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
					return nil, fmt.Errorf("setAccountInfo: txSenderAddress %w", err)
				}
				// consume cost for setAccount
				if err := t.state.BurnTxCostIncrementNonce(
					txSenderAddress,
					models.TxType_SET_ACCOUNT_INFO_URI,
					0,
					tx.GetInfoURI(),
				); err != nil {
					return nil, fmt.Errorf("setAccountInfo: burnCostIncrementNonce %w", err)
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
					return nil, fmt.Errorf("addDelegate: txSenderAddress %w", err)
				}
				if err := t.state.BurnTxCostIncrementNonce(
					txSenderAddress,
					models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
					0,
					"",
				); err != nil {
					return nil, fmt.Errorf("addDelegate: burnTxCostIncrementNonce %w", err)
				}
				if err := t.state.SetAccountDelegate(
					txSenderAddress,
					tx.Delegates,
					models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
				); err != nil {
					return nil, fmt.Errorf("addDelegate: %w", err)
				}
			case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
				txSenderAddress, err := ethereum.AddrFromSignature(vtx.SignedBody, vtx.Signature)
				if err != nil {
					return nil, fmt.Errorf("delDelegate: txSenderAddress %w", err)
				}
				if err := t.state.BurnTxCostIncrementNonce(
					txSenderAddress,
					models.TxType_DEL_DELEGATE_FOR_ACCOUNT,
					0,
					"",
				); err != nil {
					return nil, fmt.Errorf("delDelegate: burnTxCostIncrementNonce %w", err)
				}
				if err := t.state.SetAccountDelegate(
					txSenderAddress,
					tx.Delegates,
					models.TxType_DEL_DELEGATE_FOR_ACCOUNT,
				); err != nil {
					return nil, fmt.Errorf("delDelegate: %w", err)
				}
			case models.TxType_SET_ACCOUNT_VALIDATOR:
				txSenderAddress, err := ethereum.AddrFromSignature(vtx.SignedBody, vtx.Signature)
				if err != nil {
					return nil, fmt.Errorf("setValidator: txSenderAddress %w", err)
				}
				validatorAddr, err := ethereum.AddrFromPublicKey(tx.GetPublicKey())
				if err != nil {
					return nil, fmt.Errorf("setValidator: %w", err)
				}
				if err := t.state.BurnTxCostIncrementNonce(
					txSenderAddress,
					models.TxType_SET_ACCOUNT_VALIDATOR,
					0,
					validatorAddr.Hex(),
				); err != nil {
					return nil, fmt.Errorf("setValidator: burnTxCostIncrementNonce %w", err)
				}
				if err := t.state.AddValidator(&models.Validator{
					Address:          validatorAddr.Bytes(),
					PubKey:           tx.GetPublicKey(),
					Name:             tx.GetName(),
					Power:            newValidatorPower,
					ValidatorAddress: cometCrypto256k1.PubKey(tx.GetPublicKey()).Address().Bytes(),
					Height:           uint64(t.state.CurrentHeight()),
				}); err != nil {
					return nil, fmt.Errorf("setValidator: %w", err)
				}
			default:
				return nil, fmt.Errorf("setAccount: invalid transaction type")
			}
		}

	case *models.Tx_SendTokens:
		err := t.SendTokensTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("sendTokensTx: %w", err)
		}
		if forCommit {
			tx := vtx.Tx.GetSendTokens()
			from, to := common.BytesToAddress(tx.From), common.BytesToAddress(tx.To)
			err := t.state.BurnTxCostIncrementNonce(from, models.TxType_SEND_TOKENS, 0, to.Hex())
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
			if err := t.state.BurnTxCostIncrementNonce(issuerAddress, models.TxType_COLLECT_FAUCET, 0, ""); err != nil {
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

	case *models.Tx_SetSIK:
		txAddress, newSIK, err := t.SetSIKTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("setSIKTx: %w", err)
		}
		if forCommit {
			if err := t.state.BurnTxCostIncrementNonce(
				txAddress,
				models.TxType_SET_ACCOUNT_SIK,
				0,
				newSIK.String(),
			); err != nil {
				return nil, fmt.Errorf("setSIKTx: burnTxCostIncrementNonce %w", err)
			}
			if err := t.state.SetAddressSIK(txAddress, newSIK); err != nil {
				return nil, fmt.Errorf("setSIKTx: %w", err)
			}
		}
		return response, nil

	case *models.Tx_DelSIK:
		txAddress, err := t.DelSIKTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("delSIKTx: %w", err)
		}
		if forCommit {
			if err := t.state.BurnTxCostIncrementNonce(
				txAddress,
				models.TxType_DEL_ACCOUNT_SIK,
				0,
				"",
			); err != nil {
				return nil, fmt.Errorf("delSIKTx: burnTxCostIncrementNonce %w", err)
			}
			if err := t.state.InvalidateSIK(txAddress); err != nil {
				return nil, fmt.Errorf("delSIKTx: %w", err)
			}
		}
		return response, nil

	case *models.Tx_RegisterSIK:
		txAddress, SIK, pid, tempSIKs, err := t.RegisterSIKTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("registerSIKTx: %w", err)
		}
		if forCommit {
			// register the SIK
			if err := t.state.SetAddressSIK(txAddress, SIK); err != nil {
				return nil, fmt.Errorf("registerSIKTx: %w", err)
			}
			// increase the RegisterSIKTx counter
			if err := t.state.IncreaseRegisterSIKCounter(pid); err != nil {
				return nil, fmt.Errorf("registerSIKTx: %w", err)
			}
			if tempSIKs {
				log.Infow("registering tempSIK", "address", txAddress.String())
				if err := t.state.AssignSIKToElection(pid, txAddress); err != nil {
					return nil, fmt.Errorf("registerSIKTx: %w", err)
				}
			}
		}
		return response, nil

	default:
		return nil, fmt.Errorf("invalid transaction type")
	}

	return response, nil
}

// checkAccountCanPayCost checks if the account can pay the cost of the transaction.
// It returns the account and the address of the sender.
func (t *TransactionHandler) checkAccountCanPayCost(txType models.TxType, vtx *vochaintx.Tx) (*vstate.Account, *common.Address, error) {
	// extract sender address from signature
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	txSenderAcc, err := t.state.GetAccount(txSenderAddress, false)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get account: %w", err)
	}
	if txSenderAcc == nil {
		return nil, nil, vstate.ErrAccountNotExist
	}
	// get setAccount tx cost
	cost, err := t.state.TxBaseCost(txType, false)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get tx cost for %s: %w", txType.String(), err)
	}
	// check tx sender balance
	if txSenderAcc.Balance < cost {
		return nil, nil, fmt.Errorf("unauthorized: %s", vstate.ErrNotEnoughBalance)
	}
	return txSenderAcc, &txSenderAddress, nil
}
