package api

import (
	"encoding/json"

	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/state"
)

const (
	SIKHandler = "sik"
)

func (a *API) enableSIKHandlers() error {
	if err := a.endpoint.RegisterMethod(
		"/sik/roots",
		"GET",
		apirest.MethodAccessTypePublic,
		a.sikValidRootsHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/sik/proof/{address}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.sikProofHandler,
	); err != nil {
		return err
	}

	return nil
}

// sikValidRootsHandler
//
//	@Summary		List of valid SIK roots
//	@Description	Returns the list of currently valid roots of the merkle tree where the vochain account SIK's are stored.
//	@Tags			SIK
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	uint64
//	@Success		200	{object}	object{sikroots=[]string}
//	@Router			/sik/roots [get]
func (a *API) sikValidRootsHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	validSIKRoots, err := a.vocapp.State.ValidSIKRoots()
	if err != nil {
		return err
	}

	var sikRoots []types.HexBytes
	for _, root := range validSIKRoots {
		sikRoots = append(sikRoots, root)
	}
	data, err := json.Marshal(
		struct {
			SIKRoots []types.HexBytes `json:"sikroots"`
		}{SIKRoots: sikRoots},
	)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// sikProofHandler
//
//	@Summary		List of valid SIK roots
//	@Description	Returns the list of currently valid roots of the merkle tree where the vochain account SIK's are stored.
//	@Tags			SIK
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	uint64
//	@Success		200	{object}	object{sikproof=string, sikroot=string, siksiblings=[]string}
//	@Router			/sik/proof/{address} [get]
func (a *API) sikProofHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// get the address from the
	address, err := censusKeyParse(ctx.URLParam("address"))
	if err != nil {
		return err
	}
	// get the sik tree from the state
	sikTree, err := a.vocapp.State.Tx.DeepSubTree(state.StateTreeCfg(state.TreeSIK))
	if err != nil {
		return ErrCantGetCircomSiblings.WithErr(err)
	}
	response := Census{}
	// get merkle root
	if response.CensusRoot, err = sikTree.Root(); err != nil {
		return ErrCantGetCircomSiblings.WithErr(err)
	}
	// get sik merkle tree proof
	if response.Value, response.CensusProof, err = sikTree.GenProof(address); err != nil {
		return ErrCantGetCircomSiblings.WithErr(err)
	}
	// get sik merkle tree circom siblings
	if response.CensusSiblings, err = zk.ProofToCircomSiblings(response.CensusProof); err != nil {
		return ErrCantGetCircomSiblings.WithErr(err)
	}
	// encode and send the sikproof
	data, err := json.Marshal(response)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}
