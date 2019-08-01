package vochain

import (
	"github.com/cosmos/cosmos-sdk/codec"
	vlog "gitlab.com/vocdoni/go-dvote/log"
	voctypes "gitlab.com/vocdoni/go-dvote/vochain/types"
)

func splitTx(content []byte) voctypes.ValidTx {
	vlog.Infof("%+v", content)
	var validTx voctypes.ValidTx
	err := codec.Cdc.UnmarshalJSON(content, &validTx)
	txMethod, err := validTx.GetMethod()
	if err != nil {
		vlog.Fatalf("%s", err)
	}
	if txMethod != "" {
		validTx.Method = txMethod
	}
	return validTx
}

func validateHash(hash []byte) bool {
	// VALIDATE HASH LENGTH AND FORMAT
	return false
}

func validateSignature(sig []byte) bool {
	// VALIDATE WITH SIGNATURE TYPE
	return false
}

func verifySignature(sig []byte, pubk []byte) bool {
	// RETURNS TRUE IF SIG CORRESPONDS TO PASSED PUBK
	return false
}
