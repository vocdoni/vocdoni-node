package client

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/snarks"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

type pkeys struct {
	pub  []types.Key
	priv []types.Key
	comm []types.Key
	rev  []types.Key
}

func (c *Client) GetEnvelopeStatus(nullifier, pid []byte) (bool, error) {
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

func (c *Client) GetProof(pubkey, root []byte) ([]byte, error) {
	var req types.MetaRequest
	req.Method = "genProof"
	req.CensusID = hex.EncodeToString(root)
	req.Digested = true
	req.CensusKey = snarks.Poseidon.Hash(pubkey)

	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, err
	}
	if len(resp.Siblings) == 0 || !resp.Ok {
		return nil, fmt.Errorf("cannot get merkle proof: (%s)", resp.Message)
	}

	return resp.Siblings, nil
}

func (c *Client) GetResults(pid []byte) ([][]string, string, error) {
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

func (c *Client) GetEnvelopeHeight(pid []byte) (uint32, error) {
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

func (c *Client) CensusSize(cid []byte) (int64, error) {
	var req types.MetaRequest
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

func (c *Client) ImportCensus(signer *ethereum.SignKeys, uri string) ([]byte, error) {
	var req types.MetaRequest
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

func (c *Client) GetKeys(pid, eid []byte) (*pkeys, error) {
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
		rev:  resp.RevealKeys,
	}, nil
}

func (c *Client) TestResults(pid []byte, totalVotes int) ([][]string, error) {
	log.Infof("waiting for results...")
	var err error
	var results [][]string
	var block uint32
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
	total := fmt.Sprintf("%d", totalVotes)
	if results[0][1] != total ||
		results[1][2] != total ||
		results[2][3] != total ||
		results[3][4] != total {
		return nil, fmt.Errorf("invalid results: %v", results)
	}
	return results, nil
}

func (c *Client) GetMerkleProofBatch(signers []*ethereum.SignKeys,
	root []byte,
	tolerateError bool) ([][]byte, error) {

	var proofs [][]byte
	// Generate merkle proofs
	log.Infof("generating proofs...")
	for i, s := range signers {
		proof, err := c.GetProof(s.PublicKey(), root)
		if err != nil {
			if tolerateError {
				continue
			}
			return proofs, err
		}
		proofs = append(proofs, proof)
		if (i+1)%100 == 0 {
			log.Infof("proof generation progress for %s: %d%%", c.Addr, ((i+1)*100)/(len(signers)))
		}
	}
	return proofs, nil
}

func (c *Client) GetCSPproofBatch(signers []*ethereum.SignKeys,
	ca *ethereum.SignKeys,
	pid []byte) ([][]byte, error) {

	var proofs [][]byte
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
		signature, err := ca.Sign(bundleBytes)
		if err != nil {
			log.Fatal(err)
		}

		proof := &models.ProofCA{
			Bundle:    bundle,
			Type:      models.SignatureType_ECDSA,
			Signature: signature,
		}
		p, err := proto.Marshal(proof)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, p)
		if (i+1)%100 == 0 {
			log.Infof("proof generation progress for %s: %d%%", c.Addr, ((i+1)*100)/(len(signers)))
		}
	}
	return proofs, nil
}

func (c *Client) TestSendVotes(
	pid,
	eid,
	root []byte,
	startBlock uint32,
	signers []*ethereum.SignKeys,
	censusOrigin models.CensusOrigin,
	caSigner *ethereum.SignKeys,
	proofs [][]byte,
	encrypted, doubleVoting, checkNullifiers bool,
	wg *sync.WaitGroup) (time.Duration, error) {

	var err error
	var keys []string
	// Generate merkle proofs
	if proofs == nil {
		switch censusOrigin {
		case models.CensusOrigin_OFF_CHAIN_TREE:
			proofs, err = c.GetMerkleProofBatch(signers, root, false)
		case models.CensusOrigin_OFF_CHAIN_CA:
			proofs, err = c.GetCSPproofBatch(signers, caSigner, pid)
		default:
			return 0, fmt.Errorf("censusOrigin not supported")
		}
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
	keyIndexes := []uint32{}
	if encrypted {
		if pk, err := c.GetKeys(pid, eid); err != nil {
			return 0, fmt.Errorf("cannot get process keys: (%s)", err)
		} else {
			for _, k := range pk.pub {
				if len(k.Key) > 0 {
					keyIndexes = append(keyIndexes, uint32(k.Idx))
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
	var vpb []byte
	start := time.Now()

	for i := 0; i < len(signers); i++ {
		s := signers[i]
		if vpb, err = genVote(encrypted, keys); err != nil {
			return 0, err
		}
		v := &models.VoteEnvelope{
			Nonce:                util.RandomBytes(32),
			ProcessId:            pid,
			VotePackage:          vpb,
			EncryptionKeyIndexes: keyIndexes,
		}
		switch censusOrigin {
		case models.CensusOrigin_OFF_CHAIN_TREE:
			v.Proof = &models.Proof{
				Payload: &models.Proof_Graviton{
					Graviton: &models.ProofGraviton{
						Siblings: proofs[i],
					},
				},
			}

		case models.CensusOrigin_OFF_CHAIN_CA:
			p := &models.ProofCA{}
			if err := proto.Unmarshal(proofs[i], p); err != nil {
				log.Fatal(err)
			}
			v.Proof = &models.Proof{Payload: &models.Proof_Ca{Ca: p}}

		default:
			log.Fatal("censusOrigin %s not supported", censusOrigin.String())
		}

		stx := &models.SignedTx{}
		stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: v}})
		if err != nil {
			return 0, err
		}

		if stx.Signature, err = s.Sign(stx.Tx); err != nil {
			return 0, err
		}
		if req.Payload, err = proto.Marshal(stx); err != nil {
			return 0, err
		}
		pub, _ := s.HexString()
		log.Debugf("voting with pubKey:%s", pub)
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
			log.Infof("voting progress for %s: %d%%", c.Addr, ((i+1)*100)/(len(signers)))
		}

		// Try double voting (should fail)
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
			log.Warnf("error getting envelope height: %v", err)
			continue
		} else {
			if h >= uint32(len(signers)) {
				break
			}
		}
		if time.Since(checkStart) > timeDeadLine {
			return 0, fmt.Errorf("wait for envelope height took more than deadline, skipping")
		}
	}
	votingElapsedTime := time.Since(start)

	if !checkNullifiers {
		return votingElapsedTime, nil
	}
	// If checkNullifiers, wait until all votes have been verified
	for {
		for i, nullHex := range nullifiers {
			if nullHex == "registered" {
				registered++
				continue
			}
			null, err := hex.DecodeString(nullHex)
			if err != nil {
				return 0, err
			}
			if es, _ := c.GetEnvelopeStatus(null, pid); es {
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

func (c *Client) CreateProcess(oracle *ethereum.SignKeys,
	entityID, censusRoot []byte,
	censusURI string,
	pid []byte,
	envelopeType *models.EnvelopeType,
	censusOrigin models.CensusOrigin,
	duration int) (uint32, error) {
	var req types.MetaRequest
	req.Method = "submitRawTx"
	block, err := c.GetCurrentBlock()
	if err != nil {
		return 0, err
	}
	processData := &models.Process{
		EntityId:     entityID,
		CensusRoot:   censusRoot,
		CensusURI:    &censusURI,
		CensusOrigin: censusOrigin,
		BlockCount:   uint32(duration),
		ProcessId:    pid,
		StartBlock:   block + 4,
		EnvelopeType: envelopeType,
		Mode:         &models.ProcessMode{AutoStart: true, Interruptible: true},
		Status:       models.ProcessStatus_READY,
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 8},
	}
	p := &models.NewProcessTx{
		Txtype:  models.TxType_NEW_PROCESS,
		Nonce:   util.RandomBytes(32),
		Process: processData,
	}
	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_NewProcess{NewProcess: p}})
	if err != nil {
		return 0, err
	}
	if stx.Signature, err = oracle.Sign(stx.Tx); err != nil {
		return 0, err
	}
	if req.Payload, err = proto.Marshal(stx); err != nil {
		return 0, err
	}

	resp, err := c.Request(req, nil)
	if err != nil {
		return 0, err
	}
	if !resp.Ok {
		return 0, fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	return p.Process.StartBlock, nil
}

func (c *Client) EndProcess(oracle *ethereum.SignKeys, pid []byte) error {
	var req types.MetaRequest
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
	if stx.Signature, err = oracle.Sign(stx.Tx); err != nil {
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

func (c *Client) GetCurrentBlock() (uint32, error) {
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
// Users public keys can be added using censusSigner (ethereum.SignKeys) or
// censusPubKeys (raw hex public keys).
func (c *Client) CreateCensus(signer *ethereum.SignKeys, censusSigners []*ethereum.SignKeys,
	censusPubKeys []string) (root []byte, uri string, _ error) {
	var req types.MetaRequest

	// Create census
	log.Infof("Create census")
	req.Method = "addCensus"
	req.CensusID = fmt.Sprintf("test%d", rand.Int())
	resp, err := c.Request(req, signer)
	if err != nil {
		return nil, "", err
	}
	if !resp.Ok {
		return nil, "", fmt.Errorf("%s failed: %s", req.Method, resp.Message)
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
	req.CensusKey = []byte{}
	req.Digested = true
	currentSize := censusSize
	i := 0
	var hexpub string
	for currentSize > 0 {
		claims := [][]byte{}
		for j := 0; j < 100; j++ {
			if currentSize < 1 {
				break
			}
			if censusSigners != nil {
				hexpub, _ = censusSigners[currentSize-1].HexString()
			} else {
				hexpub = censusPubKeys[currentSize-1]
			}
			pub, err := hex.DecodeString(hexpub)
			if err != nil {
				return nil, "", err
			}
			claims = append(claims, snarks.Poseidon.Hash(pub))
			currentSize--
		}
		req.CensusKeys = claims
		resp, err := c.Request(req, signer)
		if err != nil {
			return nil, "", err
		}
		if !resp.Ok {
			return nil, "", fmt.Errorf("%s failed: %s", req.Method, resp.Message)
		}
		i++
		log.Infof("census creation progress: %d%%", (i*100*100)/(censusSize))
	}

	// getSize
	log.Infof("get size")
	req.Method = "getSize"
	req.RootHash = nil
	resp, err = c.Request(req, nil)
	if err != nil {
		return nil, "", err
	}
	if got := *resp.Size; int64(censusSize) != got {
		return nil, "", fmt.Errorf("expected size %v, got %v", censusSize, got)
	}

	// publish
	log.Infof("publish census")
	req.Method = "publish"
	req.CensusKeys = [][]byte{}
	resp, err = c.Request(req, signer)
	if err != nil {
		return nil, "", err
	}
	if !resp.Ok {
		return nil, "", fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	uri = resp.URI
	if len(uri) < 40 {
		return nil, "", fmt.Errorf("got invalid URI")
	}

	// getRoot
	log.Infof("get root")
	req.Method = "getRoot"
	resp, err = c.Request(req, nil)
	if err != nil {
		return nil, "", err
	}
	return resp.Root, uri, nil
}
