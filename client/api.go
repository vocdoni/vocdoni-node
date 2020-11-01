package client

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	bare "git.sr.ht/~sircmpwn/go-bare"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

type pkeys struct {
	pub  []types.Key
	priv []types.Key
	comm []types.Key
	rev  []types.Key
}

func (c *Client) GetEnvelopeStatus(nullifier, pid string) (bool, error) {
	var req types.MetaRequest
	req.Method = "getEnvelopeStatus"
	req.ProcessID = pid
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

func (c *Client) GetProof(hexpubkey, root string) (string, error) {
	var req types.MetaRequest
	req.Method = "genProof"
	req.CensusID = root
	req.Digested = true
	pubkey, err := hex.DecodeString(hexpubkey)
	if err != nil {
		return "", err
	}
	req.ClaimData = base64.StdEncoding.EncodeToString(snarks.Poseidon.Hash(pubkey))

	resp, err := c.Request(req, nil)
	if err != nil {
		return "", err
	}
	if len(resp.Siblings) == 0 || !resp.Ok {
		return "", fmt.Errorf("cannot get merkle proof: (%s)", resp.Message)
	}

	return resp.Siblings, nil
}

func (c *Client) GetResults(pid string) ([][]uint32, string, error) {
	var req types.MetaRequest
	req.Method = "getResults"
	req.ProcessID = pid
	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, "", err
	}
	if !resp.Ok {
		return nil, "", fmt.Errorf("cannot get results: (%s)", resp.Message)
	}
	if resp.Message == "no results yet" {
		return nil, resp.State, nil
	}
	return resp.Results, resp.State, nil
}

func (c *Client) GetEnvelopeHeight(pid string) (int64, error) {
	var req types.MetaRequest
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

func (c *Client) CensusSize(cid string) (int64, error) {
	var req types.MetaRequest
	req.Method = "getSize"
	req.CensusID = cid
	resp, err := c.Request(req, nil)
	if err != nil {
		return 0, err
	}
	if !resp.Ok {
		return 0, fmt.Errorf(resp.Message)
	}
	return *resp.Size, nil
}

func (c *Client) ImportCensus(signer *ethereum.SignKeys, uri string) (string, error) {
	var req types.MetaRequest
	req.Method = "addCensus"
	req.CensusID = RandomHex(16)
	resp, err := c.Request(req, signer)
	if err != nil {
		return "", err
	}
	if err != nil {
		return "", err
	}

	req.Method = "importRemote"
	req.CensusID = resp.CensusID
	req.URI = uri
	resp, err = c.Request(req, signer)
	if err != nil {
		return "", err
	}
	if !resp.Ok {
		return "", fmt.Errorf(resp.Message)
	}

	req.Method = "publish"
	req.URI = ""
	resp, err = c.Request(req, signer)
	if err != nil {
		return "", err
	}
	if !resp.Ok {
		return "", fmt.Errorf(resp.Message)
	}
	return resp.Root, nil
}

func (c *Client) GetKeys(pid, eid string) (*pkeys, error) {
	var req types.MetaRequest
	req.Method = "getProcessKeys"
	req.ProcessID = pid
	req.EntityId = eid
	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("cannot get keys for process %s: (%s)", pid, resp.Message)
	}
	return &pkeys{
		pub:  resp.EncryptionPublicKeys,
		priv: resp.EncryptionPrivKeys,
		comm: resp.CommitmentKeys,
		rev:  resp.RevealKeys}, nil
}

func (c *Client) TestResults(pid string, totalVotes int) ([][]uint32, error) {
	log.Infof("waiting for results...")
	var err error
	var results [][]uint32
	var block int64
	for {
		block, err = c.GetCurrentBlock()
		if err != nil {
			return nil, err
		}
		c.WaitUntilBlock(block + 1)
		results, _, err = c.GetResults(pid)
		if err != nil {
			return nil, err
		}
		if results != nil {
			break
		}
		log.Infof("no results yet at block %d", block+2)
	}
	total := uint32(totalVotes)
	if results[0][1] != total || results[1][2] != total || results[2][3] != total || results[3][4] != total {
		return nil, fmt.Errorf("invalid results: %v", results)
	}
	return results, nil
}

func (c *Client) GetProofBatch(signers []*ethereum.SignKeys, root string, tolerateError bool) ([]string, error) {
	var proofs []string
	var pub, proof string
	var err error
	// Generate merkle proofs
	log.Infof("generating proofs...")
	for i, s := range signers {
		if pub, _ = s.HexString(); pub == "" {
			if tolerateError {
				continue
			}
			return proofs, err
		}
		if pub, err = ethereum.DecompressPubKey(pub); err != nil {
			if tolerateError {
				continue
			}
			return proofs, err
		} // Temporary until everything is compressed

		if proof, err = c.GetProof(pub, root); err != nil {
			if tolerateError {
				continue
			}
			return proofs, err
		}
		proofs = append(proofs, proof)
		if (i+1)%100 == 0 {
			log.Infof("proof generation progress for %s: %d%%", c.Addr, int(((i+1)*100)/(len(signers))))
		}
	}
	return proofs, nil
}

func (c *Client) TestSendVotes(pid, eid, root string, startBlock int64, signers []*ethereum.SignKeys, proofs []string, encrypted bool, doubleVoting bool, wg *sync.WaitGroup) (time.Duration, error) {
	var err error
	var keys []string
	// Generate merkle proofs
	if proofs == nil {
		proofs, err = c.GetProofBatch(signers, root, false)
	}
	if err != nil {
		return 0, err
	}
	// Wait until all gateway connections are ready
	wg.Done()
	log.Infof("%s is waiting other gateways to be ready before start voting", c.Addr)
	c.WaitUntilBlock(startBlock)
	wg.Wait()

	// Get encryption keys
	keyIndexes := []int{}
	if encrypted {
		if pk, err := c.GetKeys(pid, eid); err != nil {
			return 0, fmt.Errorf("cannot get process keys: (%s)", err)
		} else {
			for _, k := range pk.pub {
				if len(k.Key) > 0 {
					keyIndexes = append(keyIndexes, k.Idx)
					keys = append(keys, k.Key)
				}
			}
		}
		if len(keys) == 0 {
			return 0, fmt.Errorf("process keys is empty")
		}
		log.Infof("got encryption keys!")
	}

	// Send votes
	log.Infof("sending votes")
	timeDeadLine := time.Second * 200
	if len(signers) > 1000 {
		timeDeadLine = time.Duration(len(signers)/5) * time.Second
	}
	log.Infof("time deadline set to %d seconds", timeDeadLine/time.Second)
	req := types.MetaRequest{Method: "submitRawTx"}
	nullifiers := []string{}
	vpb := ""
	start := time.Now()

	for i := 0; i < len(signers); i++ {
		s := signers[i]
		if vpb, err = genVote(encrypted, keys); err != nil {
			return 0, err
		}
		v := types.VoteTx{
			Nonce:                RandomHex(16),
			ProcessID:            pid,
			Proof:                proofs[i],
			VotePackage:          vpb,
			EncryptionKeyIndexes: keyIndexes,
		}

		txBytes, err := bare.Marshal(&v)
		if err != nil {
			return 0, err
		}
		if v.Signature, err = s.Sign(txBytes); err != nil {
			return 0, err
		}
		v.Type = "vote"
		if txBytes, err = bare.Marshal(&v); err != nil {
			return 0, err
		}
		pub, _ := s.HexString()
		pubd, _ := ethereum.DecompressPubKey(pub)
		log.Debugf("voting with pubKey:%s", pubd)
		req.RawTx = base64.StdEncoding.EncodeToString(txBytes)
		resp, err := c.Request(req, nil)
		if err != nil {
			return 0, err
		}
		if !resp.Ok {
			if strings.Contains(resp.Message, "mempool is full") {
				log.Warnf("mempool is full, wait and retry")
				time.Sleep(1 * time.Second)
				i--
			} else {
				return 0, fmt.Errorf("%s failed: %s", req.Method, resp.Message)
			}
		}
		nullifiers = append(nullifiers, resp.Payload)
		if (i+1)%100 == 0 {
			log.Infof("voting progress for %s: %d%%", c.Addr, int(((i+1)*100)/(len(signers))))
		}

		//Try double voting (should fail)
		if doubleVoting {
			resp, err := c.Request(req, nil)
			if err != nil {
				return 0, err
			}
			if resp.Ok {
				return 0, fmt.Errorf("double voting detected")
			}
		}
	}
	log.Infof("votes submited! took %s", time.Since(start))
	checkStart := time.Now()
	registered := 0
	log.Infof("waiting for votes to be validated...")
	for {
		time.Sleep(time.Millisecond * 500)
		if h, err := c.GetEnvelopeHeight(pid); err != nil {
			log.Warnf("error getting envelope height")
			continue
		} else {
			if h >= int64(len(signers)) {
				break
			}
		}
		if time.Since(checkStart) > timeDeadLine {
			return 0, fmt.Errorf("wait for envelope height took more than deadline, skipping")
		}
	}
	votingElapsedTime := time.Since(start)
	for {
		for i, n := range nullifiers {
			if n == "registered" {
				registered++
				continue
			}
			if es, _ := c.GetEnvelopeStatus(n, pid); es {
				nullifiers[i] = "registered"
			}
		}
		if registered == len(nullifiers) {
			break
		}
		registered = 0
		if time.Since(checkStart) > timeDeadLine {
			return 0, fmt.Errorf("check nullifier time took more than deadline, skipping")
		}
	}
	return votingElapsedTime, nil
}

func (c *Client) CreateProcess(oracle *ethereum.SignKeys, entityID, mkroot, mkuri, pid, ptype string, duration int) (int64, error) {
	var req types.MetaRequest
	req.Method = "submitRawTx"
	block, err := c.GetCurrentBlock()
	if err != nil {
		return 0, err
	}
	p := types.NewProcessTx{
		Type:           "newProcess",
		EntityID:       entityID,
		MkRoot:         mkroot,
		MkURI:          mkuri,
		NumberOfBlocks: int64(duration),
		ProcessID:      pid,
		ProcessType:    ptype,
		StartBlock:     block + 4,
	}
	txBytes, err := bare.Marshal(&p)
	if err != nil {
		return 0, err
	}
	if p.Signature, err = oracle.Sign(txBytes); err != nil {
		return 0, err
	}
	if txBytes, err = bare.Marshal(&p); err != nil {
		return 0, err
	}
	req.RawTx = base64.StdEncoding.EncodeToString(txBytes)

	resp, err := c.Request(req, nil)
	if err != nil {
		return 0, err
	}
	if !resp.Ok {
		return 0, fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	return p.StartBlock, nil
}

func (c *Client) CancelProcess(oracle *ethereum.SignKeys, pid string) error {
	var req types.MetaRequest
	req.Method = "submitRawTx"
	p := types.CancelProcessTx{
		Type:      "cancelProcess",
		ProcessID: pid,
	}
	txBytes, err := bare.Marshal(&p)
	if err != nil {
		return err
	}
	if p.Signature, err = oracle.Sign(txBytes); err != nil {
		return err
	}
	if txBytes, err = bare.Marshal(&p); err != nil {
		return err
	}
	req.RawTx = base64.StdEncoding.EncodeToString(txBytes)

	resp, err := c.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	return nil
}

func (c *Client) GetCurrentBlock() (int64, error) {
	var req types.MetaRequest
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

// CreateCensus creates a new census on the remote gateway and publishes it.
// Users public keys can be added using censusSigner (ethereum.SignKeys) or censusPubKeys (raw hex public keys).
// First has preference.
func (c *Client) CreateCensus(signer *ethereum.SignKeys, censusSigners []*ethereum.SignKeys, censusPubKeys []string) (root, uri string, _ error) {
	var req types.MetaRequest

	// Create census
	log.Infof("Create census")
	req.Method = "addCensus"
	req.CensusID = fmt.Sprintf("test%d", rand.Int())
	resp, err := c.Request(req, signer)
	if err != nil {
		return "", "", err
	}
	if !resp.Ok {
		return "", "", fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	// Set correct censusID for commint requests
	req.CensusID = resp.CensusID

	// addClaimBulk
	censusSize := 0
	if censusSigners != nil {
		censusSize = len(censusSigners)
	} else {
		censusSize = len(censusPubKeys)
	}
	log.Infof("add bulk claims (size %d)", censusSize)
	req.Method = "addClaimBulk"
	req.ClaimData = ""
	req.Digested = true
	currentSize := censusSize
	i := 0
	var data, hexpub string
	for currentSize > 0 {
		claims := []string{}
		for j := 0; j < 100; j++ {
			if currentSize < 1 {
				break
			}
			if censusSigners != nil {
				hexpub, _ = censusSigners[currentSize-1].HexString()
			} else {
				hexpub = censusPubKeys[currentSize-1]
			}
			hexpub, _ = ethereum.DecompressPubKey(hexpub) // Temporary until everything is compressed only
			pub, err := hex.DecodeString(hexpub)
			if err != nil {
				return "", "", err
			}
			data = base64.StdEncoding.EncodeToString(snarks.Poseidon.Hash(pub))
			claims = append(claims, data)
			currentSize--
		}
		req.ClaimsData = claims
		resp, err := c.Request(req, signer)
		if err != nil {
			return "", "", err
		}
		if !resp.Ok {
			return "", "", fmt.Errorf("%s failed: %s", req.Method, resp.Message)
		}
		i++
		log.Infof("census creation progress: %d%%", int((i*100*100)/(censusSize)))
	}

	// getSize
	log.Infof("get size")
	req.Method = "getSize"
	req.RootHash = ""
	resp, err = c.Request(req, nil)
	if err != nil {
		return "", "", err
	}
	if got := *resp.Size; int64(censusSize) != got {
		return "", "", fmt.Errorf("expected size %v, got %v", censusSize, got)
	}

	// publish
	log.Infof("publish census")
	req.Method = "publish"
	req.ClaimsData = []string{}
	resp, err = c.Request(req, signer)
	if err != nil {
		return "", "", err
	}
	if !resp.Ok {
		return "", "", fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	uri = resp.URI
	if len(uri) < 40 {
		return "", "", fmt.Errorf("got invalid URI")
	}

	// getRoot
	log.Infof("get root")
	req.Method = "getRoot"
	resp, err = c.Request(req, nil)
	if err != nil {
		return "", "", err
	}
	root = resp.Root
	if len(root) < 1 {
		return "", "", fmt.Errorf("got invalid root")
	}

	return root, uri, nil
}
