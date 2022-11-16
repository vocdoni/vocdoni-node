package testutil

import (
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func GetCSPproofBatch(signers []*ethereum.SignKeys,
	csp *ethereum.SignKeys, pid []byte) ([]types.HexBytes, error) {
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
