package faucet

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain"
)

const (
	FaucetHandler = "faucet"
)

// FaucetAPI is a httprouter/apirest handler for the faucet.
// It generates a signed package that can be used to request tokens from the faucet.
type FaucetAPI struct {
	signingKey *ethereum.SignKeys
	amount     uint64
}

// AttachFaucetAPI attaches the faucet API to the given http apirest router.
// The path prefix is used to define the base path in which the endpoint method will be registered.
// For example, if the pathPrefix is "/faucet", the resulting endpoint is /faucet/{network}/{to}.
// The networks map defines the amount of tokens to send for each network. Networks not defined are
// considered invalid.
func AttachFaucetAPI(signingKey *ethereum.SignKeys, amount uint64,
	api *apirest.API, pathPrefix string) error {
	f := &FaucetAPI{
		signingKey: signingKey,
		amount:     amount,
	}
	return api.RegisterMethod(
		path.Join(pathPrefix, "{to}"),
		"GET",
		apirest.MethodAccessTypePublic,
		f.faucetHandler,
	)
}

func (f *FaucetAPI) faucetHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// check network is correct and get amount to send
	// get TO address
	toStr := ctx.URLParam("to")
	if !common.IsHexAddress(toStr) {
		return api.ErrParamToInvalid
	}
	to := common.HexToAddress(toStr)

	// generate faucet package
	log.Debugw("faucet request", "from", to.String(), "amount", f.amount)
	fpackage, err := vochain.GenerateFaucetPackage(f.signingKey, to, f.amount)
	if err != nil {
		return api.ErrCantGenerateFaucetPkg.WithErr(err)
	}
	fpackageBytes, err := json.Marshal(FaucetPackage{
		FaucetPayload: fpackage.Payload,
		Signature:     fpackage.Signature,
	})
	if err != nil {
		return err
	}
	// send response
	resp := &FaucetResponse{
		Amount:        fmt.Sprintf("%d", f.amount),
		FaucetPackage: fpackageBytes,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}
