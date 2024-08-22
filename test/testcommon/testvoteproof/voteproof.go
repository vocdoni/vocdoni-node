package testvoteproof

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func GetCSPproofBatch(signers []*ethereum.SignKeys,
	csp *ethereum.SignKeys, pid []byte,
) ([]types.HexBytes, error) {
	var proofs []types.HexBytes
	for _, k := range signers {
		bundle := &models.CAbundle{
			ProcessId: pid,
			Address:   k.Address().Bytes(),
		}
		bundleBytes, err := proto.Marshal(bundle)
		if err != nil {
			log.Fatal(err)
		}
		signature, err := csp.SignEthereum(bundleBytes)
		if err != nil {
			log.Fatal(err)
		}

		caProof := &models.ProofCA{
			Bundle:    bundle,
			Type:      models.ProofCA_ECDSA,
			Signature: signature,
		}
		caProofBytes, err := proto.Marshal(caProof)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, caProofBytes)
	}
	return proofs, nil
}

// CreateKeysAndBuildCensus creates a bunch of random keys and a new census tree.
// It returns the keys, the census root and the proofs for each key.
func CreateKeysAndBuildCensus(t *testing.T, size int) ([]*ethereum.SignKeys, []byte, [][]byte) {
	db := metadb.NewTest(t)
	tr, err := censustree.New(censustree.Options{
		Name: "testcensus", ParentDB: db,
		MaxLevels: censustree.DefaultMaxLevels, CensusType: models.Census_ARBO_BLAKE2B,
	})
	if err != nil {
		t.Fatal(err)
	}

	keys := ethereum.NewSignKeysBatch(size)
	hashedKeys := [][]byte{}
	for _, k := range keys {
		c, err := tr.Hash(k.Address().Bytes())
		qt.Check(t, err, qt.IsNil)
		c = c[:censustree.DefaultMaxKeyLen]
		err = tr.Add(c, nil)
		qt.Check(t, err, qt.IsNil)
		hashedKeys = append(hashedKeys, c)
	}

	var proofs [][]byte
	for i := range keys {
		_, proof, err := tr.GenProof(hashedKeys[i])
		qt.Check(t, err, qt.IsNil)
		proofs = append(proofs, proof)
	}
	root, err := tr.Root()
	qt.Check(t, err, qt.IsNil)
	return keys, root, proofs
}

// BuildSignedVoteForOffChainTree builds a signed vote for an off-chain merkle-tree election.
func BuildSignedVoteForOffChainTree(t *testing.T, electionID []byte, key *ethereum.SignKeys,
	proof []byte, votePackage []int, chainID string,
) *models.SignedTx {
	var stx models.SignedTx
	var err error
	vp, err := json.Marshal(votePackage)
	qt.Check(t, err, qt.IsNil)
	vote := &models.VoteEnvelope{
		Nonce:     util.RandomBytes(32),
		ProcessId: electionID,
		Proof: &models.Proof{
			Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:     models.ProofArbo_BLAKE2B,
					Siblings: proof,
					KeyType:  models.ProofArbo_ADDRESS,
				},
			},
		},
		VotePackage: vp,
	}

	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_Vote{Vote: vote},
	})
	qt.Check(t, err, qt.IsNil)
	stx.Signature, err = key.SignVocdoniTx(stx.Tx, chainID)
	qt.Check(t, err, qt.IsNil)
	return &stx
}
