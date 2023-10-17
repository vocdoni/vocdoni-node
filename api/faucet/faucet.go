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
	networks   map[string]uint64
}

// AttachFaucetAPI attaches the faucet API to the given http apirest router.
// The path prefix is used to define the base path in which the endpoint method will be registered.
// For example, if the pathPrefix is "/faucet", the resulting endpoint is /faucet/{network}/{to}.
// The networks map defines the amount of tokens to send for each network. Networks not defined are
// considered invalid.
func AttachFaucetAPI(signingKey *ethereum.SignKeys, networks map[string]uint64,
	api *apirest.API, pathPrefix string) error {
	f := &FaucetAPI{
		signingKey: signingKey,
		networks:   networks,
	}
	return api.RegisterMethod(
		path.Join(pathPrefix, "{network}/{to}"),
		"GET",
		apirest.MethodAccessTypePublic,
		f.faucetHandler,
	)
}

func (f *FaucetAPI) faucetHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// check network is correct and get amount to send
	network := ctx.URLParam("network")
	amount, ok := f.networks[network]
	if !ok || amount == 0 {
		return api.ErrParamNetworkInvalid
	}
	// get TO address
	toStr := ctx.URLParam("to")
	if !common.IsHexAddress(toStr) {
		return api.ErrParamToInvalid
	}
	to := common.HexToAddress(toStr)

	// generate faucet package
	log.Debugf("faucet request from %s for network %s", to, network)
	fpackage, err := vochain.GenerateFaucetPackage(f.signingKey, to, amount)
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
		Amount:        fmt.Sprint(amount),
		FaucetPackage: fpackageBytes,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}
