package vochain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/tree"
	vochaintypes "gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

// ValidateTx splits a tx into method and args parts and does some basic checks
func ValidateTx(content []byte, state *VochainState) (interface{}, error) {
	var txType vochaintypes.Tx
	err := json.Unmarshal(content, &txType)
	if err != nil || len(txType.Type) < 1 {
		return nil, fmt.Errorf("cannot extract type (%s)", err)
	}

	structType := vochaintypes.ValidateType(txType.Type)

	switch structType {
	case "VoteTx":
		var voteTx vochaintypes.VoteTx
		if err := json.Unmarshal(content, &voteTx); err != nil {
			return nil, fmt.Errorf("cannot parse VoteTX")
		}
		return voteTx, VoteTxCheck(voteTx, state)

	case "AdminTx":
		var adminTx vochaintypes.AdminTx
		if err := json.Unmarshal(content, &adminTx); err != nil {
			return nil, fmt.Errorf("cannot parse AdminTx")
		}
		return adminTx, AdminTxCheck(adminTx, state)
	case "NewProcessTx":
		var processTx vochaintypes.NewProcessTx
		if err := json.Unmarshal(content, &processTx); err != nil {
			return nil, fmt.Errorf("cannot parse NewProcessTx")
		}
		return processTx, NewProcessTxCheck(processTx, state)
	}

	return nil, fmt.Errorf("invalid type")
}

// ValidateAndDeliverTx validates a tx and executes the methods required for changing the app state
func ValidateAndDeliverTx(content []byte, state *VochainState) error {
	tx, err := ValidateTx(content, state)
	if err != nil {
		return fmt.Errorf("transaction validation failed with error (%s)", err)
	}
	switch tx := tx.(type) {
	case vochaintypes.VoteTx:
		process, _ := state.Process(tx.ProcessID)
		if process == nil {
			return fmt.Errorf("process with id (%s) does not exists", tx.ProcessID)
		}
		vote := new(vochaintypes.Vote)
		switch process.Type {
		case "snark-vote":
			vote.Nullifier = util.TrimHex(tx.Nullifier)
			vote.Nonce = util.TrimHex(tx.Nonce)
			vote.ProcessID = util.TrimHex(tx.ProcessID)
			vote.VotePackage = util.TrimHex(tx.VotePackage)
			vote.Proof = util.TrimHex(tx.Proof)
		case "poll-vote", "petition-sign":
			vote.Nonce = tx.Nonce
			vote.ProcessID = tx.ProcessID
			vote.Proof = tx.Proof
			vote.VotePackage = tx.VotePackage

			voteBytes, err := json.Marshal(vote)
			if err != nil {
				return fmt.Errorf("cannot marshal vote (%s)", err)
			}
			pubKey, err := signature.PubKeyFromSignature(string(voteBytes), tx.Signature)
			if err != nil {
				// log.Warnf("cannot extract pubKey: %s", err)
				return fmt.Errorf("cannot extract public key from signature (%s)", err)
			}
			addr, err := signature.AddrFromPublicKey(string(pubKey))
			if err != nil {
				return fmt.Errorf("cannot extract address from public key")
			}
			vote.Nonce = util.TrimHex(tx.Nonce)
			vote.VotePackage = util.TrimHex(tx.VotePackage)
			vote.Signature = util.TrimHex(tx.Signature)
			vote.Proof = util.TrimHex(tx.Proof)
			vote.ProcessID = util.TrimHex(tx.ProcessID)
			nullifier, err := GenerateNullifier(addr, vote.ProcessID)
			if err != nil {
				return fmt.Errorf("cannot generate nullifier")
			}
			vote.Nullifier = nullifier

		default:
			return fmt.Errorf("invalid process type")
		}
		// log.Debugf("adding vote: %+v", vote)
		return state.AddVote(vote)
	case vochaintypes.AdminTx:
		switch tx.Type {
		case "addOracle":
			return state.AddOracle(tx.Address)
		case "removeOracle":
			return state.RemoveOracle(tx.Address)
		case "addValidator":
			return state.AddValidator(tx.Address, tx.Power)
		case "removeValidator":
			return state.RemoveValidator(tx.Address)
		}
	case vochaintypes.NewProcessTx:
		newprocess := &vochaintypes.Process{
			EntityID:             util.TrimHex(tx.EntityID),
			EncryptionPublicKeys: tx.EncryptionPublicKeys,
			MkRoot:               util.TrimHex(tx.MkRoot),
			NumberOfBlocks:       tx.NumberOfBlocks,
			StartBlock:           tx.StartBlock,
			CurrentState:         vochaintypes.Scheduled,
			Type:                 tx.ProcessType,
		}
		return state.AddProcess(newprocess, tx.ProcessID)
	}
	return fmt.Errorf("invalid type")
}

// VoteTxCheck is an abstraction of ABCI checkTx for submitting a vote
func VoteTxCheck(vote vochaintypes.VoteTx, state *VochainState) error {
	process, _ := state.Process(vote.ProcessID)
	if process == nil {
		return fmt.Errorf("process with id (%s) does not exists", vote.ProcessID)
	}

	switch process.Type {
	case "snark-vote":
		voteID := fmt.Sprintf("%s_%s", util.TrimHex(vote.ProcessID), util.TrimHex(vote.Nullifier))
		v, _ := state.Envelope(voteID)
		if v != nil {
			log.Debugf("vote already exists")
			return fmt.Errorf("vote already exists")
		}
		// TODO check snark

	case "poll-vote", "petition-sign":
		var voteTmp vochaintypes.VoteTx
		voteTmp.Nonce = vote.Nonce
		voteTmp.ProcessID = vote.ProcessID
		voteTmp.Proof = vote.Proof
		voteTmp.VotePackage = vote.VotePackage

		voteBytes, err := json.Marshal(voteTmp)
		if err != nil {
			return fmt.Errorf("cannot marshal vote (%s)", err)
		}
		// log.Debugf("executing VoteTxCheck of: %s", voteBytes)
		pubKey, err := signature.PubKeyFromSignature(string(voteBytes), vote.Signature)
		if err != nil {
			return fmt.Errorf("cannot extract public key from signature (%s)", err)
		}

		addr, err := signature.AddrFromPublicKey(string(pubKey))
		if err != nil {
			return fmt.Errorf("cannot extract address from public key")
		}
		// assign a nullifier
		nullifier, err := GenerateNullifier(addr, vote.ProcessID)
		if err != nil {
			return fmt.Errorf("cannot generate nullifier")
		}
		voteTmp.Nullifier = nullifier
		log.Debugf("generated nullifier: %s", voteTmp.Nullifier)
		// check if vote exists
		voteID := fmt.Sprintf("%s_%s", util.TrimHex(vote.ProcessID), util.TrimHex(voteTmp.Nullifier))
		v, _ := state.Envelope(voteID)
		if v != nil {
			return fmt.Errorf("vote already exists")
		}

		// check merkle proof
		log.Debugf("extracted pubkey: %s", pubKey)
		pubKeyHash := signature.HashPoseidon(pubKey)
		if len(pubKeyHash) > 32 || len(pubKeyHash) == 0 { // TO-DO check the exact size of PoseidonHash
			return fmt.Errorf("wrong Poseidon hash size (%s)", err)
		}
		valid, err := checkMerkleProof(process.MkRoot, vote.Proof, pubKeyHash)
		if err != nil {
			return fmt.Errorf("cannot check merkle proof (%s)", err)
		}
		if !valid {
			return fmt.Errorf("proof not valid")
		}

	default:
		return fmt.Errorf("invalid process type")
	}
	return nil
}

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func NewProcessTxCheck(process vochaintypes.NewProcessTx, state *VochainState) error {
	// get oracles
	oracles, err := state.Oracles()
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}
	sign := process.Signature
	process.Signature = ""

	processBytes, err := json.Marshal(process)
	if err != nil {
		return fmt.Errorf("cannot marshal process (%s)", err)
	}
	authorized, addr := VerifySignatureAgainstOracles(oracles, string(processBytes), sign)
	if !authorized {
		return fmt.Errorf("unauthorized to create a process, message: %s, recovered addr: %s", string(processBytes), addr)
	}
	// get process
	_, err = state.Process(process.ProcessID)
	if err == nil {
		return fmt.Errorf("process with id (%s) already exists", process.ProcessID)
	}
	// check type
	switch process.ProcessType {
	case "snark-vote", "poll-vote", "petition-sign":
		// ok
	default:
		return fmt.Errorf("process type (%s) not valid", process.ProcessType)
	}
	return nil
}

// AdminTxCheck is an abstraction of ABCI checkTx for an admin transaction
func AdminTxCheck(adminTx vochaintypes.AdminTx, state *VochainState) error {
	// get oracles
	oracles, err := state.Oracles()
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}
	sign := adminTx.Signature
	adminTx.Signature = ""
	adminTxBytes, err := json.Marshal(adminTx)
	if err != nil {
		return fmt.Errorf("cannot marshal adminTx (%s)", err)
	}
	authorized, addr := VerifySignatureAgainstOracles(oracles, string(adminTxBytes), sign)
	if !authorized {
		return fmt.Errorf("unauthorized to perform an adminTx, address: %s, message: %s", addr, string(adminTxBytes))
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
		oraclesAddr[i] = signature.AddressFromString(fmt.Sprintf("0x%s", v))
	}
	signKeys := signature.SignKeys{
		Authorized: oraclesAddr,
	}
	res, addr, _ := signKeys.VerifySender(message, signHex)
	return res, addr
}

// GenerateNullifier generates the nullifier of a vote (hash(address+processId))
func GenerateNullifier(address, processID string) (string, error) {
	var err error
	addrBytes, err := hex.DecodeString(util.TrimHex(address))
	if err != nil {
		return "", err
	}
	pidBytes, err := hex.DecodeString(util.TrimHex(processID))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", signature.HashRaw(fmt.Sprintf("%s%s", addrBytes, pidBytes))), nil
}

// GenerateAddressFromEd25519PublicKeyString returns the address as string from given pubkey represented as string
func GenerateAddressFromEd25519PublicKeyString(publicKey string) string {
	return crypto.Address(tmhash.SumTruncated([]byte(publicKey))).String()
}
