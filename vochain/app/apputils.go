package vochain

import (
	"errors"

	codec "github.com/cosmos/cosmos-sdk/codec"
	eth "gitlab.com/vocdoni/go-dvote/crypto/signature"
	voctypes "gitlab.com/vocdoni/go-dvote/vochain/types"
)

// ValidateTx splits a tx into method and args parts and does some basic checks
func ValidateTx(content []byte) (voctypes.ValidTx, error) {
	var t voctypes.Tx
	var vt voctypes.ValidTx
	var err error

	err = codec.Cdc.UnmarshalJSON(content, &t)
	// unmarshal bytes
	if err != nil {
		return vt, err
	}

	// validate method name
	m := t.ValidateMethod()
	if m == voctypes.InvalidTx {
		return vt, err
	}
	vt.Method = m

	// validate method args
	args, err := t.ValidateArgs()
	if err != nil {
		return vt, err
	}

	// create specific args struct depending on tx method
	switch m {
	case voctypes.NewProcessTx:
		vt.Args = args.(*voctypes.NewProcessTxArgs)
	case voctypes.VoteTx:
		vt.Args = args.(*voctypes.VoteTxArgs)
	case voctypes.AddTrustedOracleTx:
		vt.Args = args.(*voctypes.AddTrustedOracleTxArgs)
	case voctypes.RemoveTrustedOracleTx:
		vt.Args = args.(*voctypes.RemoveTrustedOracleTxArgs)
	case voctypes.AddValidatorTx:
		vt.Args = args.(*voctypes.AddValidatorTxArgs)
	case voctypes.RemoveValidatorTx:
		vt.Args = args.(*voctypes.RemoveValidatorTxArgs)
	case voctypes.InvalidTx:
		vt.Args = nil
	}

	// voteTx does not require signature
	if vt.Method == voctypes.VoteTx {
		return vt, nil
	}

	// validate signature
	dataToSign := vt.Args.String()
	if verifySignature(dataToSign, t.Signature) {
		return vt, nil
	}

	return vt, errors.New("Invalid signature")
}

// verifies a signature given a message and the signature
func verifySignature(message, signature string) bool {
	sigPubKey, err := eth.PubKeyFromSignature(message, signature)
	if err != nil {
		return false
	}
	ok, err := eth.Verify(message, signature, sigPubKey)
	return ok
}

// VerifySignatureAgainstOracles verifies that a signature match with one of the oracles
func VerifySignatureAgainstOracles(oracles []eth.Address, message, signature string) bool {
	sigPubKey, err := eth.PubKeyFromSignature(message, signature)
	if err != nil {
		return false
	}
	_, err = eth.Verify(message, signature, sigPubKey)
	if err != nil {
		return false
	}
	sigAddr, err := eth.AddrFromPublicKey(sigPubKey)
	if err != nil {
		return false
	}
	for _, o := range oracles {
		if sigAddr == o.String() {
			if ok, _ := eth.Verify(message, signature, o.String()); ok {
				return true
			}
		}
	}
	return false
}
