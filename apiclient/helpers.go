package apiclient

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// ChainInfo returns some information about the chain, such as block height.
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
	resp, code, err := c.Request(HTTPGET, nil, "chain", "dateToBlock", fmt.Sprintf("%d", date.Unix()))
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
	resp, code, err := c.Request(HTTPPOST, tx, "chain", "transactions")
	if err != nil {
		return nil, nil, err
	}
	if code != 200 {
		return nil, nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	if err := json.Unmarshal(resp, tx); err != nil {
		return nil, nil, fmt.Errorf("could not decode response: %w", err)
	}
	return tx.Hash, tx.Response, nil
}

// TODO: add contexts to all the Wait* methods

// WaitUntilHeight waits until the given height is reached.
func (c *HTTPclient) WaitUntilHeight(height uint32) {
	for {
		info, err := c.ChainInfo()
		if err != nil {
			log.Warn(err)
		} else {
			if *info.Height > height {
				break
			}
		}
		time.Sleep(time.Second * 4)
	}
}

// WaitUntilElectionStarts waits until the given election starts.
func (c *HTTPclient) WaitUntilElectionStarts(electionID types.HexBytes) error {
	election, err := c.Election(electionID)
	if err != nil {
		return err
	}
	startHeight, err := c.DateToHeight(election.StartDate)
	if err != nil {
		return err
	}
	c.WaitUntilHeight(startHeight + 1) // add a block to be sure
	return nil
}

// WaitUntilElectionStatus waits until the given election has the given status.
// If the status is not reached after 10 minutes, it returns os.ErrDeadlineExceeded.
func (c *HTTPclient) WaitUntilElectionStatus(electionID types.HexBytes, status string) error {
	for startTime := time.Now(); time.Since(startTime) < time.Second*300; {
		election, err := c.Election(electionID)
		if err != nil {
			log.Fatal(err)
		}
		if election.Status == status {
			return nil
		}
		time.Sleep(time.Second * 5)
	}
	return os.ErrDeadlineExceeded
}

// EnsureTxIsMined waits until the given transaction is mined. If the transaction is not mined
// after 3 minutes, it returns os.ErrDeadlineExceeded.
func (c *HTTPclient) EnsureTxIsMined(txHash types.HexBytes) error {
	for startTime := time.Now(); time.Since(startTime) < 180*time.Second; {
		_, err := c.TransactionReference(txHash)
		if err == nil {
			return nil
		}
		time.Sleep(4 * time.Second)
	}
	return os.ErrDeadlineExceeded
}
