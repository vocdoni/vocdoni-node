package ethereum

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	sendTokensTemplate    = "Signing a Vocdoni transaction of type SEND_TOKENS for an amount of %d VOC tokens to destination address %x. The hash of the transaction is %x and the destination chainID is %s."
	voteTemplate          = "Signing a Vocdoni transaction of type VOTE for process ID %x. The hash of the transaction is %x and the destination chainID is %s."
	newProcessTemplate    = "Signing a Vocdoni transaction of type NEW_PROCESS. The hash of the transaction is %x and the destination chainID is %s."
	collectFaucetTemplate = "Signing a Vocdoni transaction of type COLLECT_FAUCET. The hash of the transaction is %x and the destination chainID is %s."
	setSIKTemplate        = "Signing a Vocdoni transaction of type SET_SIK for secret identity key %x. The hash of the transaction is %x and the destination chainID is %s."
	delSIKTemplate        = "Signing a Vocdoni transaction of type DEL_SIK for secret identity key %x. The hash of the transaction is %x and the destination chainID is %s."
	registerSIKTemplate   = "Signing a Vocdoni transaction of type REGISTER_SIK for secret identity key %x. The hash of the transaction is %x and the destination chainID is %s."

	setAccountDefaultTemplate  = "Signing a Vocdoni transaction of type SET_ACCOUNT/%s. The hash of the transaction is %x and the destination chainID is %s."
	createAccountTemplate      = "Signing a Vocdoni transaction of type CREATE_ACCOUNT for address %x. The hash of the transaction is %x and the destination chainID is %s."
	setAccountInfoURITemplate  = "Signing a Vocdoni transaction of type SET_ACCOUNT_INFO_URI for address %x with URI %s. The hash of the transaction is %x and the destination chainID is %s."
	addDelegateAccountTemplate = "Signing a Vocdoni transaction of type ADD_DELEGATE_FOR_ACCOUNT for address %x. The hash of the transaction is %x and the destination chainID is %s."
	delDelegateAccountTemplte  = "Signing a Vocdoni transaction of type DEL_DELEGATE_FOR_ACCOUNT for address %x. The hash of the transaction is %x and the destination chainID is %s."

	setProcessDefaultTemplate = "Signing a Vocdoni transaction of type SET_PROCESS/%s with process ID %x. The hash of the transaction is %x and the destination chainID is %s."
	setProcessCensusTemplate  = "Signing a Vocdoni transaction of type SET_PROCESS_CENSUS for process ID %x and census %x. The hash of the transaction is %x and the destination chainID is %s."
	setProcessStatusTemplate  = "Signing a Vocdoni transaction of type SET_PROCESS_STATUS for process ID %x and status %s. The hash of the transaction is %x and the destination chainID is %s."

	defaultTemplate = "Vocdoni signed transaction:\n%s\n%x"
)

// BuildVocdoniProtoTxMessage builds the message to be signed for a vocdoni transaction.
// It takes an optional transaction hash, if it is not provided it will be computed.
// It returns the payload that needs to be signed.
func BuildVocdoniProtoTxMessage(tx *models.Tx, chainID string, hash []byte) ([]byte, error) {
	var msg string

	marshaledTx, err := proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Tx: %v", err)
	}
	if hash == nil {
		hash = HashRaw(marshaledTx)
	}

	switch tx.Payload.(type) {
	case *models.Tx_SendTokens:
		t := tx.GetSendTokens()
		if t == nil {
			return nil, fmt.Errorf("send tokens payload is nil")
		}
		msg = fmt.Sprintf(sendTokensTemplate, t.Value, t.To, hash, chainID)
	case *models.Tx_SetProcess:
		t := tx.GetSetProcess()
		if t == nil {
			return nil, fmt.Errorf("set process payload is nil")
		}
		switch t.Txtype {
		case models.TxType_SET_PROCESS_CENSUS:
			msg = fmt.Sprintf(setProcessCensusTemplate, t.ProcessId, t.GetCensusRoot(), hash, chainID)
		case models.TxType_SET_PROCESS_STATUS:
			msg = fmt.Sprintf(setProcessStatusTemplate, t.ProcessId, strings.ToLower(t.GetStatus().String()), hash, chainID)
		default:
			msg = fmt.Sprintf(setProcessDefaultTemplate, t.Txtype.String(), t.ProcessId, hash, chainID)
		}
	case *models.Tx_Vote:
		t := tx.GetVote()
		if t == nil {
			return nil, fmt.Errorf("vote payload is nil")
		}
		msg = fmt.Sprintf(voteTemplate, t.ProcessId, hash, chainID)
	case *models.Tx_NewProcess:
		t := tx.GetNewProcess().GetProcess()
		if t == nil {
			return nil, fmt.Errorf("new process payload is nil")
		}
		msg = fmt.Sprintf(newProcessTemplate, hash, chainID)
	case *models.Tx_SetAccount:
		t := tx.GetSetAccount()
		if t == nil {
			return nil, fmt.Errorf("set account payload is nil")
		}
		switch t.Txtype {
		case models.TxType_CREATE_ACCOUNT:
			msg = fmt.Sprintf(createAccountTemplate, t.Account, hash, chainID)
		case models.TxType_SET_ACCOUNT_INFO_URI:
			msg = fmt.Sprintf(setAccountInfoURITemplate, t.Account, t.GetInfoURI(), hash, chainID)
		case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
			msg = fmt.Sprintf(addDelegateAccountTemplate, t.Account, hash, chainID)
		case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
			msg = fmt.Sprintf(delDelegateAccountTemplte, t.Account, hash, chainID)
		default:
			msg = fmt.Sprintf(setAccountDefaultTemplate, t.Txtype.String(), hash, chainID)
		}
	case *models.Tx_CollectFaucet:
		t := tx.GetCollectFaucet()
		if t == nil {
			return nil, fmt.Errorf("collect faucet payload is nil")
		}
		msg = fmt.Sprintf(collectFaucetTemplate, hash, chainID)
	case *models.Tx_SetSIK:
		t := tx.GetSetSIK()
		if t == nil {
			return nil, fmt.Errorf("set SIK payload is nil")
		}
		msg = fmt.Sprintf(setSIKTemplate, t.SIK, hash, chainID)
	case *models.Tx_DelSIK:
		t := tx.GetSetSIK()
		if t == nil {
			return nil, fmt.Errorf("del SIK payload is nil")
		}
		msg = fmt.Sprintf(delSIKTemplate, t.SIK, hash, chainID)
	case *models.Tx_RegisterSIK:
		t := tx.GetRegisterSIK()
		if t == nil {
			return nil, fmt.Errorf("register SIK payload is nil")
		}
		msg = fmt.Sprintf(registerSIKTemplate, t.SIK, hash, chainID)
	default:
		msg = fmt.Sprintf(defaultTemplate, chainID, hash)
	}
	return []byte(msg), nil
}

// SignVocdoniTx signs a vocdoni transaction. TxData is the full transaction payload (no HexString nor a Hash)
func (k *SignKeys) SignVocdoniTx(txData []byte, chainID string) ([]byte, error) {
	if k.Private.D == nil {
		return nil, errors.New("no private key available")
	}
	// TODO: this unmarshal is not necessary, but it is required to keep compatibility with the old code
	// the SIgnVocdoniTx method should take directly a models.Tx as parameter and return the signature
	// and marhaled tx.
	tx := &models.Tx{}
	if err := proto.Unmarshal(txData, tx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Tx: %v", err)
	}

	payloadToSign, err := BuildVocdoniProtoTxMessage(tx, chainID, HashRaw(txData))
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction message: %v", err)
	}

	signature, err := ethcrypto.Sign(Hash(payloadToSign), &k.Private)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

// SignVocdoniMsg signs a vocdoni message. Message is the full payload (no HexString nor a Hash)
func (k *SignKeys) SignVocdoniMsg(message []byte) ([]byte, error) {
	if k.Private.D == nil {
		return nil, errors.New("no private key available")
	}
	signature, err := ethcrypto.Sign(Hash(BuildVocdoniMessage(message)), &k.Private)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

// BuildVocdoniTransaction builds the payload for a Vocdoni transaction.
// It returns the payload that needs to be signed and the unmarshaled transaction struct.
func BuildVocdoniTransaction(marshaledTx []byte, chainID string) ([]byte, *models.Tx, error) {
	var tx models.Tx
	if err := proto.Unmarshal(marshaledTx, &tx); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal Tx: %v", err)
	}
	message, err := BuildVocdoniProtoTxMessage(&tx, chainID, HashRaw(marshaledTx))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build transaction message: %v", err)
	}
	return message, &tx, nil
}

// BuildVocdoniMessage builds the payload of a vocdoni message
// ready to be signed
func BuildVocdoniMessage(message []byte) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Vocdoni signed message:\n%x", HashRaw(message))
	return buf.Bytes()
}
