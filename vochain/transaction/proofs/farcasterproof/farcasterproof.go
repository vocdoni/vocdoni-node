package farcasterproof

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
	"lukechampine.com/blake3"

	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/proofs/arboproof"
	farcasterpb "go.vocdoni.io/dvote/vochain/transaction/proofs/farcasterproof/proto"
)

const (
	frameHashSize           = 20
	pollURLpattern          = `([0-9a-fA-F]{64})`
	pollURLpatternShortened = `([0-9a-zA-Z+/]{8})`
)

var (
	// DisableElectionIDVerification is a flag to dissable the election ID verification on the poll URL.
	// This should be used only for testing purposes.
	DisableElectionIDVerification = false
	re                            *regexp.Regexp
	reShortened                   *regexp.Regexp
)

func init() {
	re = regexp.MustCompile(pollURLpattern)
	reShortened = regexp.MustCompile(pollURLpatternShortened)
}

// FarcasterVerifier is a proof verifier for the Farcaster frame protocol.
type FarcasterVerifier struct{}

// Verify checks the validity of a Farcaster frame proof.
func (*FarcasterVerifier) Verify(process *models.Process, envelope *models.VoteEnvelope, vID state.VoterID) (bool, *big.Int, error) {
	proof := envelope.Proof.GetFarcasterFrame()
	if proof == nil {
		return false, nil, fmt.Errorf("farcaster proof is empty")
	}
	// Verify the frame signature and extract the public key
	frameAction, pubkey, err := VerifyFrameSignature(proof.SignedFrameMessageBody)
	if err != nil {
		return false, nil, fmt.Errorf("failed to verify farcaster frame signature: %w", err)
	}
	// Verify the voter ID matches the frame action public key
	frameVoterID := state.NewVoterID(state.VoterIDTypeEd25519, pubkey)
	if !bytes.Equal(frameVoterID.Address(), vID.Address()) {
		return false, nil, fmt.Errorf("voter ID mismatch (got %x, expected %x)", frameVoterID.Address(), vID.Address())
	}

	// Verify the vote package matches the frame action
	if envelope.VotePackage == nil {
		return false, nil, fmt.Errorf("vote package is empty")
	}
	vp := state.VotePackage{}
	if err := vp.Decode(envelope.VotePackage); err != nil {
		return false, nil, fmt.Errorf("failed to decode vote package: %w", err)
	}
	if len(vp.Votes) > 1 {
		return false, nil, fmt.Errorf("vote package contains more than one vote")
	}

	if uint32(vp.Votes[0]) != frameAction.ButtonIndex-1 {
		return false, nil, fmt.Errorf("vote package button index mismatch (got %d, expected %d)", frameAction.ButtonIndex, vp.Votes[0]+1)
	}

	// Verify the vote URL matches with the process ID. We enforce the process ID to be present in the poll URL because it
	// is the only way to ensure the process ID is correct.
	if !DisableElectionIDVerification {
		matches := re.FindStringSubmatch(string(frameAction.Url))
		// If a match is found, matches[1] contains the processID
		if len(matches) > 1 {
			votePID, err := hex.DecodeString(matches[1])
			if err != nil {
				return false, nil, fmt.Errorf("failed to decode process ID: %w", err)
			}
			if !bytes.Equal(votePID, envelope.ProcessId) {
				return false, nil, fmt.Errorf("process ID mismatch (got %x, expected %x)", votePID, envelope.ProcessId)
			}
		} else {
			// If no match is found, we try to match a shortened process ID
			matches = reShortened.FindStringSubmatch(string(frameAction.Url))
			if len(matches) > 1 {
				if matches[1] != generateShortenedProcessID(envelope.ProcessId) {
					return false, nil, fmt.Errorf("shortened process ID mismatch (got %s, expected %s)",
						matches[1], generateShortenedProcessID(envelope.ProcessId))
				}
			} else {
				// If no match is found, we return an error
				return false, nil, fmt.Errorf("no process ID found on poll URL")
			}
		}
	}

	// Verify the census arbo proof (is the signer of the frame action allowed to vote?)
	arboVerifier := arboproof.ProofVerifierArbo{}
	valid, weight, err := arboVerifier.Verify(process, &models.VoteEnvelope{
		Proof: &models.Proof{
			Payload: &models.Proof_Arbo{Arbo: proof.CensusProof},
		},
		ProcessId: envelope.ProcessId,
	}, vID)
	if err != nil {
		return false, nil, fmt.Errorf("failed to verify arbo proof: %w", err)
	}
	if !valid {
		return false, weight, fmt.Errorf("census proof is invalid")
	}
	return true, weight, nil
}

// DecodeMessage decodes the signed message body and returns the frame action body, the message data and the public key.
func DecodeMessage(signedMessageBody []byte) (*farcasterpb.FrameActionBody, *farcasterpb.Message, error) {
	msg := farcasterpb.Message{}
	if err := proto.Unmarshal(signedMessageBody, &msg); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal Message: %w", err)
	}
	if msg.Data == nil {
		return nil, nil, fmt.Errorf("invalid message data")
	}
	if msg.Data.Type != farcasterpb.MessageType_MESSAGE_TYPE_FRAME_ACTION {
		return nil, nil, fmt.Errorf("invalid message type, got %s", msg.Data.Type.String())
	}
	if msg.SignatureScheme != farcasterpb.SignatureScheme_SIGNATURE_SCHEME_ED25519 {
		return nil, nil, fmt.Errorf("invalid signature scheme")
	}
	actionBody := msg.Data.GetFrameActionBody()
	if actionBody == nil {
		return nil, nil, fmt.Errorf("invalid action body")
	}
	return actionBody, &msg, nil
}

// VerifyFrameSignature validates the frame message and returns de deserialized frame action and public key.
func VerifyFrameSignature(messageBody []byte) (*farcasterpb.FrameActionBody, ed25519.PublicKey, error) {
	actionBody, msg, err := DecodeMessage(messageBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode message body: %w", err)
	}

	pubkey := msg.GetSigner()
	if pubkey == nil {
		return nil, nil, fmt.Errorf("signer is nil")
	}

	// Verify the hash and signature
	msgDataBytes, err := proto.Marshal(msg.Data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal message data: %w", err)
	}

	log.Debugw("verifying message signature", "size", len(msgDataBytes))
	h := blake3.New(160, nil)
	if _, err := h.Write(msgDataBytes); err != nil {
		return nil, nil, fmt.Errorf("failed to hash message: %w", err)
	}
	hashed := h.Sum(nil)[:frameHashSize]

	if !bytes.Equal(msg.Hash, hashed) {
		return nil, nil, fmt.Errorf("hash mismatch (got %x, expected %x)", hashed, msg.Hash)
	}

	if !ed25519.Verify(pubkey, hashed, msg.GetSignature()) {
		return nil, nil, fmt.Errorf("signature verification failed")
	}

	return actionBody, pubkey, nil
}

// InitializeFarcasterFrameVote initializes a farcaster frame vote. It does not check the proof nor includes the weight of the vote.
func InitializeFarcasterFrameVote(voteEnvelope *models.VoteEnvelope, height uint32) (*state.Vote, error) {
	// Create a new vote object with the provided parameters
	vote := &state.Vote{
		Height:      height,
		ProcessID:   voteEnvelope.ProcessId,
		VotePackage: voteEnvelope.VotePackage,
	}
	// Check if the proof is nil or invalid
	if voteEnvelope.Proof == nil {
		return nil, fmt.Errorf("proof not found on transaction")
	}
	if voteEnvelope.Proof.Payload == nil {
		return nil, fmt.Errorf("invalid proof payload provided")
	}
	frameProof := voteEnvelope.Proof.GetFarcasterFrame()
	if frameProof == nil {
		return nil, fmt.Errorf("farcaster frame proof not found on transaction")
	}
	if frameProof.PublicKey == nil {
		return nil, fmt.Errorf("farcaster frame public key not found on transaction")
	}
	_, msg, err := DecodeMessage(frameProof.SignedFrameMessageBody)
	if err != nil {
		return nil, fmt.Errorf("failed to decode farcaster frame message: %w", err)
	}
	if msg.Data == nil {
		return nil, fmt.Errorf("farcaster frame cast ID not found on transaction")
	}
	// Generate the voter ID and assign it to the vote
	vote.VoterID = append([]byte{state.VoterIDTypeEd25519}, frameProof.PublicKey...)
	// Generate the nullifier
	vote.Nullifier = GenerateNullifier(msg.Data.Fid, voteEnvelope.ProcessId)
	return vote, nil
}

// GenerateNullifier generates a nullifier for a farcaster frame vote.
// As nullifier we use: hash(farcasterID+processID) because the farcasterID is unique per voter while
// the public key is not.
func GenerateNullifier(farcasterID uint64, processID []byte) []byte {
	fidBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(fidBytes, farcasterID)
	return ethereum.HashRaw(append(fidBytes, processID...))
}

func generateShortenedProcessID(processID []byte) string {
	// We take the 8 first chars of the base64 encoded hash of the processID.
	// The probability of at least one collision among 100,000 generated hashes is approximately 0.0018%.
	// The probability of at least one collision among 1,000,000 generated hashes is approximately 0.177%.
	hash := blake3.Sum256(processID)
	return base64.StdEncoding.EncodeToString(hash[:])[:8]
}
