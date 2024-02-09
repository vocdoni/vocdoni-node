package test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func TestAPIFarcasterVote(t *testing.T) {
	server := testcommon.APIserver{}
	server.Start(t,
		api.ChainHandler,
		api.CensusHandler,
		api.VoteHandler,
		api.AccountHandler,
		api.ElectionHandler,
		api.WalletHandler,
	)
	// Block 1
	server.VochainAPP.AdvanceTestBlock()

	token1 := uuid.New()
	c := testutil.NewTestHTTPclient(t, server.ListenAddr, &token1)

	// create a new census
	resp, code := c.Request("POST", nil, "censuses", "weighted")
	qt.Assert(t, code, qt.Equals, 200)
	censusData := &api.Census{}
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	id1 := censusData.CensusID.String()

	// add the farcaster hardcoded voters
	voter1PubKey, err := hex.DecodeString(frameVote1.pubkey)
	qt.Assert(t, err, qt.IsNil)
	voter1 := state.NewVoterID(state.VoterIDTypeEd25519, voter1PubKey)

	voter2PubKey, err := hex.DecodeString(frameVote2.pubkey)
	qt.Assert(t, err, qt.IsNil)
	voter2 := state.NewVoterID(state.VoterIDTypeEd25519, voter2PubKey)

	cparts := api.CensusParticipants{
		Participants: []api.CensusParticipant{
			{
				Key:    voter1.Address(),
				Weight: (*types.BigInt)(big.NewInt(int64(1))),
			},
			{
				Key:    voter2.Address(),
				Weight: (*types.BigInt)(big.NewInt(int64(1))),
			},
		},
	}

	_, code = c.Request("POST", &cparts, "censuses", id1, "participants")
	qt.Assert(t, code, qt.Equals, 200)

	resp, code = c.Request("POST", nil, "censuses", id1, "publish")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.CensusID, qt.IsNotNil)
	root := censusData.CensusID

	election := createFarcasterElection(t, c, server.Account, censusData.CensusID, server.VochainAPP.ChainID())

	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 2)

	// Send the first vote
	resp, code = c.Request("GET", nil, "censuses", root.String(), "proof", fmt.Sprintf("%x", voter1.Address()))
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, "1")

	votePackage := &state.VotePackage{
		Votes: []int{frameVote1.buttonIndex},
	}
	votePackageBytes, err := votePackage.Encode()
	qt.Assert(t, err, qt.IsNil)

	vote := &models.VoteEnvelope{
		Nonce:       util.RandomBytes(16),
		ProcessId:   election.ElectionID,
		VotePackage: votePackageBytes,
	}

	frameSignedMessage, err := hex.DecodeString(frameVote1.signedMessage)
	qt.Assert(t, err, qt.IsNil)

	vote.Proof = &models.Proof{
		Payload: &models.Proof_FarcasterFrame{
			FarcasterFrame: &models.ProofFarcasterFrame{
				SignedFrameMessageBody: frameSignedMessage,
				PublicKey:              voter1PubKey,
				CensusProof: &models.ProofArbo{
					Type:            models.ProofArbo_BLAKE2B,
					Siblings:        censusData.CensusProof,
					AvailableWeight: censusData.Value,
					VoteWeight:      censusData.Value,
				},
			},
		},
	}

	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote}})
	qt.Assert(t, err, qt.IsNil)
	stxb, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	v := &api.Vote{TxPayload: stxb}
	resp, code = c.Request("POST", v, "votes")
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &v)
	qt.Assert(t, err, qt.IsNil)

	// Block 3
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 3)

	// Verify the vote
	_, code = c.Request("GET", nil, "votes", "verify", election.ElectionID.String(), v.VoteID.String())
	qt.Assert(t, code, qt.Equals, 200)

	// Get the vote and check the data
	resp, code = c.Request("GET", nil, "votes", v.VoteID.String())
	qt.Assert(t, code, qt.Equals, 200)
	v2 := &api.Vote{}
	err = json.Unmarshal(resp, v2)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v2.VoteID.String(), qt.Equals, v.VoteID.String())
	qt.Assert(t, v2.BlockHeight, qt.Equals, uint32(2))
	qt.Assert(t, *v2.TransactionIndex, qt.Equals, int32(0))
	qt.Assert(t, v2.VoterID.String(), qt.Equals, hex.EncodeToString(voter1.Address()))

	// Test sending the same vote again (should fail)
	v = &api.Vote{TxPayload: stxb}
	_, code = c.Request("POST", v, "votes")
	qt.Assert(t, code, qt.Equals, 500)

	// Test sending the second vote
	server.VochainAPP.AdvanceTestBlock()
	resp, code = c.Request("GET", nil, "censuses", root.String(), "proof", fmt.Sprintf("%x", voter2.Address()))
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, "1")

	votePackage = &state.VotePackage{
		Votes: []int{frameVote2.buttonIndex},
	}
	votePackageBytes, err = votePackage.Encode()
	qt.Assert(t, err, qt.IsNil)

	vote = &models.VoteEnvelope{
		Nonce:       util.RandomBytes(16),
		ProcessId:   election.ElectionID,
		VotePackage: votePackageBytes,
	}

	frameSignedMessage, err = hex.DecodeString(frameVote2.signedMessage)
	qt.Assert(t, err, qt.IsNil)

	vote.Proof = &models.Proof{
		Payload: &models.Proof_FarcasterFrame{
			FarcasterFrame: &models.ProofFarcasterFrame{
				SignedFrameMessageBody: frameSignedMessage,
				PublicKey:              voter2PubKey,
				CensusProof: &models.ProofArbo{
					Type:            models.ProofArbo_BLAKE2B,
					Siblings:        censusData.CensusProof,
					AvailableWeight: censusData.Value,
					VoteWeight:      censusData.Value,
				},
			},
		},
	}

	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote}})
	qt.Assert(t, err, qt.IsNil)
	stxb, err = proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	v = &api.Vote{TxPayload: stxb}
	resp, code = c.Request("POST", v, "votes")
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &v)
	qt.Assert(t, err, qt.IsNil)
}

func createFarcasterElection(t testing.TB, c *testutil.TestHTTPclient,
	signer *ethereum.SignKeys,
	censusRoot types.HexBytes,
	chainID string,
) api.ElectionCreate {
	metadataBytes, err := json.Marshal(
		&api.ElectionMetadata{
			Title:       map[string]string{"default": "test farcaster election"},
			Description: map[string]string{"default": "test farcaster election description"},
			Version:     "1.0",
		})

	qt.Assert(t, err, qt.IsNil)
	metadataURI := ipfs.CalculateCIDv1json(metadataBytes)

	tx := models.Tx_NewProcess{
		NewProcess: &models.NewProcessTx{
			Txtype: models.TxType_NEW_PROCESS,
			Nonce:  0,
			Process: &models.Process{
				StartTime:    0,
				Duration:     100,
				Status:       models.ProcessStatus_READY,
				CensusRoot:   censusRoot,
				CensusOrigin: models.CensusOrigin_FARCASTER_FRAME,
				Mode:         &models.ProcessMode{AutoStart: true, Interruptible: true},
				VoteOptions: &models.ProcessVoteOptions{
					MaxCount: 1,
					MaxValue: 4,
				},
				EnvelopeType: &models.EnvelopeType{
					EncryptedVotes: false,
					Anonymous:      false,
				},
				Metadata:      &metadataURI,
				MaxCensusSize: 10,
			},
		},
	}

	txb, err := proto.Marshal(&models.Tx{Payload: &tx})
	qt.Assert(t, err, qt.IsNil)
	signedTxb, err := signer.SignVocdoniTx(txb, chainID)
	qt.Assert(t, err, qt.IsNil)
	stx := models.SignedTx{Tx: txb, Signature: signedTxb}
	stxb, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	election := api.ElectionCreate{
		TxPayload: stxb,
		Metadata:  metadataBytes,
	}
	resp, code := c.Request("POST", election, "elections")
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &election)
	qt.Assert(t, err, qt.IsNil)

	return election
}

type farcasterFrame struct {
	signedMessage string
	pubkey        string
	buttonIndex   int
}

var frameVote1 = farcasterFrame{
	signedMessage: "0a4a080d109fc20e18ecb4de2e200182013a0a1a68747470733a2f2f63656c6f6e692e766f63646f6e692e6e657410021a1a089fc20e121423f83af6df9c2c960a5ba50af25fe887325bf91812144183c8a56b1d3e1874a4165013cc92e067a362b01801224022709f664c73d7fed004ab7f6bc09c376a17abb221d19096fa739f0422172535151c0cc5382aceed7338356adae00982c9ae69746fca3c9fbebb23e832562a0328013220ec327cd438995a59ce78fddd29631e9b2e41eafc3a6946dd26b4da749f47140d",
	pubkey:        "ec327cd438995a59ce78fddd29631e9b2e41eafc3a6946dd26b4da749f47140d",
	buttonIndex:   2,
}

var frameVote2 = farcasterFrame{
	signedMessage: "0a4a080d10eced12188bc0de2e200182013a0a1a68747470733a2f2f63656c6f6e692e766f63646f6e692e6e657410021a1a089fc20e121423f83af6df9c2c960a5ba50af25fe887325bf9181214da970b6c4b9e905c9dd95965b5e5e300e88bc5b5180122409e819761b9292c72d48856369ba84926513ef9a341d4564a40d028dd8651d9c305c163023ac440dc3ca93a2993c12b5c3a7361215b913e8e7e9771b41bd6190c28013220d843cf9636184c99fe6ed7db8b5934c2cd31dd19b63a2409b305898fa560459c",
	pubkey:        "d843cf9636184c99fe6ed7db8b5934c2cd31dd19b63a2409b305898fa560459c",
	buttonIndex:   2,
}
