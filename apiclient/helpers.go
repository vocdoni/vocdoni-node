package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/api/faucet"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	WaitTimeout  = config.DefaultMinerTargetBlockTime * 3
	PollInterval = config.DefaultMinerTargetBlockTime / 2
)

func (c *HTTPclient) DateToHeight(date time.Time) (uint32, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "dateToBlock", fmt.Sprintf("%d", date.Unix()))
	if err != nil {
		return 0, err
	}
	if code != apirest.HTTPstatusOK {
		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	var h struct {
		Height uint32 `json:"height"`
	}
	if err := json.Unmarshal(resp, &h); err != nil {
		return 0, err
	}
	return h.Height, nil
}

// SignAndSendTx signs the given transaction and sends it to the blockchain.
// It returns the transaction hash and the blockchain response (if any).
// Takes a protobuf marshaled transaction as input of type models.Tx
func (c *HTTPclient) SignAndSendTx(marshaledTx []byte) (types.HexBytes, []byte, error) {
	// Sign the transaction
	sitnature, err := c.account.SignVocdoniTx(marshaledTx, c.ChainID())
	if err != nil {
		return nil, nil, err
	}
	// Build the signed transaction
	stx, err := proto.Marshal(
		&models.SignedTx{
			Tx:        marshaledTx,
			Signature: sitnature,
		})
	if err != nil {
		return nil, nil, err
	}
	// Send the signed transaction and fetch the response
	tx := &api.Transaction{Payload: stx}
	resp, code, err := c.Request(HTTPPOST, tx, "chain", "transactions")
	if err != nil {
		return nil, nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	if err := json.Unmarshal(resp, tx); err != nil {
		return nil, nil, fmt.Errorf("could not decode response: %w", err)
	}
	return tx.Hash, tx.Response, nil
}

// SendTx sends a transaction to the blockchain.
// It returns the transaction hash and the blockchain response (if any).
// Takes a protobuf marshaled transaction as input of type models.SignedTx
func (c *HTTPclient) SendTx(marshaledSignedTx []byte) (types.HexBytes, []byte, error) {
	// Send the signed transaction and fetch the response
	tx := &api.Transaction{Payload: marshaledSignedTx}
	resp, code, err := c.Request(HTTPPOST, tx, "chain", "transactions")
	if err != nil {
		return nil, nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	if err := json.Unmarshal(resp, tx); err != nil {
		return nil, nil, fmt.Errorf("could not decode response: %w", err)
	}
	return tx.Hash, tx.Response, nil
}

// WaitUntilNextBlock waits until next block, and returns nil
//
// It uses a context.WithTimeout(24s) before giving up and returning ctx.Err()
func (c *HTTPclient) WaitUntilNextBlock() error {
	var cancel context.CancelFunc
	ctx, cancel := context.WithTimeout(context.Background(), WaitTimeout)
	defer cancel()
	return c.WaitUntilNBlocks(ctx, 1)
}

// WaitUntilNBlocks waits until N blocks are produced, and returns nil.
//
// If ctx.Done() is reached, returns ctx.Err() instead.
func (c *HTTPclient) WaitUntilNBlocks(ctx context.Context, n uint32) error {
	for {
		info, err := c.ChainInfo()
		if err != nil {
			log.Errorw(err, "ChainInfo failed, will retry")
			time.Sleep(PollInterval)
			continue
		}
		return c.WaitUntilHeight(ctx, info.Height+n)
	}
}

// WaitUntilHeight waits until the given height is reached and returns nil.
//
// If ctx.Done() is reached, returns ctx.Err() instead.
func (c *HTTPclient) WaitUntilHeight(ctx context.Context, height uint32) error {
	for {
		info, err := c.ChainInfo()
		if err != nil {
			log.Errorw(err, "ChainInfo failed, will retry")
			time.Sleep(PollInterval)
			continue
		}
		if info.Height >= height {
			return nil
		}
		select {
		case <-time.After(PollInterval):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WaitUntilElectionCreated waits until the given election is created.
func (c *HTTPclient) WaitUntilElectionCreated(ctx context.Context, electionID types.HexBytes) (*api.Election, error) {
	return c.WaitUntilElectionStatus(ctx, electionID, "READY")
}

// WaitUntilElectionStarts waits until the given election starts.
func (c *HTTPclient) WaitUntilElectionStarts(ctx context.Context, electionID types.HexBytes) (*api.Election, error) {
	election, err := c.WaitUntilElectionCreated(ctx, electionID)
	if err != nil {
		return nil, err
	}
	// wait until blockchain timestamp is after election start date
	log.Infow("waiting for election start", "election", electionID.String(), "start", election.StartDate.String())
	for {
		time.Sleep(5 * time.Second)
		info, err := c.ChainInfo()
		if err != nil {
			return nil, fmt.Errorf("chainInfo failed on wait until election created: %w", err)
		}
		if time.Unix(int64(info.Timestamp), 0).After(election.StartDate) {
			break
		}
	}
	return election, nil
}

// WaitUntilElectionStatus waits until the given election has the given status.
func (c *HTTPclient) WaitUntilElectionStatus(ctx context.Context, electionID types.HexBytes, status string) (*api.Election, error) {
	log.Infow("waiting for election status", "election", electionID.String(), "status", status)
	startTime := time.Now()
	for {
		election, err := c.Election(electionID)
		if err != nil {
			// Return an error if the received error is not a '404 - Not found'
			// error which means that the election has not yet been created.
			if !strings.Contains(err.Error(), "API error: 404") {
				return nil, err
			}
		}
		if election != nil && election.Status == status {
			log.Infow("election reached status", "election", electionID.String(),
				"status", status, "duration", time.Since(startTime).String())
			return election, nil
		}
		select {
		case <-time.After(PollInterval):
			continue
		case <-ctx.Done():
			return nil, fmt.Errorf("election %x never reached status %s after %s: %w",
				electionID, status, time.Since(startTime).String(), ctx.Err())
		}
	}
}

// WaitUntilElectionResults waits until the given election has published final results.
func (c *HTTPclient) WaitUntilElectionResults(ctx context.Context, electionID types.HexBytes) (*api.ElectionResults, error) {
	log.Infof("waiting for election %s to publish final results", electionID.String())
	startTime := time.Now()
	for {
		election, err := c.ElectionResults(electionID)
		if err != nil && !strings.Contains(err.Error(), "5024") { // TODO: proper code matching
			return nil, err
		}
		if election != nil {
			log.Infow("election published results", "election",
				electionID.String(), "duration", time.Since(startTime).String())
			return election, nil
		}
		select {
		case <-time.After(PollInterval):
			continue
		case <-ctx.Done():
			return nil, fmt.Errorf("election %s never published resuls after %s: %w",
				electionID.String(), time.Since(startTime).String(), ctx.Err())
		}
	}
}

// WaitUntilTxIsMined waits until the given transaction is mined (included in a block)
func (c *HTTPclient) WaitUntilTxIsMined(ctx context.Context, txHash types.HexBytes) (*api.TransactionReference, error) {
	startTime := time.Now()
	for {
		tr, err := c.TransactionReference(txHash)
		if err == nil {
			time.Sleep(PollInterval / 2) // wait a bit longer to make sure the tx is committed
			log.Infow("transaction mined", "tx",
				txHash.String(), "duration", time.Since(startTime).String())
			return tr, nil
		}
		select {
		case <-time.After(PollInterval):
			continue
		case <-ctx.Done():
			return nil, fmt.Errorf("transaction %s never mined after %s: %w",
				txHash.String(), time.Since(startTime).String(), ctx.Err())
		}
	}
}

// WaitUntilElectionKeys waits until the election has published its encryption keys,
// and returns them.
func (c *HTTPclient) WaitUntilElectionKeys(ctx context.Context, electionID types.HexBytes) (*api.ElectionKeys, error) {
	log.Debugf("fetching election keys for %x", electionID)
	startTime := time.Now()
	for {
		ek, err := c.ElectionKeys(electionID)
		if err == nil {
			log.Infow("election keys published", "election", electionID.String(),
				"duration", time.Since(startTime).String())
			return ek, nil
		}
		select {
		case <-time.After(PollInterval):
			continue
		case <-ctx.Done():
			return nil, fmt.Errorf("election %s keys not yet published after %s: %w",
				electionID, time.Since(startTime).String(), ctx.Err())
		}
	}
}

// EncryptionKeys returns the encryption keys for the given election.
func (c *HTTPclient) EncryptionKeys(electionID types.HexBytes) ([]api.Key, error) {
	keysEnc := []api.Key{}

	ctx, cancel := context.WithTimeout(context.Background(), WaitTimeout)
	defer cancel()
	ek, err := c.WaitUntilElectionKeys(ctx, electionID)
	if err != nil {
		return nil, err
	}

	for _, k := range ek.PublicKeys {
		if len(k.Key) > 0 {
			keysEnc = append(keysEnc, api.Key{
				Key:   k.Key,
				Index: k.Index,
			})
		}
	}
	return keysEnc, nil
}

// GetFaucetPackageFromDefaultService returns a faucet package.
// Needs the destination wallet address and the chainID.
func GetFaucetPackageFromDefaultService(account string, chainID string) (*models.FaucetPackage, error) {
	faucetChainID := chainID
	chainIDsplit := strings.Split(chainID, "/")
	if len(chainIDsplit) > 1 {
		faucetChainID = strings.ToLower(chainIDsplit[1])
	}
	baseURL, ok := DefaultFaucetURLs[faucetChainID]
	if !ok {
		return nil, fmt.Errorf("no default faucet URL for network %s", chainID)
	}
	url, err := util.BuildURL(baseURL, account)
	if err != nil {
		return nil, err
	}
	log.Infow("requesting faucet package", "url", url)
	return GetFaucetPackageFromRemoteService(
		url,
		"",
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
	header := http.Header{
		"User-Agent": []string{"Vocdoni API client / 1.0"},
	}
	if token != "" {
		header.Add("Authorization", "Bearer "+token)
	}
	c := http.Client{}
	resp, err := c.Do(&http.Request{
		Method: HTTPGET,
		URL:    u,
		Header: header,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != apirest.HTTPstatusOK {
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
