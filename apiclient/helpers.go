package apiclient

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/iden3/go-iden3-crypto/poseidon"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/api/faucet"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	DefaultSIKContent    = "vocdoni"
	DefaultBlockInterval = 8 * time.Second
	WaitTimeout          = 3 * DefaultBlockInterval
	PollInterval         = DefaultBlockInterval / 2
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
func (c *HTTPclient) WaitUntilElectionResults(ctx context.Context,
	electionID types.HexBytes) (*api.ElectionResults, error) {
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
func (c *HTTPclient) WaitUntilTxIsMined(ctx context.Context,
	txHash types.HexBytes) (*api.TransactionReference, error) {
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
func (c *HTTPclient) WaitUntilElectionKeys(ctx context.Context, electionID types.HexBytes) (
	*api.ElectionKeys, error) {
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

// GetFaucetPackageFromDevService returns a faucet package.
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

// GenerateSik function generates the Secret Identity Key for the current client
// account with the signature and the secret provided following the definition:
//
//	SIK = poseidon(address, signature, secret*)
//
// *The secret is optional.
func (c *HTTPclient) GenerateSik(sign, secret []byte) ([]byte, error) {
	if sign == nil {
		return nil, fmt.Errorf("signature not provided")
	}
	seed := []*big.Int{
		zk.BigToFF(c.MyAddress().Big()),
		zk.BigToFF(new(big.Int).SetBytes(sign)),
	}
	if secret != nil {
		seed = append(seed, arbo.BytesToBigInt(secret))
	}
	hash, err := poseidon.Hash(seed)
	if err != nil {
		return nil, err
	}
	return arbo.BigIntToBytes(arbo.HashFunctionPoseidon.Len(), hash), nil
}

// Nullifier returns snark friendly nullifier based on the provided signature,
// the user secret and electionId. The nullifier is calculated following its
// definition:
//
//	nullifier = poseidon(signature, secret*, sha256(electionId))
//
// *The secret is optional.
func (c *HTTPclient) CalcNullifier(sign, secret, electionId []byte) (*big.Int, error) {
	// Encode the electionId -> sha256(electionId)
	hashedElectionId := sha256.Sum256(electionId)
	intElectionId := []*big.Int{
		new(big.Int).SetBytes(arbo.SwapEndianness(hashedElectionId[:16])),
		new(big.Int).SetBytes(arbo.SwapEndianness(hashedElectionId[16:])),
	}
	seed := []*big.Int{arbo.BytesToBigInt(sign)}
	if secret != nil {
		seed = append(seed, arbo.BytesToBigInt(secret))
	}
	return poseidon.Hash(append(seed, intElectionId...))
}
