package api

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
)

const (
	WalletHandler = "wallet"

	// ParamPrivateKey is the URL param of POST /wallet/add/{privateKey}.
	ParamPrivateKey = "privateKey"
)

const (
	// walletAddDeprecation is the RFC 9745 Deprecation header value of
	// POST /wallet/add/{privateKey}: 2026-10-22T00:00:00Z as a Unix timestamp.
	walletAddDeprecation = "@1792627200"
	// walletAddWarning tells a client reading the headers that the key is not
	// kept and that the token only serves the census endpoints.
	walletAddWarning = "the private key is not stored: the token is only a bearer for the /censuses endpoints, " +
		reasonWallet
)

func (a *API) enableWalletHandlers() error {
	return a.Endpoint.RegisterMethod(
		"/wallet/add/{privateKey}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.walletAddHandler,
	)
}

// walletAddHandler
//
// It exists so census clients get a bearer token: @vocdoni/sdk calls it with a
// throwaway key before it creates a census. The private key is only used to
// derive its address and it is not stored. The node's request logs redact it
// (httprouter.RedactRequestURI), but it travels in the URL, so proxies and
// access logs in front of the node may record it: call this route only with a
// freshly generated key that holds nothing. The token is a new random UUID,
// which is what the /censuses endpoints accept as bearer.
//
// Every reply carries Deprecation and Warning headers, and every call is
// logged with the caller's address and user agent, so the remaining callers
// can be found before the route is removed.
//
//	@Summary		Get a bearer token for the census endpoints
//	@Description	Validates the hex private key (with or without 0x), derives its address and returns it with a new random UUID token. The key is not stored, but it travels in the URL and proxies may log it: use only a freshly generated key that holds nothing. The token is the bearer the /censuses endpoints accept.
//	@Deprecated
//	@Tags		Censuses
//	@Produce	json
//	@Param		privateKey	path		string	true	"Hex encoded private key"
//	@Success	200			{object}	WalletToken
//	@Router		/wallet/add/{privateKey} [post]
func (*API) walletAddHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	r := ctx.Request
	log.Infow("deprecated route called", "route", "POST /wallet/add/{privateKey}",
		"remoteAddr", r.RemoteAddr,
		"cfConnectingIP", r.Header.Get("CF-Connecting-IP"),
		"xForwardedFor", r.Header.Get("X-Forwarded-For"),
		"userAgent", r.UserAgent(),
		"origin", r.Header.Get("Origin"))
	ctx.SetHeader("Deprecation", walletAddDeprecation)
	ctx.SetHeader("Warning", fmt.Sprintf("299 - %q", walletAddWarning))

	signer := ethereum.NewSignKeys()
	if err := signer.AddHexKey(ctx.URLParam(ParamPrivateKey)); err != nil {
		return ErrCantParsePrivateKey
	}
	data, err := json.Marshal(WalletToken{
		Token:   uuid.New(),
		Address: signer.Address().Bytes(),
	})
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}
