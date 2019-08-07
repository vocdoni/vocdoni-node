package vochain

import (
	"github.com/cosmos/cosmos-sdk/codec"
	vlog "gitlab.com/vocdoni/go-dvote/log"
	voctypes "gitlab.com/vocdoni/go-dvote/vochain/types"
)

func splitTx(content []byte) voctypes.Tx {
	var validTx voctypes.Tx
	err := codec.Cdc.UnmarshalJSON(content, &validTx)
	vlog.Infof("%s", validTx.String())
	txMethod := validTx.GetMethod()
	
	if err != nil {
		vlog.Infof("%s", err)
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
