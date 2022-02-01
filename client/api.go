package client

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/artifacts"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

type pkeys struct {
	pub  []api.Key
	priv []api.Key
}

// GetChainID returns the chainID of the Vochain
func (c *Client) GetChainID() (string, error) {
	var req api.APIrequest
	req.Method = "getInfo"
	resp, err := c.Request(req, nil)
	if err != nil {
		return "", err
	}
	if !resp.Ok {
		return "", fmt.Errorf("cannot get chain ID: %v", resp.Message)
	}
	return resp.ChainID, nil
}

// GetProcessInfo returns useful process information and parameters
func (c *Client) GetProcessInfo(pid []byte) (*indexertypes.Process, error) {
	var req api.APIrequest
	req.Method = "getProcessInfo"
	req.ProcessID = pid
	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Ok || resp.Process == nil {
		return nil, fmt.Errorf("cannot getProcessInfo: %v", resp.Message)
	}
	return resp.Process, nil
}

// GetEnvelopeStatus returns true if the vote envelope is registered
func (c *Client) GetEnvelopeStatus(nullifier, pid []byte) (bool, error) {
	var req api.APIrequest
	req.Method = "getEnvelopeStatus"
	req.Nullifier = nullifier
	resp, err := c.Request(req, nil)
	if err != nil {
		return false, err
	}
	if !resp.Ok || resp.Registered == nil {
		return false, fmt.Errorf("cannot check envelope (%s)", resp.Message)
	}
	return *resp.Registered, nil
}

// GetProof tries to generate a return a census proof
func (c *Client) GetProof(pubkey, root []byte, digested bool) ([]byte, []byte, error) {
	var req api.APIrequest
	req.Method = "genProof"
	req.CensusID = hex.EncodeToString(root)
	req.Digested = digested
	req.CensusKey = pubkey

	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, nil, err
	}
	if len(resp.Siblings) == 0 || !resp.Ok {
		return nil, nil, fmt.Errorf("cannot get merkle proof: (%s)", resp.Message)
	}

	return resp.Siblings, resp.CensusValue, nil
}

// GetResults gets the results of a given process from the scrutinizer
func (c *Client) GetResults(pid []byte) ([][]string, string, bool, error) {
	var req api.APIrequest
	req.Method = "getResults"
	req.ProcessID = pid
	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, "", false, err
	}
	if !resp.Ok {
		return nil, "", false, fmt.Errorf("cannot get results: (%s)", resp.Message)
	}
	if resp.Message == "no results yet" {
		return nil, resp.State, false, nil
	}
	return resp.Results, resp.State, *resp.Final, nil
}

// GetEnvelopeHeight returns the height at which the envelope was registered
func (c *Client) GetEnvelopeHeight(pid []byte) (uint32, error) {
	var req api.APIrequest
	req.Method = "getEnvelopeHeight"
	req.ProcessID = pid
	resp, err := c.Request(req, nil)
	if err != nil {
		return 0, err
	}
	if !resp.Ok {
		return 0, fmt.Errorf(resp.Message)
	}
	return *resp.Height, nil
}

// CensusSize returns the size of a census as int64
func (c *Client) CensusSize(cid []byte) (int64, error) {
	var req api.APIrequest
	req.Method = "getSize"
	req.CensusID = hex.EncodeToString(cid)
	resp, err := c.Request(req, nil)
	if err != nil {
		return 0, err
	}
	if !resp.Ok {
		return 0, fmt.Errorf(resp.Message)
	}
	return *resp.Size, nil
}

// ImportCensus adds, imports and publish a given census
func (c *Client) ImportCensus(signer *ethereum.SignKeys, uri string) ([]byte, error) {
	var req api.APIrequest
	req.Method = "addCensus"
	req.CensusID = RandomHex(16)
	resp, err := c.Request(req, signer)
	if err != nil {
		return nil, err
	}

	req.Method = "importRemote"
	req.CensusID = resp.CensusID
	req.URI = uri
	resp, err = c.Request(req, signer)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf(resp.Message)
	}

	req.Method = "publish"
	req.URI = ""
	resp, err = c.Request(req, signer)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf(resp.Message)
	}
	return resp.Root, nil
}

// GetKeys gets the process encryption keys
func (c *Client) GetKeys(pid, eid []byte) (*pkeys, error) {
	var req api.APIrequest
	req.Method = "getProcessKeys"
	req.ProcessID = pid
	req.EntityId = eid
	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("cannot get keys for process %x: %s", pid, resp.Message)
	}
	return &pkeys{
		pub:  resp.EncryptionPublicKeys,
		priv: resp.EncryptionPrivKeys,
	}, nil
}

// GetCircuitConfig gets the ZK circuit config used for anonymous voting
func (c *Client) GetCircuitConfig(pid []byte) (*int, *artifacts.CircuitConfig, error) {
	var req api.APIrequest
	req.Method = "getProcessCircuitConfig"
	req.ProcessID = pid
	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, nil, err
	}
	if !resp.Ok {
		return nil, nil, fmt.Errorf("cannot get circuitConfig for process %x: %s", pid, resp.Message)
	}
	return resp.CircuitIndex, resp.CircuitConfig, nil
}

// GetRollingCensusSize returns the size of a given rolling census
func (c *Client) GetRollingCensusSize(pid []byte) (int64, error) {
	var req api.APIrequest
	req.Method = "getProcessRollingCensusSize"
	req.ProcessID = pid
	resp, err := c.Request(req, nil)
	if err != nil {
		return 0, err
	}
	if !resp.Ok {
		return 0, fmt.Errorf("cannot get RollingCensusSize for process %x: %s", pid, resp.Message)
	}
	return *resp.Size, nil
}

// GetRollingCensusVoterWeight returns the weight in the census of a specific address
func (c *Client) GetRollingCensusVoterWeight(pid []byte, address common.Address) (*big.Int, error) {
	var req api.APIrequest
	req.Method = "getPreregisterVoterWeight"
	req.ProcessID = pid
	req.VoterAddress = address.Bytes()
	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("cannot get pre register voter weight for process %x: %s", pid, resp.Message)
	}
	return resp.Weight.ToInt(), nil
}

// TestResults checks the correctness of a given process results
func (c *Client) TestResults(pid []byte, totalVotes int, withWeight uint64) ([][]string, error) {
	log.Infof("waiting for results...")
	var err error
	var results [][]string
	var block uint32
	var final bool
	for {
		block, err = c.GetCurrentBlock()
		if err != nil {
			return nil, err
		}
		c.WaitUntilBlock(block + 1)
		results, _, final, err = c.GetResults(pid)
		if err != nil {
			return nil, err
		}
		if final {
			break
		}
		log.Infof("no results yet at block %d", block+2)
	}
	total := fmt.Sprintf("%d", uint64(totalVotes)*withWeight)
	if results[0][1] != total ||
		results[1][2] != total ||
		results[2][3] != total ||
		results[3][4] != total {
		return nil, fmt.Errorf("invalid results: %v", results)
	}
	return results, nil
}

// Proof wraps a merkle proof siblings and value
type Proof struct {
	Siblings []byte
	Value    []byte
}

// GetMerkleProofBatch returns an array of merkle proofs for a set of given addresses
func (c *Client) GetMerkleProofBatch(signers []*ethereum.SignKeys,
	root []byte,
	tolerateError bool) ([]*Proof, error) {

	var proofs []*Proof
	// Generate merkle proofs
	log.Infof("generating proofs...")
	for i, s := range signers {
		siblings, value, err := c.GetProof(s.PublicKey(), root, false)
		if err != nil {
			if tolerateError {
				continue
			}
			return proofs, err
		}
		proofs = append(proofs, &Proof{Siblings: siblings, Value: value})
		if (i+1)%100 == 0 {
			log.Infof("proof generation progress for %s: %d%%", c.Addr, ((i+1)*100)/(len(signers)))
		}
	}
	return proofs, nil
}

// GetMerkleProofPoseidonBatch returns an array of poseidon merkle proofs for a set of given addresses
func (c *Client) GetMerkleProofPoseidonBatch(signers []*ethereum.SignKeys,
	root []byte,
	tolerateError bool) ([]*Proof, error) {

	var proofs []*Proof
	// Generate merkle proofs
	log.Infof("generating poseidon proofs...")
	for i, s := range signers {
		zkCensusKey, _ := testGetZKCensusKey(s)
		siblings, value, err := c.GetProof(zkCensusKey, root, true)
		if err != nil {
			if tolerateError {
				continue
			}
			return proofs, err
		}
		proofs = append(proofs, &Proof{Siblings: siblings, Value: value})
		if (i+1)%100 == 0 {
			log.Infof("proof generation progress for %s: %d%%", c.Addr, ((i+1)*100)/(len(signers)))
		}
	}
	return proofs, nil
}

// GetCSPproofBatch returns an array of CSP like merkle proofs for a set of given addresses
func (c *Client) GetCSPproofBatch(signers []*ethereum.SignKeys,
	ca *ethereum.SignKeys,
	pid []byte) ([]*Proof, error) {

	var proofs []*Proof
	// Generate merkle proofs
	log.Infof("generating proofs...")
	for i, k := range signers {
		bundle := &models.CAbundle{
			ProcessId: pid,
			Address:   k.Address().Bytes(),
		}
		bundleBytes, err := proto.Marshal(bundle)
		if err != nil {
			log.Fatal(err)
		}
		signature, err := ca.SignEthereum(bundleBytes)
		if err != nil {
			log.Fatal(err)
		}

		caProof := &models.ProofCA{
			Bundle:    bundle,
			Type:      models.ProofCA_ECDSA,
			Signature: signature,
		}
		caProofBytes, err := proto.Marshal(caProof)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, &Proof{Siblings: caProofBytes})
		if (i+1)%100 == 0 {
			log.Infof("proof generation progress for %s: %d%%", c.Addr, ((i+1)*100)/(len(signers)))
		}
	}
	return proofs, nil
}

// CreateOrSetAccount creates or sets the infoURI of a Vochain account
func (c *Client) CreateOrSetAccount(signer *ethereum.SignKeys, to common.Address, infoURI string, nonce uint32) error {
	var req api.APIrequest
	var err error
	req.Method = "submitRawTx"

	tx := &models.SetAccountInfoTx{
		Txtype:  models.TxType_SET_ACCOUNT_INFO,
		Nonce:   nonce,
		InfoURI: infoURI,
		Account: to.Bytes(),
	}

	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccountInfo{SetAccountInfo: tx}})
	if err != nil {
		return err
	}
	if stx.Signature, err = signer.SignVocdoniTx(stx.Tx); err != nil {
		return err
	}
	if req.Payload, err = proto.Marshal(stx); err != nil {
		return err
	}

	resp, err := c.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	return nil
}

// GetAccount returns information of a given account
func (c *Client) GetAccount(signer *ethereum.SignKeys, accountAddr common.Address) (*vochain.Account, error) {
	req := api.APIrequest{Method: "getAccount", EntityId: accountAddr.Bytes()}
	resp, err := c.Request(req, signer)
	acc := &vochain.Account{}
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("could not get account: %s", resp.Message)
	}
	if resp.Balance != nil {
		acc.Balance = *resp.Balance
	}
	if resp.Nonce != nil {
		acc.Nonce = *resp.Nonce
	}
	if resp.InfoURI != "" {
		acc.InfoURI = resp.InfoURI
	}
	if len(resp.Delegates) > 0 {
		for _, v := range resp.Delegates {
			acc.DelegateAddrs = append(acc.DelegateAddrs, common.HexToAddress(v).Bytes())
		}
	}
	return acc, nil
}

// GetTreasurer returns information about the treasurer
func (c *Client) GetTreasurer(signer *ethereum.SignKeys) (*models.Treasurer, error) {
	req := api.APIrequest{Method: "getTreasurer"}
	resp, err := c.Request(req, signer)
	treasurer := &models.Treasurer{}
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("could not get treasurer: %s", resp.Message)
	}
	if resp.EntityID != "" {
		treasurer.Address = common.HexToAddress(resp.EntityID).Bytes()
	}
	if resp.Nonce != nil {
		treasurer.Nonce = *resp.Nonce
	}
	return treasurer, nil
}

// del = true -> delete operation
// SetAccountDelegate adds or remove a given account delegate
func (c *Client) SetAccountDelegate(signer *ethereum.SignKeys, delegate common.Address, nonce uint32, del bool) error {
	var req api.APIrequest
	var err error
	req.Method = "submitRawTx"

	tx := &models.SetAccountDelegateTx{
		Txtype:   models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
		Nonce:    nonce,
		Delegate: delegate.Bytes(),
	}

	if del {
		tx.Txtype = models.TxType_DEL_DELEGATE_FOR_ACCOUNT
	}

	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccountDelegateTx{SetAccountDelegateTx: tx}})
	if err != nil {
		return err
	}
	if stx.Signature, err = signer.SignVocdoniTx(stx.Tx); err != nil {
		return err
	}
	if req.Payload, err = proto.Marshal(stx); err != nil {
		return err
	}

	resp, err := c.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	return nil
}

// SendTokens sends tokens to a given address
func (c *Client) SendTokens(from *ethereum.SignKeys, to common.Address, value uint64, nonce uint32) error {
	var req api.APIrequest
	var err error
	req.Method = "submitRawTx"

	tx := &models.SendTokensTx{
		Txtype: models.TxType_SEND_TOKENS,
		Nonce:  nonce,
		From:   from.Address().Bytes(),
		To:     to.Bytes(),
		Value:  value,
	}

	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SendTokens{SendTokens: tx}})
	if err != nil {
		return err
	}
	if stx.Signature, err = from.SignVocdoniTx(stx.Tx); err != nil {
		return err
	}
	if req.Payload, err = proto.Marshal(stx); err != nil {
		return err
	}

	resp, err := c.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	return nil
}

// GenerateFaucetPackage generates a faucet package
func (c *Client) GenerateFaucetPackage(from *ethereum.SignKeys, to common.Address, value uint64) (*models.FaucetPackage, error) {
	rand.Seed(time.Now().UnixNano())
	payload := &models.FaucetPayload{
		Identifier: rand.Uint64(),
		To:         to.Bytes(),
		Amount:     value,
	}
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return nil, err
	}
	payloadSignature, err := from.SignEthereum(payloadBytes)
	if err != nil {
		return nil, err
	}
	return &models.FaucetPackage{
		Payload:   payload,
		Signature: payloadSignature,
	}, nil
}

// CollectFaucet sends a CollectFaucetTx given a valid faucet package
func (c *Client) CollectFaucet(signer *ethereum.SignKeys, fpkg *models.FaucetPackage, nonce uint32) error {
	var req api.APIrequest
	var err error
	req.Method = "submitRawTx"

	tx := &models.CollectFaucetTx{
		TxType:        models.TxType_COLLECT_FAUCET,
		FaucetPackage: fpkg,
		// Nonce: nonce
	}

	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_CollectFaucet{CollectFaucet: tx}})
	if err != nil {
		return err
	}
	if stx.Signature, err = signer.SignVocdoniTx(stx.Tx); err != nil {
		return err
	}
	if req.Payload, err = proto.Marshal(stx); err != nil {
		return err
	}

	resp, err := c.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	return nil
}

// MintTokens sends a MintTokensTx in order to mint tokens for a given address
func (c *Client) MintTokens(treasurer *ethereum.SignKeys, to common.Address, amount uint64, nonce uint32) error {
	var req api.APIrequest
	var err error
	req.Method = "submitRawTx"

	tx := &models.MintTokensTx{
		Txtype: models.TxType_MINT_TOKENS,
		Nonce:  nonce,
		To:     to.Bytes(),
		Value:  amount,
	}

	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_MintTokens{MintTokens: tx}})
	if err != nil {
		return err
	}
	if stx.Signature, err = treasurer.SignVocdoniTx(stx.Tx); err != nil {
		return err
	}
	if req.Payload, err = proto.Marshal(stx); err != nil {
		return err
	}

	resp, err := c.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	return nil
}

// EndProcess sends a SetProcessStatusTx for ending a given process
func (c *Client) EndProcess(oracle *ethereum.SignKeys, pid []byte) error {
	var req api.APIrequest
	var err error
	req.Method = "submitRawTx"
	status := models.ProcessStatus_ENDED
	p := &models.SetProcessTx{
		Txtype:    models.TxType_SET_PROCESS_STATUS,
		ProcessId: pid,
		Status:    &status,
		Nonce:     util.RandomBytes(32),
	}
	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetProcess{SetProcess: p}})
	if err != nil {
		return err
	}
	if stx.Signature, err = oracle.SignVocdoniTx(stx.Tx); err != nil {
		return err
	}
	if req.Payload, err = proto.Marshal(stx); err != nil {
		return err
	}

	resp, err := c.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	return nil
}

// GetCurrentBlock returns the current vochain block number
func (c *Client) GetCurrentBlock() (uint32, error) {
	var req api.APIrequest
	req.Method = "getBlockHeight"
	resp, err := c.Request(req, nil)
	if err != nil {
		return 0, err
	}
	if !resp.Ok {
		return 0, fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	if resp.Height == nil {
		return 0, fmt.Errorf("height is nil")
	}
	return *resp.Height, nil
}
