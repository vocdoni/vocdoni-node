package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/api/faucet"
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

// GetFaucetPackageFromRemoteService returns a faucet package from a remote HTTP faucet service.
// This service usually requires a valid bearer token.
// faucetURL usually includes the destination wallet address that will receive the funds.
func GetFaucetPackageFromRemoteService(faucetURL, token string) (*models.FaucetPackage, error) {
	u, err := url.Parse(faucetURL)
	if err != nil {
		return nil, err
	}
	c := http.Client{}
	resp, err := c.Do(&http.Request{
		Method: HTTPGET,
		URL:    u,
		Header: http.Header{
			"Authorization": []string{"Bearer " + token},
			"User-Agent":    []string{"Vocdoni API client / 1.0"},
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("faucet request failed: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fresp := faucet.FaucetResponse{}
	if err := json.Unmarshal(data, &fresp); err != nil {
		return nil, err
	}
	if fresp.Amount == "" {
		return nil, fmt.Errorf("faucet response is missing amount")
	}
	if fresp.FaucetPackage == nil {
		return nil, fmt.Errorf("faucet response is missing package")
	}
	return UnmarshalFaucetPackage(fresp.FaucetPackage)
}

// UnmarshalFaucetPackage unmarshals a faucet package into a FaucetPackage struct.
func UnmarshalFaucetPackage(data []byte) (*models.FaucetPackage, error) {
	fpackage := faucet.FaucetPackage{}
	if err := json.Unmarshal(data, &fpackage); err != nil {
		return nil, err
	}
	return &models.FaucetPackage{
		Payload:   fpackage.FaucetPayload,
		Signature: fpackage.Signature,
	}, nil

}
