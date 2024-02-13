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
	"go.vocdoni.io/dvote/vochain/transaction/proofs/farcasterproof"
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
	// We need to disable the election ID verification, since the signed farcaster frame messages are hardcoded and contain the processId in the URL.
	farcasterproof.DisableElectionIDVerification = true
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

	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 2)

	// Send the first vote
	resp, code = c.Request("GET", nil, "censuses", root.String(), "proof", fmt.Sprintf("%x", voter1.Address()))
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, "1")

	votePackage := &state.VotePackage{
		Votes: []int{frameVote1.buttonIndex - 1},
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
		Votes: []int{frameVote2.buttonIndex - 1},
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

	// first attempt with the wrong pubKey (should fail)
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

	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote}})
	qt.Assert(t, err, qt.IsNil)
	stxb, err = proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	v = &api.Vote{TxPayload: stxb}
	_, code = c.Request("POST", v, "votes")
	qt.Assert(t, code, qt.Equals, 500)

	// second attempt with the good pubKey (should work)
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
					MaxValue: 3,
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

// var farcasterElectionID = "63f57be98f806f959214b4581eb8791e6c80eaf72102d4e93f76100000000003"

type farcasterFrame struct {
	signedMessage string
	pubkey        string
	buttonIndex   int
	fid           int
}

var frameVote1 = farcasterFrame{
	signedMessage: "0a8b01080d109fc20e18b4a7f52e200182017b0a5b68747470733a2f2f63656c6f6e692e766f63646f6e692e6e65742f3633663537626539386638303666393539323134623435383165623837393165366338306561663732313032643465393366373631303030303030303030303310011a1a089fc20e1214000000000000000000000000000000000000000112142d9bd29806c7e54cf5f80f98d9adf710a2ebc58518012240379f4f9897901b24544fba46fcb51183f79d79a5041c47f554c5a4e407c020fdbf43ef27490b944e05372850b1dc78dd97c728a88bdbc14f0174ed589c795a0928013220ec327cd438995a59ce78fddd29631e9b2e41eafc3a6946dd26b4da749f47140d",
	pubkey:        "ec327cd438995a59ce78fddd29631e9b2e41eafc3a6946dd26b4da749f47140d",
	buttonIndex:   1,
	fid:           237855,
}

var frameVote2 = farcasterFrame{
	buttonIndex:   3,
	signedMessage: "0a8901080d10e04e18d1aaf52e200182017a0a5b68747470733a2f2f63656c6f6e692e766f63646f6e692e6e65742f3633663537626539386638303666393539323134623435383165623837393165366338306561663732313032643465393366373631303030303030303030303310031a1908e04e12140000000000000000000000000000000000000001121496f560a1f5c90fa24278277321d7be35d18cf0711801224029863962ecff4b7db6dd8736fc1c238ed6ed5a147d3a36e6eac32e06f10d2dcc1df1618d5da6ce21286e0233656ef985a5b1cced2bee5f2cbb4fd1bfc168aa0128013220d6424e655287aa61df38205da19ddab23b0ff9683c6800e0dbc3e8b65d3eb2e3",
	pubkey:        "d6424e655287aa61df38205da19ddab23b0ff9683c6800e0dbc3e8b65d3eb2e3",
	fid:           10080,
}

//var frameVote3 = farcasterFrame{
//	buttonIndex:   1,
//	fid:           195929,
//	signedMessage: "0a8b01080d10d9fa0b18f1a8f52e200182017b0a5b68747470733a2f2f63656c6f6e692e766f63646f6e692e6e65742f3633663537626539386638303666393539323134623435383165623837393165366338306561663732313032643465393366373631303030303030303030303310011a1a08d9fa0b121400000000000000000000000000000000000000011214e597fda6f6252ae1d20011d81072a13b17afa0901801224089b9d6427af2558188669b498890871d733f67d51c2df0d93a3da84727d09f0d87a6b9b83b02adcb5546b93619b0d8395cc84d675c3f68f92e55db526710c10a28013220b44b21f817a968423a3669c13865deafa7389ae0df89c6ad615cfc17b86118f8",
//	pubkey:        "b44b21f817a968423a3669c13865deafa7389ae0df89c6ad615cfc17b86118f8",
//}
