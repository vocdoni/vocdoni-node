package rpcapi

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	api "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/types"
)

func (r *RPCAPI) getAccount(request *api.APIrequest) (*api.APIresponse, error) {
	// check pid
	if len(request.EntityId) != types.EntityIDsize {
		return nil, fmt.Errorf("cannot get account info: (malformed entityId)")

	}
	// Check account and send reply
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
	delegates := make([]string, len(acc.DelegateAddrs))
	for i := 0; i < len(acc.DelegateAddrs); i++ {
		delegates = append(delegates, common.BytesToAddress(acc.DelegateAddrs[i]).String())
	}
	response.Delegates = delegates
	return &response, nil
}
