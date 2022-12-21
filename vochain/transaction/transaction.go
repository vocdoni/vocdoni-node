package transaction

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	snarkTypes "github.com/vocdoni/go-snark/types"
	"go.vocdoni.io/dvote/crypto/ethereum"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	models "go.vocdoni.io/proto/build/go/models"
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
	// dataDir is the path for storing some files
	dataDir string
	// ZkVKs contains the VerificationKey for each circuit parameters index
	ZkVKs []*snarkTypes.Vk

	NewZkVKs [][]byte
}

// NewTransactionHandler creates a new TransactionHandler.
func NewTransactionHandler(state *vstate.State, dataDir string) (*TransactionHandler, error) {
	return &TransactionHandler{
		state:   state,
		dataDir: dataDir,
	}, nil
}

// LoadZkVerificationKeys loads or downloads the zk Circuits VerificationKey files.
// It is required for veirying zkSnark proofs.
func (t *TransactionHandler) LoadZkVerificationKeys() error {
	// get the zk Circuits VerificationKey files
	vks, err := LoadZkVerificationKeys(t.dataDir, t.state.ChainID())
	if err != nil {
		return fmt.Errorf("could not load zk verification keys: %w", err)
	}
	t.ZkVKs = vks
	return nil
}

// CheckTx check the validity of a transaction and adds it to the state if forCommit=true.
// It returns a bytes value which depends on the transaction type:
//
//	Tx_Vote: vote nullifier
//	default: []byte{}
func (t *TransactionHandler) CheckTx(vtx *vochaintx.VochainTx, forCommit bool) (*TransactionResponse, error) {
	if vtx.Tx == nil || vtx.Tx.Payload == nil {
		return nil, fmt.Errorf("transaction is empty")
	}
	response := &TransactionResponse{
		TxHash: vtx.TxID[:],
	}
	switch vtx.Tx.Payload.(type) {
	case *models.Tx_Vote:
		v, voterID, err := t.VoteTxCheck(vtx, forCommit)
		if err != nil || v == nil {
			return nil, fmt.Errorf("voteTx: %w", err)
		}
		response.Data = v.Nullifier
		if forCommit {
			return response, t.state.AddVote(v, voterID)
		}

	case *models.Tx_Admin:
		_, err := t.AdminTxCheck(vtx)
		if err != nil {
			return nil, fmt.Errorf("adminTx: %w", err)
		}
		if forCommit {
			tx := vtx.Tx.GetAdmin()
			switch tx.Txtype {
			case models.TxType_ADD_ORACLE:
				if err := t.state.AddOracle(common.BytesToAddress(tx.Address)); err != nil {
					return nil, fmt.Errorf("addOracle: %w", err)
				}
				return response, t.state.IncrementTreasurerNonce()
			case models.TxType_REMOVE_ORACLE:
				if err := t.state.RemoveOracle(common.BytesToAddress(tx.Address)); err != nil {
					return nil, fmt.Errorf("removeOracle: %w", err)
				}
				return response, t.state.IncrementTreasurerNonce()
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
		p, txSender, err := t.NewProcessTxCheck(vtx, forCommit)
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
			return response, t.state.BurnTxCostIncrementNonce(txSender, models.TxType_NEW_PROCESS)
		}

	case *models.Tx_SetProcess:
		txSender, err := t.SetProcessTxCheck(vtx, forCommit)
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
			case models.TxType_SET_PROCESS_RESULTS:
				if tx.GetResults() == nil {
					return nil, fmt.Errorf("set process results, results is nil")
				}
				if err := t.state.SetProcessResults(tx.ProcessId, tx.Results, true); err != nil {
					return nil, fmt.Errorf("setProcessResults: %s", err)
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
			return response, t.state.BurnTxCostIncrementNonce(txSender, tx.Txtype)
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
		switch tx.Txtype {
		case models.TxType_CREATE_ACCOUNT:
			err := t.CreateAccountTxCheck(vtx)
			if err != nil {
				return nil, fmt.Errorf("createAccountTx: %w", err)
			}

		case models.TxType_SET_ACCOUNT_INFO_URI:
			err := t.SetAccountInfoTxCheck(vtx)
			if err != nil {
				return nil, fmt.Errorf("setAccountInfoTx: %w", err)
			}

		case models.TxType_ADD_DELEGATE_FOR_ACCOUNT, models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
			err := t.SetAccountDelegateTxCheck(vtx)
			if err != nil {
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
				if tx.FaucetPackage != nil {
					faucetIssuerAddress, err := ethereum.AddrFromSignature(tx.FaucetPackage.Payload, tx.FaucetPackage.Signature)
					if err != nil {
						return nil, fmt.Errorf("createAccountTx: faucetIssuerAddress %w", err)
					}
					txCost, err := t.state.TxCost(models.TxType_CREATE_ACCOUNT, false)
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
					// transfer balance from faucet package issuer to created account
					return response, t.state.TransferBalance(
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
				if err := t.state.BurnTxCostIncrementNonce(
					txSenderAddress,
					models.TxType_SET_ACCOUNT_INFO_URI,
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
			if err := t.state.SetTxCost(vtx.Tx.GetSetTransactionCosts().Txtype, cost); err != nil {
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
			if err := t.state.MintBalance(common.BytesToAddress(tx.To), tx.Value); err != nil {
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
			err := t.state.BurnTxCostIncrementNonce(from, models.TxType_SEND_TOKENS)
			if err != nil {
				return nil, fmt.Errorf("sendTokensTx: burnTxCostIncrementNonce %w", err)
			}
			return response, t.state.TransferBalance(from, to, tx.Value)
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
			if err := t.state.BurnTxCostIncrementNonce(issuerAddress, models.TxType_COLLECT_FAUCET); err != nil {
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
			return response, t.state.TransferBalance(issuerAddress,
				common.BytesToAddress(faucetPayload.To),
				faucetPayload.Amount,
			)
		}

	default:
		return nil, fmt.Errorf("invalid transaction type")
	}

	return response, nil
}
