package vochain

import (
	codec "github.com/cosmos/cosmos-sdk/codec"
	eth "gitlab.com/vocdoni/go-dvote/crypto/signature"
	voctypes "gitlab.com/vocdoni/go-dvote/vochain/types"
)

// SplitAndCheckTxBytes splits a tx into method and args parts and does some basic checks
func SplitAndCheckTxBytes(content []byte) (voctypes.ValidTx, error) {
	var t voctypes.Tx
	var vt voctypes.ValidTx
	var err error

	err = codec.Cdc.UnmarshalJSON(content, &t)
	// unmarshal bytes
	if err != nil {
		return vt, err
	}

	// validate method name
	m := t.ValidateMethod()
	if m == "" {
		return vt, err
	}
	vt.Method = m

	// validate method args
	args, err := t.ValidateArgs()
	if err != nil {
		return vt, err
	}
	vt.Args = args

	return vt, nil
}

func VerifyAgainstOracles(oracles []eth.Address, message, signature string) bool {
	for c, k := range oracles {

	}
}
