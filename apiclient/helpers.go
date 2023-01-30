package apiclient

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/api/faucet"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const pollInterval = 4 * time.Second

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

// WaitUntilHeight waits until the given height is reached and returns nil.
//
// If ctx.Done() is reached, returns ctx.Err() instead.
func (c *HTTPclient) WaitUntilHeight(ctx context.Context, height uint32) error {
	for {
		info, err := c.ChainInfo()
		if err != nil {
			log.Warn(err)
		}
		if *info.Height >= height {
			return nil
		}
		select {
		case <-time.After(pollInterval):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *HTTPclient) WaitUntilElectionCreated(ctx context.Context,
	electionID types.HexBytes) (*api.Election, error) {
	return c.WaitUntilElectionStatus(ctx, electionID, "READY")
}

// WaitUntilElectionStarts waits until the given election starts.
func (c *HTTPclient) WaitUntilElectionStarts(ctx context.Context,
	electionID types.HexBytes) (*api.Election, error) {
	election, err := c.WaitUntilElectionCreated(ctx, electionID)
	if err != nil {
		return nil, err
	}
	startHeight, err := c.DateToHeight(election.StartDate)
	if err != nil {
		return nil, err
	}
	// api.DateToHeight() is not exact, wait for 1 additional block
	return election, c.WaitUntilHeight(ctx, startHeight+1)
}

// WaitUntilElectionStatus waits until the given election has the given status.
func (c *HTTPclient) WaitUntilElectionStatus(ctx context.Context,
	electionID types.HexBytes, status string) (*api.Election, error) {
	log.Infof("waiting for election %s to reach status %s", electionID.String(), status)
	for {
		election, err := c.Election(electionID)
		if err != nil {
			if !strings.Contains(err.Error(), "No data found for this key") {
				return nil, err
			}
		}
		if election != nil && election.Status == status {
			return election, nil
		}
		select {
		case <-time.After(pollInterval):
			continue
		case <-ctx.Done():
			return nil, fmt.Errorf("election %v never reached status %s: %w", electionID, status, ctx.Err())
		}
	}
}

// WaitUntilTxIsMined waits until the given transaction is mined (included in a block)
func (c *HTTPclient) WaitUntilTxIsMined(ctx context.Context,
	txHash types.HexBytes) (*api.TransactionReference, error) {
	for {
		tr, err := c.TransactionReference(txHash)
		if err == nil {
			return tr, nil
		}
		select {
		case <-time.After(pollInterval):
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// GetFaucetPackageFromDefaultDevService returns a faucet package.
// Needs just the destination wallet address, the URL and bearer token are hardcoded
func GetFaucetPackageFromDevService(account string) (*models.FaucetPackage, error) {
	return GetFaucetPackageFromRemoteService(
		DefaultDevelopmentFaucetURL+account,
		DefaultDevelopmentFaucetToken,
	)
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

// BabyJubJubPrivKey returns a private BabyJubJub key generated based on the
// provided account ethereum.SignKeys.
func BabyJubJubPrivKey(account *ethereum.SignKeys) (babyjub.PrivateKey, error) {
	privKey := babyjub.PrivateKey{}
	_, strPrivKey := account.HexString()
	if _, err := hex.Decode(privKey[:], []byte(strPrivKey)); err != nil {
		return babyjub.PrivateKey{}, fmt.Errorf("error generating babyjub key: %w", err)
	}

	return privKey, nil
}

// BabyJubJubPubKey returns the public key associated to the provided
// BabuJubJub private key encoded to slice of bytes arbo tree ready.
func BabyJubJubPubKey(privKey babyjub.PrivateKey) (types.HexBytes, error) {
	pubKey, err := poseidon.Hash([]*big.Int{
		privKey.Public().X,
		privKey.Public().Y,
	})
	if err != nil {
		return nil, fmt.Errorf("error hashing babyjub public key: %w", err)
	}

	return arbo.BigIntToBytes(arbo.HashFunctionPoseidon.Len(), pubKey), nil
}
