package vochain

import (
	"encoding/json"
	"fmt"

	dbm "github.com/tendermint/tm-db"
	signature "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	tree "gitlab.com/vocdoni/go-dvote/tree"
	voctypes "gitlab.com/vocdoni/go-dvote/vochain/types"
)

// ValidateTx splits a tx into method and args parts and does some basic checks
func ValidateTx(content []byte, appDb *dbm.GoLevelDB) (voctypes.ValidTx, error) {

	var t voctypes.Tx
	var vt voctypes.ValidTx
	var err error

	err = json.Unmarshal(content, &t)
	if err != nil {
		log.Debugf("Error in unmarshall: %s", err)
	}
	// unmarshal bytes
	if err != nil {
		return vt, err
	}
	//log.Debugf("Unmarshaled content: %v", t)
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
		voteArgs := args.(*voctypes.VoteTxArgs)
		if voteArgs == nil {
			return vt, fmt.Errorf("cannot parse VoteTX")
		}
		vt.Args = voteArgs
		return vt, voteTxCheck(voteArgs, appDb)
	case voctypes.AddOracleTx:
		vt.Args = args.(*voctypes.AddOracleTxArgs)
		break
	case voctypes.RemoveOracleTx:
		vt.Args = args.(*voctypes.RemoveOracleTxArgs)
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

	// validate signature TBD

	return vt, nil
}

// VerifySignatureAgainstOracles verifies that a signature match with one of the oracles
func VerifySignatureAgainstOracles(oracles []signature.Address, message, signHex string) bool {

	signKeys := signature.SignKeys{
		Authorized: oracles,
	}
	res, _, err := signKeys.VerifySender(message, signHex)

	if err != nil {
		return false
	}

	return res
}

func voteTxCheck(voteArgs *voctypes.VoteTxArgs, appDb *dbm.GoLevelDB) error {
	var processInfo voctypes.Process
	err := json.Unmarshal(appDb.Get([]byte(voteArgs.ProcessID)), &processInfo)
	if err != nil {
		return fmt.Errorf("cannot get process Info on VoteTx (%s)", err.Error())
	}
	var voteArgsSign voctypes.VoteTxArgsSigned
	voteArgsSign.Nonce = voteArgs.Nonce
	voteArgsSign.ProcessID = voteArgs.ProcessID
	voteArgsSign.Proof = voteArgs.Proof
	voteArgsSign.VotePackage = voteArgs.VotePackage
	jsonVoteArgsSign, err := json.Marshal(voteArgsSign)
	if err != nil {
		return fmt.Errorf("cannot marshal voteArgsSign (%s)", err.Error())
	}
	pubKey, err := signature.PubKeyFromSignature(string(jsonVoteArgsSign), voteArgs.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from signature (%s)", err.Error())
	}
	pubKeyHash := signature.HashPoseidon(pubKey)
	if len(pubKeyHash) < 31 {
		return fmt.Errorf("wrong Poseidon hash (%s)", err.Error())
	}
	valid, err := checkMerkleProof(processInfo.MkRoot, voteArgs.Proof, pubKeyHash)
	if err != nil {
		return fmt.Errorf("cannot check merkle proof (%s)", err.Error())
	}
	if !valid {
		return fmt.Errorf("proof not valid")
	}
	return nil
}

func checkMerkleProof(rootHash, proof string, leafData []byte) (bool, error) {
	return tree.CheckProof(rootHash, proof, leafData)
}
