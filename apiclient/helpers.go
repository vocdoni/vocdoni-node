package apiclient

import (
	"encoding/json"
	"fmt"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func (c *HTTPclient) ChainInfo() (*api.ChainInfo, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "info")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	info := &api.ChainInfo{}
	err = json.Unmarshal(resp, info)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (c *HTTPclient) DateToHeight(date time.Time) (uint32, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "blockdate", fmt.Sprintf("%d", date.Unix()))
	if err != nil {
		return 0, err
	}
	if code != 200 {
		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	var h struct {
		Height uint32 `json:"height"`
	}
	err = json.Unmarshal(resp, &h)
	if err != nil {
		return 0, err
	}
	return h.Height, nil
}

func (c *HTTPclient) SignAndSendTx(stx *models.SignedTx) (types.HexBytes, []byte, error) {
	var err error
	if stx.Signature, err = c.account.SignVocdoniTx(stx.Tx, c.ChainID()); err != nil {
		return nil, nil, err
	}
	txData, err := proto.Marshal(stx)
	if err != nil {
		return nil, nil, err
	}

	tx := &api.Transaction{Payload: txData}
	resp, code, err := c.Request(HTTPPOST, tx, "chain", "transaction", "submit")
	if err != nil {
		return nil, nil, err
	}
	if code != 200 {
		return nil, nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}

	return tx.Hash, tx.Response, nil
}
