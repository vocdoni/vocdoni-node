package vochain

import (
	"encoding/json"
	"fmt"
	"reflect"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/tree"
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
		return nil, "", fmt.Errorf("cannot extract type (%s)", err)
	}

	structType := vochaintypes.ValidateType(txType.Type)

	switch structType {
	case "VoteTx":
		var voteTx vochaintypes.VoteTx
		err = json.Unmarshal(content, &voteTx)
		if err != nil {
			return nil, "", fmt.Errorf("cannot parse VoteTX")
		}
		return voteTx, "VoteTx", VoteTxCheck(voteTx, state)

	case "AdminTx":
		var adminTx vochaintypes.AdminTx
		err = json.Unmarshal(content, &adminTx)
		if err != nil {
			return nil, "", fmt.Errorf("cannot parse AdminTx")
		}
		return adminTx, "AdminTx", AdminTxCheck(adminTx, state)
	case "NewProcessTx":
		var processTx vochaintypes.NewProcessTx
		err = json.Unmarshal(content, &processTx)
		if err != nil {
			return nil, "", fmt.Errorf("cannot parse NewProcessTx")
		}
		return processTx, "NewProcessTx", NewProcessTxCheck(processTx, state)
	}

	return nil, "", fmt.Errorf("invalid type")
}

// ValidateAndDeliverTx validates a tx and executes the methods required for changing the app state
func ValidateAndDeliverTx(content []byte, state *VochainState) error {
	tx, txType, err := ValidateTx(content, state)
	if err != nil {
		return fmt.Errorf("transaction validation failed with error (%s)", err)
	}
	switch txType {
	case "VoteTx":
		votetx := reflect.Indirect(reflect.ValueOf(tx)).Interface().(vochaintypes.VoteTx)
		process, _ := state.GetProcess(votetx.ProcessID)
		if process == nil {
			return fmt.Errorf("process with id (%s) does not exists", votetx.ProcessID)
		}
		vote := new(vochaintypes.Vote)
		if process.Type == "snark-vote" {
			vote.Nullifier = sanitizeHex(votetx.Nullifier)
			vote.Nonce = sanitizeHex(votetx.Nonce)
			vote.ProcessID = sanitizeHex(votetx.ProcessID)
			vote.VotePackage = sanitizeHex(votetx.VotePackage)
			vote.Proof = sanitizeHex(votetx.Proof)
		} else if process.Type == "poll-vote" || process.Type == "petition-sign" {
			vote.Nonce = votetx.Nonce
			vote.ProcessID = votetx.ProcessID
			vote.Proof = votetx.Proof
			vote.VotePackage = votetx.VotePackage

			voteBytes, err := json.Marshal(vote)
			if err != nil {
				return fmt.Errorf("cannot marshal vote (%s)", err)
			}
			pubKey, err := signature.PubKeyFromSignature(string(voteBytes), votetx.Signature)
			if err != nil {
				//log.Warnf("cannot extract pubKey: %s", err)
				return fmt.Errorf("cannot extract public key from signature (%s)", err)
			}
			addr, err := signature.AddrFromPublicKey(string(pubKey))
			if err != nil {
				return fmt.Errorf("cannot extract address from public key")
			}
			vote.Nonce = sanitizeHex(votetx.Nonce)
			vote.VotePackage = sanitizeHex(votetx.VotePackage)
			vote.Signature = sanitizeHex(votetx.Signature)
			vote.Proof = sanitizeHex(votetx.Proof)
			vote.ProcessID = sanitizeHex(votetx.ProcessID)
			vote.Nullifier = GenerateNullifier(addr, vote.ProcessID)

		} else {
			return fmt.Errorf("invalid process type")
		}
		//log.Debugf("adding vote: %+v", vote)
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
			EntityID:             sanitizeHex(processtx.EntityID),
			EncryptionPublicKeys: processtx.EncryptionPublicKeys,
			MkRoot:               sanitizeHex(processtx.MkRoot),
			NumberOfBlocks:       processtx.NumberOfBlocks,
			StartBlock:           processtx.StartBlock,
			CurrentState:         vochaintypes.Scheduled,
			Type:                 processtx.ProcessType,
		}
		return state.AddProcess(newprocess, processtx.ProcessID)
	}
	return fmt.Errorf("invalid type")

}

// VoteTxCheck is an abstraction of ABCI checkTx for submitting a vote
func VoteTxCheck(vote vochaintypes.VoteTx, state *VochainState) error {
	process, _ := state.GetProcess(vote.ProcessID)
	if process == nil {
		return fmt.Errorf("process with id (%s) does not exists", vote.ProcessID)
	}

	if process.Type == "snark-vote" {
		voteID := fmt.Sprintf("%s_%s", signature.SanitizeHex(vote.ProcessID), signature.SanitizeHex(vote.Nullifier))
		v, _ := state.GetEnvelope(voteID)
		if v != nil {
			return fmt.Errorf("vote already exists")
		}
		// TODO check snark
	} else if process.Type == "poll-vote" || process.Type == "petition-sign" {
		var voteTmp vochaintypes.VoteTx
		voteTmp.Nonce = vote.Nonce
		voteTmp.ProcessID = vote.ProcessID
		voteTmp.Proof = vote.Proof
		voteTmp.VotePackage = vote.VotePackage

		voteBytes, err := json.Marshal(voteTmp)
		if err != nil {
			return fmt.Errorf("cannot marshal vote (%s)", err)
		}
		//log.Debugf("executing VoteTxCheck of: %s", voteBytes)
		pubKey, err := signature.PubKeyFromSignature(string(voteBytes), vote.Signature)
		if err != nil {
			log.Warnf("cannot extract pubKey: %s", err)
			return fmt.Errorf("cannot extract public key from signature (%s)", err)
		}

		addr, err := signature.AddrFromPublicKey(string(pubKey))
		if err != nil {
			return fmt.Errorf("cannot extract address from public key")
		}
		// assign a nullifier
		voteTmp.Nullifier = GenerateNullifier(addr, vote.ProcessID)

		// check if vote exists
		voteID := fmt.Sprintf("%s_%s", sanitizeHex(vote.ProcessID), sanitizeHex(voteTmp.Nullifier))
		v, _ := state.GetEnvelope(voteID)
		if v != nil {
			return fmt.Errorf("vote already exists")
		}
		// UNCOMMENT FOR POSEIDON HASH
		/*
			//log.Debugf("extracted pubkey: %s", pubKey)
			pubKeyHash := signature.HashPoseidon(pubKey)
			if len(pubKeyHash) > 32 || len(pubKeyHash) == 0 {
				return fmt.Errorf("wrong Poseidon hash size (%s)", err)
			}
			valid, err := checkMerkleProof(process.MkRoot, vote.Proof, pubKeyHash)
			if err != nil {
				return fmt.Errorf("cannot check merkle proof (%s)", err)
			}
			//log.Debugf("proof valid? %t", valid)
			if !valid {
				return fmt.Errorf("proof not valid")
			}
		*/
	} else {
		return fmt.Errorf("invalid process type")
	}
	return nil
}

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func NewProcessTxCheck(process vochaintypes.NewProcessTx, state *VochainState) error {
	// get oracles
	oracles, err := state.GetOracles()
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
	_, err = state.GetProcess(process.ProcessID)
	if err == nil {
		return fmt.Errorf("process with id (%s) already exists", process.ProcessID)
	}
	// check type
	if process.ProcessType == "snark-vote" || process.ProcessType == "poll-vote" || process.ProcessType == "petition-sign" {
		// ok
	} else {
		return fmt.Errorf("process type (%s) not valid", process.ProcessType)
	}
	return nil
}

// AdminTxCheck is an abstraction of ABCI checkTx for an admin transaction
func AdminTxCheck(adminTx vochaintypes.AdminTx, state *VochainState) error {
	// get oracles
	oracles, err := state.GetOracles()
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
func GenerateNullifier(address, processID string) string {
	return fmt.Sprintf("%x", signature.HashRaw(fmt.Sprintf("%s%s", signature.SanitizeHex(address), signature.SanitizeHex(processID))))
}

// GenerateAddressFromEd25519PublicKeyString returns the address as string from given pubkey represented as string
func GenerateAddressFromEd25519PublicKeyString(publicKey string) string {
	return crypto.Address(tmhash.SumTruncated([]byte(publicKey))).String()
}
