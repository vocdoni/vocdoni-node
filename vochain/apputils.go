package vochain

import (
	"math"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// GenerateFaucetPackage generates a faucet package.
// The package is signed by the given `from` key (holder of the funds) and sent to the `to` address.
// The `amount` is the amount of tokens to be sent.
func GenerateFaucetPackage(from *ethereum.SignKeys, to ethcommon.Address, amount uint64) (*models.FaucetPackage, error) {
	nonce := util.RandomInt(0, math.MaxInt32)
	payload := &models.FaucetPayload{
		Identifier: uint64(nonce),
		To:         to.Bytes(),
		Amount:     amount,
	}
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return nil, err
	}
	payloadSignature, err := from.SignEthereum(payloadBytes)
	if err != nil {
		return nil, err
	}
	return &models.FaucetPackage{
		Payload:   payloadBytes,
		Signature: payloadSignature,
	}, nil
}
