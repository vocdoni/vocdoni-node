package rpcapi

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	api "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
)

func (r *RPCAPI) getAccount(request *api.APIrequest) (*api.APIresponse, error) {
	// check entity id
	if len(request.EntityId) != types.EntityIDsize {
		return nil, fmt.Errorf("cannot get account info: (malformed entityId)")

	}
	// check account and send reply
	entityIdAddr := common.BytesToAddress(request.EntityId)
	acc, err := r.vocapp.State.GetAccount(entityIdAddr, true)
	if err != nil {
		return nil, fmt.Errorf("cannot get account: %w", err)
	}
	// account does not exist
	if acc == nil {
		return nil, vochain.ErrAccountNotExist
	}
	response := api.APIresponse{
		InfoURI:      acc.InfoURI,
		Balance:      &acc.Balance,
		Nonce:        &acc.Nonce,
		ProcessNonce: &acc.ProcessIndex,
	}
	if len(acc.DelegateAddrs) > 0 {
		for _, v := range acc.DelegateAddrs {
			response.Delegates = append(response.Delegates, common.BytesToAddress(v).String())
		}
	}
	return &response, nil
}

func (r *RPCAPI) getTreasurer(request *api.APIrequest) (*api.APIresponse, error) {
	// try get treasurer and send reply
	t, err := r.vocapp.State.Treasurer(true)
	if err != nil {
		return nil, fmt.Errorf("cannot get treasurer: %w", err)
	}
	// treasurer does not exist
	if t == nil {
		return nil, vochain.ErrAccountNotExist
	}
	response := api.APIresponse{EntityID: common.BytesToAddress(t.Address).String()}
	response.Nonce = new(uint32)
	*response.Nonce = t.Nonce
	return &response, nil
}

func (r *RPCAPI) getTransactionCost(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	// tx valid tx type
	txType := vochain.TxCostNameToTxType(request.Type)
	if txType == models.TxType_TX_UNKNOWN {
		return nil, fmt.Errorf("invalid tx type: %s", request.Type)
	}
	c, err := r.vocapp.State.TxCost(txType, true)
	if err != nil {
		return nil, fmt.Errorf("cannot get tx cost: %w", err)
	}
	response.Amount = new(uint64)
	*response.Amount = c
	return &response, nil
}
