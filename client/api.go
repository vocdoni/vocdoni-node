package client

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

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

func (c *APIConnection) GetEnvelopeStatus(nullifier, pid string) (bool, error) {
	var req types.MetaRequest
	req.Method = "getEnvelopeStatus"
	req.ProcessID = pid
	req.Nullifier = nullifier
	resp := c.Request(req, nil)
	if !resp.Ok || resp.Registered == nil {
		return false, fmt.Errorf("cannot check envelope (%s)", resp.Message)
	}
	return *resp.Registered, nil
}

func (c *APIConnection) GetProof(hexpubkey, root string) (string, error) {
	var req types.MetaRequest
	req.Method = "genProof"
	req.CensusID = root
	req.Digested = true
	pubkey, err := hex.DecodeString(hexpubkey)
	if err != nil {
		return "", err
	}
	req.ClaimData = base64.StdEncoding.EncodeToString(snarks.Poseidon.Hash(pubkey))

	resp := c.Request(req, nil)
	if len(resp.Siblings) == 0 || !resp.Ok {
		return "", fmt.Errorf("cannot get merkle proof: (%s)", resp.Message)
	}

	return resp.Siblings, nil
}

func (c *APIConnection) GetResults(pid string) ([][]uint32, error) {
	var req types.MetaRequest
	req.Method = "getResults"
	req.ProcessID = pid
	resp := c.Request(req, nil)
	if !resp.Ok {
		return nil, fmt.Errorf("cannot get results: (%s)", resp.Message)
	}
	return resp.Results, nil
}

func (c *APIConnection) GetEnvelopeHeight(pid string) (int64, error) {
	var req types.MetaRequest
	req.Method = "getEnvelopeHeight"
	req.ProcessID = pid
	resp := c.Request(req, nil)
	if !resp.Ok {
		return 0, fmt.Errorf(resp.Message)
	}
	return *resp.Height, nil
}

func (c *APIConnection) ImportCensus(signer *ethereum.SignKeys, uri string) (string, error) {
	var req types.MetaRequest
	req.Method = "addCensus"
	req.CensusID = RandomHex(16)
	resp := c.Request(req, signer)

	req.Method = "importRemote"
	req.CensusID = resp.CensusID
	req.URI = uri
	resp = c.Request(req, signer)
	if !resp.Ok {
		return "", fmt.Errorf(resp.Message)
	}

	req.Method = "publish"
	req.URI = ""
	resp = c.Request(req, signer)
	if !resp.Ok {
		return "", fmt.Errorf(resp.Message)
	}
	return resp.Root, nil
}

func (c *APIConnection) GetKeys(pid, eid string) (*pkeys, error) {
	var req types.MetaRequest
	req.Method = "getProcessKeys"
	req.ProcessID = pid
	req.EntityId = eid
	resp := c.Request(req, nil)
	if !resp.Ok {
		return nil, fmt.Errorf("cannot get keys for process %s: (%s)", pid, resp.Message)
	}
	return &pkeys{
		pub:  resp.EncryptionPublicKeys,
		priv: resp.EncryptionPrivKeys,
		comm: resp.CommitmentKeys,
		rev:  resp.RevealKeys}, nil
}

func (c *APIConnection) Results(pid string, totalVotes int, startBlock, duration int64) (results [][]uint32, err error) {
	log.Infof("waiting for results...")
	c.WaitUntilBlock(startBlock + duration + 2)
	if results, err = c.GetResults(pid); err != nil {
		return nil, err
	}
	total := uint32(totalVotes)
	if results[0][1] != total || results[1][2] != total || results[2][3] != total || results[3][4] != total {
		return nil, fmt.Errorf("invalid results")
	}
	return results, nil
}

func (c *APIConnection) SendVotes(pid, eid, root string, startBlock, duration int64, signers []*ethereum.SignKeys, encrypted bool) (time.Duration, error) {
	var pub, proof string
	var err error
	var keys []string
	var proofs []string

	log.Infof("generating proofs...")
	for i, s := range signers {
		pub, _ = s.HexString()
		pub, _ = ethereum.DecompressPubKey(pub) // Temporary until everything is compressed
		if proof, err = c.GetProof(pub, root); err != nil {
			return 0, err
		}
		proofs = append(proofs, proof)
		if (i+1)%100 == 0 {
			log.Infof("proof generation progress: %d%%", int(((i+1)*100)/(len(signers))))
		}
	}

	c.WaitUntilBlock(startBlock)
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

	var req types.MetaRequest
	var nullifiers []string
	var vpb string
	req.Method = "submitRawTx"
	start := time.Now()
	log.Infof("sending votes")

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

		txBytes, err := json.Marshal(v)
		if err != nil {
			return 0, err
		}
		if v.Signature, err = s.Sign(txBytes); err != nil {
			return 0, err
		}
		v.Type = "vote"
		if txBytes, err = json.Marshal(v); err != nil {
			return 0, err
		}
		req.RawTx = base64.StdEncoding.EncodeToString(txBytes)
		resp := c.Request(req, nil)
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
			log.Infof("voting progress: %d%%", int(((i+1)*100)/(len(signers))))
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
		if time.Since(checkStart) > time.Minute*10 {
			return 0, fmt.Errorf("wait for envelope height took more than 10 minutes, skipping")
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
		if time.Since(checkStart) > time.Minute*10 {
			return 0, fmt.Errorf("check nullifier time took more than 10 minutes, skipping")
		}
	}
	return votingElapsedTime, nil
}

func (c *APIConnection) CreateProcess(oracle *ethereum.SignKeys, entityID, mkroot, mkuri, pid, ptype string, duration int) (int64, error) {
	var req types.MetaRequest
	req.Method = "submitRawTx"
	p := types.NewProcessTx{
		Type:           "newProcess",
		EntityID:       entityID,
		MkRoot:         mkroot,
		MkURI:          mkuri,
		NumberOfBlocks: int64(duration),
		ProcessID:      pid,
		ProcessType:    ptype,
		StartBlock:     c.GetCurrentBlock() + 4,
	}
	txBytes, err := json.Marshal(p)
	if err != nil {
		return 0, err
	}
	if p.Signature, err = oracle.Sign(txBytes); err != nil {
		return 0, err
	}
	if txBytes, err = json.Marshal(p); err != nil {
		return 0, err
	}
	req.RawTx = base64.StdEncoding.EncodeToString(txBytes)

	resp := c.Request(req, nil)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	return p.StartBlock, nil
}

func (c *APIConnection) CancelProcess(oracle *ethereum.SignKeys, pid string) error {
	var req types.MetaRequest
	req.Method = "submitRawTx"
	p := types.NewProcessTx{
		Type:      "cancelProcess",
		ProcessID: pid,
	}
	txBytes, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if p.Signature, err = oracle.Sign(txBytes); err != nil {
		return err
	}
	if txBytes, err = json.Marshal(p); err != nil {
		return err
	}
	req.RawTx = base64.StdEncoding.EncodeToString(txBytes)

	resp := c.Request(req, nil)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	return nil
}

func (c *APIConnection) GetCurrentBlock() int64 {
	var req types.MetaRequest
	req.Method = "getBlockHeight"
	resp := c.Request(req, nil)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	if resp.Height == nil {
		log.Fatalf("height is nil!")
	}
	return *resp.Height

}

// CreateCensus creates a new census on the remote gateway and publishes it.
// Users public keys can be added using censusSigner (ethereum.SignKeys) or censusPubKeys (raw hex public keys).
// First has preference.
func (c *APIConnection) CreateCensus(signer *ethereum.SignKeys, censusSigners []*ethereum.SignKeys, censusPubKeys []string) (root, uri string) {
	var req types.MetaRequest
	var resp *types.MetaResponse

	// Create census
	log.Infof("Create census")
	req.Method = "addCensus"
	req.CensusID = fmt.Sprintf("test%d", rand.Int())
	resp = c.Request(req, signer)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
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
				log.Fatal(err)
			}
			data = base64.StdEncoding.EncodeToString(snarks.Poseidon.Hash(pub))
			claims = append(claims, data)
			currentSize--
		}
		req.ClaimsData = claims
		resp = c.Request(req, signer)
		if !resp.Ok {
			log.Fatalf("%s failed: %s", req.Method, resp.Message)
		}
		i++
		log.Infof("census creation progress: %d%%", int((i*100*100)/(censusSize)))
	}

	// getSize
	log.Infof("get size")
	req.Method = "getSize"
	req.RootHash = ""
	resp = c.Request(req, nil)
	if got := *resp.Size; int64(censusSize) != got {
		log.Fatalf("expected size %v, got %v", censusSize, got)
	}

	// publish
	log.Infof("publish census")
	req.Method = "publish"
	req.ClaimsData = []string{}
	resp = c.Request(req, signer)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	uri = resp.URI
	if len(uri) < 40 {
		log.Fatalf("got invalid URI")
	}

	// getRoot
	log.Infof("get root")
	req.Method = "getRoot"
	resp = c.Request(req, nil)
	root = resp.Root
	if len(root) < 1 {
		log.Fatalf("got invalid root")
	}

	return root, uri
}
