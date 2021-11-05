package client

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/artifacts"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

type pkeys struct {
	pub  []api.Key
	priv []api.Key
}

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

func (c *Client) GetEnvelopeStatus(nullifier, pid []byte) (bool, error) {
	var req api.APIrequest
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
	var req api.APIrequest
	req.Method = "genProof"
	req.CensusID = hex.EncodeToString(root)
	req.Digested = false
	req.CensusKey = pubkey

	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, err
	}
	if len(resp.Siblings) == 0 || !resp.Ok {
		return nil, fmt.Errorf("cannot get merkle proof: (%s)", resp.Message)
	}

	return resp.Siblings, nil
}

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
		return nil, fmt.Errorf("cannot get keys for process %s: (%s)", pid, resp.Message)
	}
	return &pkeys{
		pub:  resp.EncryptionPublicKeys,
		priv: resp.EncryptionPrivKeys,
	}, nil
}

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

func (c *Client) TestResults(pid []byte, totalVotes int) ([][]string, error) {
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

func (c *Client) GetMerkleProofPoseidonBatch(signers []*ethereum.SignKeys,
	root []byte,
	tolerateError bool) ([][]byte, error) {

	var proofs [][]byte
	// Generate merkle proofs
	log.Infof("generating poseidon proofs...")
	for i, s := range signers {
		zkCensusKey, _ := testGetZKCensusKey(s)
		proof, err := c.GetProof(zkCensusKey, root)
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
			Type:      models.ProofCA_ECDSA,
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

// testGetZKCensusKey returns zkCensusKey, secretKey.  For testing purposes, we
// generate a secretKey from a signature by the ethereum key.
func testGetZKCensusKey(s *ethereum.SignKeys) ([]byte, []byte) {
	// secret is 65 bytes
	secret, err := s.Sign([]byte("secretKey"))
	if err != nil {
		log.Fatalf("Cannot sign: %v", err)
	}
	hasher := arbo.HashPoseidon{}
	secretKey, err := hasher.Hash(secret[:22], secret[22:44], secret[44:])
	if err != nil {
		log.Fatalf("Cannnot calculate pre-register key with Poseidon: %v", err)
	}
	pubKey, err := hasher.Hash(secretKey)
	if err != nil {
		log.Fatalf("Cannnot calculate pre-register key with Poseidon: %v", err)
	}
	return pubKey, secretKey
}

func (c *Client) TestPreRegisterKeys(
	pid,
	eid,
	root []byte,
	startBlock uint32,
	signers []*ethereum.SignKeys,
	censusOrigin models.CensusOrigin,
	caSigner *ethereum.SignKeys,
	proofs [][]byte,
	doubleVote, doublePreRegister, checkNullifiers bool,
	wg *sync.WaitGroup) (time.Duration, error) {

	var err error
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
	log.Infof("%s is waiting other gateways to be ready before it can start voting", c.Addr)
	wg.Wait()

	// Wait for process to be registered
	log.Infof("waiting for process %x to be registered...", pid)
	for {
		proc, err := c.GetProcessInfo(pid)
		if err != nil {
			log.Infof("Process not yet available: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Infof("Process: %+v\n", proc)
		break
	}
	cb, err := c.GetCurrentBlock()
	if err != nil {
		return 0, err
	}
	log.Infof("Current block: %v", cb)

	// Send votes
	log.Infof("sending pre-register keys")
	timeDeadLine := time.Second * 200
	if len(signers) > 1000 {
		timeDeadLine = time.Duration(len(signers)/5) * time.Second
	}
	log.Infof("time deadline set to %d seconds", timeDeadLine/time.Second)
	req := api.APIrequest{Method: "submitRawTx"}
	start := time.Now()

	for i := 0; i < len(signers); i++ {
		s := signers[i]
		zkCensusKey, _ := testGetZKCensusKey(s)
		v := &models.RegisterKeyTx{
			Nonce:     util.RandomBytes(32),
			ProcessId: pid,
			NewKey:    zkCensusKey,
		}
		switch censusOrigin {
		case models.CensusOrigin_OFF_CHAIN_TREE:
			v.Proof = &models.Proof{
				Payload: &models.Proof_Arbo{
					Arbo: &models.ProofArbo{
						Type:     models.ProofArbo_BLAKE2B,
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
		stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_RegisterKey{RegisterKey: v}})
		if err != nil {
			return 0, err
		}

		if stx.Signature, err = s.Sign(stx.Tx); err != nil {
			return 0, err
		}
		if req.Payload, err = proto.Marshal(stx); err != nil {
			return 0, err
		}
		log.Debugf("pre-registering zkCensusKey:%x", zkCensusKey)
		resp, err := c.Request(req, nil)
		if err != nil {
			return 0, err
		}
		if !resp.Ok {
			if strings.Contains(resp.Message, "mempool is full") {
				log.Warnf("mempool is full, waiting and retrying")
				time.Sleep(1 * time.Second)
				i--
			} else {
				return 0, fmt.Errorf("%s failed: %s", req.Method, resp.Message)
			}
		}
		if (i+1)%100 == 0 {
			log.Infof("pre-register progress for %s: %d%%", c.Addr, ((i+1)*100)/(len(signers)))
		}

		// Try double preRegister (should fail)
		if doublePreRegister {
			resp, err := c.Request(req, nil)
			if err != nil {
				return 0, err
			}
			if resp.Ok {
				return 0, fmt.Errorf("double pre-register not detected")
			}
		}
	}

	log.Infof("pre-registers submited! took %s", time.Since(start))
	preRegisterEapsedTime := time.Since(start)

	return preRegisterEapsedTime, nil
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
	log.Infof("%s is waiting other gateways to be ready before it can start voting", c.Addr)
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
	req := api.APIrequest{Method: "submitRawTx"}
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
				Payload: &models.Proof_Arbo{
					Arbo: &models.ProofArbo{
						Type:     models.ProofArbo_BLAKE2B,
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
				log.Warnf("mempool is full, waiting and retrying")
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
			return 0, fmt.Errorf("waiting for envelope height took longer than deadline, skipping")
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
			return 0, fmt.Errorf("checking nullifier time took more than deadline, skipping")
		}
	}

	return votingElapsedTime, nil
}

func (c *Client) TestSendAnonVotes(
	pid,
	eid,
	root []byte,
	startBlock uint32,
	signers []*ethereum.SignKeys,
	doubleVoting, checkNullifiers bool,
	wg *sync.WaitGroup) (time.Duration, error) {

	proofs, err := c.GetMerkleProofPoseidonBatch(signers, root, false)
	if err != nil {
		return 0, err
	}
	log.Infof("Requested %v proofs", len(proofs))
	circuitIndex, circuitConfig, err := c.GetCircuitConfig(pid)
	if err != nil {
		return 0, err
	}
	log.Infof("CircuitIndex: %v, CircuitConfit: %+v", *circuitIndex, *circuitConfig)
	// Wait until all gateway connections are ready
	wg.Done()
	log.Infof("%s is waiting other gateways to be ready before it can start voting", c.Addr)
	c.WaitUntilBlock(startBlock)
	wg.Wait()

	// Send votes
	log.Infof("sending votes")
	timeDeadLine := time.Second * 200
	if len(signers) > 1000 {
		timeDeadLine = time.Duration(len(signers)/5) * time.Second
	}
	log.Infof("time deadline set to %d seconds", timeDeadLine/time.Second)
	req := api.APIrequest{Method: "submitRawTx"}
	nullifiers := []string{}
	var vpb []byte
	start := time.Now()

	for i := 0; i < len(signers); i++ {
		s := signers[i]
		if vpb, err = genVote(false, nil); err != nil {
			return 0, err
		}
		v := &models.VoteEnvelope{
			Nonce:       util.RandomBytes(32),
			ProcessId:   pid,
			VotePackage: vpb,
		}
		// TODO: Generate SNARK proof here.  Inputs:
		// merkleTreeProof := proofs[i] // []byte // TODO: JS: Uncompress the merkle proof
		// zkCensusKey, secretKey := testGetZKCensusKey(s) // []byte, []byte
		// rollingCensusRoot := root
		// MISSING: Index (Merkle Tree key)
		// MISSING: VoteHash see https://github.com/vocdoni/zk-franchise-proof-circuit/blob/fd55e571949b8db429d798062c670bf23b463486/test/go-inputs-generator/census_test.go#L90
		// electionId := eid
		// MISSING nullifier: https://github.com/vocdoni/zk-franchise-proof-circuit/blob/fd55e571949b8db429d798062c670bf23b463486/test/go-inputs-generator/census_test.go#L94
		// `node run gen_proof.js '{"censusRoot": "1234", "censusSiblings": ...}'`
		v.Proof = &models.Proof{
			Payload: &models.Proof_ZkSnark{
				ZkSnark: &models.ProofZkSNARK{
					CircuitParametersIndex: int32(*circuitIndex),
					// TODO: Insert SNARK proof here
					A:            nil,
					B:            nil,
					C:            nil,
					PublicInputs: nil,
				},
			},
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
				log.Warnf("mempool is full, waiting and retrying")
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
			return 0, fmt.Errorf("waiting for envelope height took longer than deadline, skipping")
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
			return 0, fmt.Errorf("checking nullifier time took more than deadline, skipping")
		}
	}

	return votingElapsedTime, nil
}

func (c *Client) CreateProcess(oracle *ethereum.SignKeys,
	entityID, censusRoot []byte,
	censusURI string,
	pid []byte,
	envelopeType *models.EnvelopeType,
	mode *models.ProcessMode,
	censusOrigin models.CensusOrigin,
	duration int,
	maxCensusSize uint64) (uint32, error) {

	var req api.APIrequest
	req.Method = "submitRawTx"
	block, err := c.GetCurrentBlock()
	if err != nil {
		return 0, err
	}
	if mode == nil {
		mode = &models.ProcessMode{AutoStart: true, Interruptible: true}
	}
	processData := &models.Process{
		EntityId:      entityID,
		CensusRoot:    censusRoot,
		CensusURI:     &censusURI,
		CensusOrigin:  censusOrigin,
		BlockCount:    uint32(duration),
		ProcessId:     pid,
		StartBlock:    block + 4,
		EnvelopeType:  envelopeType,
		Mode:          mode,
		Status:        models.ProcessStatus_READY,
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 8},
		MaxCensusSize: &maxCensusSize,
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

// CreateCensus creates a new census on the remote gateway and publishes it.
// Users public keys can be added using censusSigner (ethereum.SignKeys) or
// censusPubKeys (raw hex public keys).
func (c *Client) CreateCensus(signer *ethereum.SignKeys, censusSigners []*ethereum.SignKeys,
	censusPubKeys []string) (root []byte, uri string, _ error) {
	var req api.APIrequest

	// Create census
	log.Infof("Create census")
	req.Method = "addCensus"
	req.CensusType = models.Census_ARBO_BLAKE2B
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
	req.Digested = false
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
			claims = append(claims, pub)
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
