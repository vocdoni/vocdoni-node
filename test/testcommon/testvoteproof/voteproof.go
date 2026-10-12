package testvoteproof

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
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

// CreateKeysAndBuildCensus creates a bunch of random voter keys and a CSP
// that authorizes them to vote on the election pid. It returns the keys, the
// census root (the CSP public key) and the CSP proof of each key.
func CreateKeysAndBuildCensus(t *testing.T, size int, pid []byte) ([]*ethereum.SignKeys, []byte, []*models.ProofCA) {
	csp := ethereum.NewSignKeys()
	qt.Assert(t, csp.Generate(), qt.IsNil)
	keys := ethereum.NewSignKeysBatch(size)
	proofs := make([]*models.ProofCA, 0, size)
	for _, k := range keys {
		bundle := &models.CAbundle{
			ProcessId: pid,
			Address:   k.Address().Bytes(),
		}
		bundleBytes, err := proto.Marshal(bundle)
		qt.Assert(t, err, qt.IsNil)
		signature, err := csp.SignEthereum(bundleBytes)
		qt.Assert(t, err, qt.IsNil)
		proofs = append(proofs, &models.ProofCA{
			Type:      models.ProofCA_ECDSA,
			Bundle:    bundle,
			Signature: signature,
		})
	}
	return keys, csp.PublicKey(), proofs
}
