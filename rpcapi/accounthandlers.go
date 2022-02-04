package rpcapi

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	api "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/types"
)

func (r *RPCAPI) getAccount(request *api.APIrequest) (*api.APIresponse, error) {
	// check entity id
	if len(request.EntityId) != types.EntityIDsize {
		return nil, fmt.Errorf("cannot get account info: (malformed entityId)")

	}
	// check account and send reply
	var response api.APIresponse
	entityIdAddr := common.BytesToAddress(request.EntityId)
	acc, err := r.vocapp.State.GetAccount(entityIdAddr, true)
	if err != nil {
		return nil, fmt.Errorf("cannot get account: %w", err)
	}
	// account does not exist
	if acc == nil {
		// response ok meaning the account does not exist
		return &response, nil
	}
	response.Balance = new(uint64)
	*response.Balance = acc.Balance
	response.Nonce = new(uint32)
	*response.Nonce = acc.Nonce
	response.InfoURI = acc.InfoURI
	if len(acc.DelegateAddrs) > 0 {
		for _, v := range acc.DelegateAddrs {
			response.Delegates = append(response.Delegates, common.BytesToAddress(v).String())
		}
	}
	return &response, nil
}

func (r *RPCAPI) getTreasurer(request *api.APIrequest) (*api.APIresponse, error) {
	// try get treasurer and send reply
	var response api.APIresponse
	t, err := r.vocapp.State.Treasurer(true)
	if err != nil {
		return nil, fmt.Errorf("cannot get treasurer: %w", err)
	}
	// treasurer does not exist
	if t == nil {
		// response ok meaning the treasurer does not exist
		return &response, nil
	}
	response.EntityID = common.BytesToAddress(t.Address).String()
	response.Nonce = new(uint32)
	*response.Nonce = t.Nonce
	return &response, nil
}
