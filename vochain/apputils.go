package vochain

import (
	"encoding/json"
	"fmt"
	"reflect"

	signature "gitlab.com/vocdoni/go-dvote/crypto/signature"
	tree "gitlab.com/vocdoni/go-dvote/tree"
	vochaintypes "gitlab.com/vocdoni/go-dvote/types"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

// ValidateTx splits a tx into method and args parts and does some basic checks
func ValidateTx(content []byte, state *VochainState) (interface{}, string, error) {

	var txType vochaintypes.Tx
	var err error

	err = json.Unmarshal(content, &txType)
	if err != nil || len(txType.Type) < 1 {
		return nil, "", fmt.Errorf("cannot extract type (%s)", err.Error())
	}

	structType := vochaintypes.ValidateType(txType.Type)

	switch structType {
	case "VoteTx":
		var vote *vochaintypes.VoteTx
		err = json.Unmarshal(content, &vote)
		if err != nil {
			return nil, "", fmt.Errorf("cannot parse VoteTX")
		}
		return vote, "VoteTx", VoteTxCheck(vote, state)

	case "AdminTx":
		var adminTx *vochaintypes.AdminTx
		err = json.Unmarshal(content, &adminTx)
		if err != nil {
			return nil, "", fmt.Errorf("cannot parse AdminTx")
		}
		return adminTx, "AdminTx", AdminTxCheck(adminTx, state)
	case "NewProcessTx":
		var process *vochaintypes.NewProcessTx
		err = json.Unmarshal(content, &process)
		if err != nil {
			return nil, "", fmt.Errorf("cannot parse NewProcessTx")
		}
		return process, "NewProcessTx", NewProcessTxCheck(process, state)
	}

	return nil, "", fmt.Errorf("invalid type")
}

// ValidateAndDeliverTx validates a tx and executes the methods required for changing the app state
func ValidateAndDeliverTx(content []byte, state *VochainState) error {
	tx, txType, err := ValidateTx(content, state)
	if err != nil {
		return fmt.Errorf("transaction validation failed with error (%s)", err.Error())
	}
	switch txType {
	case "VoteTx":
		votetx := reflect.Indirect(reflect.ValueOf(tx)).Interface().(vochaintypes.VoteTx)
		vote := &vochaintypes.Vote{
			Nullifier:   votetx.Nullifier,
			Nonce:       votetx.Nonce,
			ProcessID:   votetx.ProcessID,
			VotePackage: votetx.VotePackage,
			Signature:   votetx.Signature,
			Proof:       votetx.Proof,
		}
		return state.AddVote(vote)
	case "AdminTx":
		adminTx := reflect.Indirect(reflect.ValueOf(tx)).Interface().(vochaintypes.AdminTx)
		switch adminTx.Type {
		case "addOracle":
			return state.AddOracle(adminTx.Address)
		case "removeOracle":
			return state.RemoveOracle(adminTx.Address)
		case "addValidator":
			return state.AddValidator(adminTx.Address, adminTx.Power)
		case "removeValidator":
			return state.RemoveValidator(adminTx.Address)
		}
	case "NewProcessTx":
		processtx := reflect.Indirect(reflect.ValueOf(tx)).Interface().(vochaintypes.NewProcessTx)
		newprocess := &vochaintypes.Process{
			EntityID:              processtx.EntityID,
			EncryptionPrivateKeys: []string{},
			EncryptionPublicKeys:  processtx.EncryptionPublicKeys,
			MkRoot:                processtx.MkRoot,
			NumberOfBlocks:        processtx.NumberOfBlocks,
			StartBlock:            processtx.StartBlock,
			CurrentState:          vochaintypes.Scheduled,
		}
		return state.AddProcess(newprocess, processtx.ProcessID)
	}
	return fmt.Errorf("invalid type")

}

// VoteTxCheck is an abstraction of ABCI checkTx for submitting a vote
func VoteTxCheck(vote *vochaintypes.VoteTx, state *VochainState) error {
	process, _ := state.GetProcess(vote.ProcessID)
	if process == nil {
		return fmt.Errorf("process with id (%s) does not exists", vote.ProcessID)
	}
	voteID := fmt.Sprintf("%s%s", vote.ProcessID, vote.Nullifier)
	v, _ := state.GetVote(voteID)
	if v != nil {
		return fmt.Errorf("vote already exists")
	}
	sign := vote.Signature
	vote.Signature = ""
	voteBytes, err := json.Marshal(vote)
	if err != nil {
		return fmt.Errorf("cannot marshal vote (%s)", err.Error())
	}
	pubKey, err := signature.PubKeyFromSignature(string(voteBytes), sign)
	if err != nil {
		return fmt.Errorf("cannot extract public key from signature (%s)", err.Error())
	}
	pubKeyHash := signature.HashPoseidon(pubKey)
	if len(pubKeyHash) > 32 {
		return fmt.Errorf("wrong Poseidon hash size (%s)", err.Error())
	}
	valid, err := checkMerkleProof(process.MkRoot, vote.Proof, pubKeyHash)
	if err != nil {
		return fmt.Errorf("cannot check merkle proof (%s)", err.Error())
	}
	if !valid {
		return fmt.Errorf("proof not valid")
	}
	return nil
}

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func NewProcessTxCheck(process *vochaintypes.NewProcessTx, state *VochainState) error {
	// get oracles
	oracles, err := state.GetOracles()
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}
	sign := process.Signature
	process.Signature = ""
	processBytes, err := json.Marshal(process)
	if err != nil {
		return fmt.Errorf("cannot marshal process (%s)", err.Error())
	}
	authorized, addr := VerifySignatureAgainstOracles(oracles, string(processBytes), sign)
	if !authorized {
		return fmt.Errorf("unauthorized to create a process, message: %s, recovered addr: %s", string(processBytes), addr)
	}
	// get process
	_, err = state.GetProcess(process.ProcessID)
	if err == nil {
		return fmt.Errorf("process with id (%s) already exists", process.ProcessID)
	}
	return nil
}

// AdminTxCheck is an abstraction of ABCI checkTx for an admin transaction
func AdminTxCheck(adminTx *vochaintypes.AdminTx, state *VochainState) error {
	// get oracles
	oracles, err := state.GetOracles()
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}
	sign := adminTx.Signature
	adminTx.Signature = ""
	adminTxBytes, err := json.Marshal(adminTx)
	if err != nil {
		return fmt.Errorf("cannot marshal adminTx (%s)", err.Error())
	}
	authorized, addr := VerifySignatureAgainstOracles(oracles, string(adminTxBytes), sign)
	if !authorized {
		return fmt.Errorf("unauthorized to perform an adminTx, address: %s", addr)
	}
	return nil
}

// hexproof is the hexadecimal a string. leafData is the claim data in byte format
func checkMerkleProof(rootHash, hexproof string, leafData []byte) (bool, error) {
	return tree.CheckProof(rootHash, hexproof, leafData)
}

// VerifySignatureAgainstOracles verifies that a signature match with one of the oracles
func VerifySignatureAgainstOracles(oracles []string, message, signHex string) (bool, string) {
	oraclesAddr := make([]signature.Address, len(oracles))
	for i, v := range oracles {
		oraclesAddr[i] = signature.AddressFromString(v)
	}
	signKeys := signature.SignKeys{
		Authorized: oraclesAddr,
	}
	res, addr, _ := signKeys.VerifySender(message, signHex)
	return res, addr
}

// GenerateNullifier generates the nullifier of a vote (hash(address+processId))
func GenerateNullifier(address, processID string) []byte {
	return signature.HashRaw(fmt.Sprintf("%s%s", address, processID))
}

// GenerateAddressFromEd25519PublicKeyString returns the address as string from given pubkey represented as string
func GenerateAddressFromEd25519PublicKeyString(publicKey string) string {
	return crypto.Address(tmhash.SumTruncated([]byte(publicKey))).String()
}
