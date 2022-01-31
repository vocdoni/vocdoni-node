package client

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/artifacts"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
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
		return nil, fmt.Errorf("cannot get keys for process %x: %s", pid, resp.Message)
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

type Proof struct {
	Siblings []byte
	Value    []byte
}

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

// testGetZKCensusKey returns zkCensusKey, secretKey.  For testing purposes, we
// generate a secretKey from a signature by the ethereum key.
func testGetZKCensusKey(s *ethereum.SignKeys) ([]byte, []byte) {
	// secret is 65 bytes
	secret, err := s.SignVocdoniMsg([]byte("secretKey"))
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

type SNARKProofCircom struct {
	A []string   `json:"pi_a"`
	B [][]string `json:"pi_b"`
	C []string   `json:"pi_c"`
	// PublicInputs []string // onl
}

type SNARKProof struct {
	A            []string
	B            []string
	C            []string
	PublicInputs []string // only nullifier
}

type SNARKProofInputs struct {
	CensusRoot     string   `json:"censusRoot"`
	CensusSiblings []string `json:"censusSiblings"`
	Index          string   `json:"index"`
	SecretKey      string   `json:"secretKey"`
	VoteHash       []string `json:"voteHash"`
	ProcessID      []string `json:"processId"`
	Nullifier      string   `json:"nullifier"`
}

type GenSNARKData struct {
	CircuitIndex  int                      `json:"circuitIndex"`
	CircuitConfig *artifacts.CircuitConfig `json:"circuitConfig"`
	Inputs        SNARKProofInputs         `json:"inputs"`
}

func testGenSNARKProof(circuitIndex int, circuitConfig *artifacts.CircuitConfig,
	censusRoot, merkleProof []byte, treeSize int64,
	secretKey, votePackage, processId []byte) (*SNARKProof, []byte, error) {
	if len(merkleProof) < 8 {
		return nil, nil, fmt.Errorf("merkleProof too short")
	}
	indexLE := merkleProof[:8]
	index := binary.LittleEndian.Uint64(indexLE)
	siblingsBytes := merkleProof[8:]

	levels := int(math.Log2(float64(treeSize)))
	siblings, err := arbo.UnpackSiblings(arbo.HashFunctionPoseidon, siblingsBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot arbo.UnpackSiblings: %w", err)
	}
	for i := len(siblings); i < levels; i++ {
		siblings = append(siblings, []byte{0})
	}
	siblings = append(siblings, []byte{0})
	var siblingsStr []string
	for i := 0; i < len(siblings); i++ {
		siblingsStr = append(siblingsStr, arbo.BytesToBigInt(siblings[i]).String())
	}

	voteHash := sha256.Sum256(votePackage)
	voteHash0, voteHash1 := voteHash[:16], voteHash[16:]

	processId0, processId1 := processId[:16], processId[16:]
	poseidon := arbo.HashPoseidon{}
	nullifier, err := poseidon.Hash(
		secretKey,
		processId0,
		processId1,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("poseidon: %w", err)
	}

	inputs := SNARKProofInputs{
		CensusRoot:     arbo.BytesToBigInt(censusRoot).String(),
		CensusSiblings: siblingsStr,
		Index:          new(big.Int).SetUint64(index).String(),
		SecretKey:      arbo.BytesToBigInt(secretKey).String(),
		VoteHash: []string{
			arbo.BytesToBigInt(voteHash0).String(),
			arbo.BytesToBigInt(voteHash1).String(),
		},
		ProcessID: []string{
			arbo.BytesToBigInt(processId0).String(),
			arbo.BytesToBigInt(processId1).String(),
		},
		Nullifier: arbo.BytesToBigInt(nullifier).String(),
	}
	data := GenSNARKData{
		CircuitIndex:  circuitIndex,
		CircuitConfig: circuitConfig,
		Inputs:        inputs,
	}
	dataJSON, err := json.Marshal(&data)
	if err != nil {
		return nil, nil, err
	}
	log.Debugf("gen-vote-snark.js input: %v", string(dataJSON))
	cmd := exec.Command("node", "/app/js/gen-vote-snark.js", string(dataJSON))
	proofJSON, err := cmd.CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("node /app/js/gen-vote-snark.js: %w\n%s",
			err, string(proofJSON))
	}
	log.Debugf("gen-vote-snark.js output: %v", string(proofJSON))
	var proofCircom SNARKProofCircom
	if err := json.Unmarshal(proofJSON, &proofCircom); err != nil {
		return nil, nil, fmt.Errorf("/app/js/gen-vote-snark.js output unmarshal: %w\n%s",
			err, string(proofJSON))
	}
	return &SNARKProof{
		A: proofCircom.A,
		B: []string{
			proofCircom.B[0][0], proofCircom.B[0][1],
			proofCircom.B[1][0], proofCircom.B[1][1],
			proofCircom.B[2][0], proofCircom.B[2][1],
		},
		C: proofCircom.C,
		// PublicInputs is not used in the anonymous-voting flow, as
		// the only needed public input from user's side is the
		// nullifier, which is already in the VoteEnvelope struct
	}, nullifier, nil
}

func (c *Client) TestPreRegisterKeys(
	pid,
	eid,
	root []byte,
	startBlock uint32,
	signers []*ethereum.SignKeys,
	censusOrigin models.CensusOrigin,
	caSigner *ethereum.SignKeys,
	proofs []*Proof,
	doublePreRegister, checkNullifiers bool,
	wg *sync.WaitGroup) (time.Duration, error) {

	var err error
	registerKeyWeight := "1"
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
			Weight:    registerKeyWeight,
		}
		switch censusOrigin {
		case models.CensusOrigin_OFF_CHAIN_TREE:
			v.Proof = &models.Proof{
				Payload: &models.Proof_Arbo{
					Arbo: &models.ProofArbo{
						Type:     models.ProofArbo_BLAKE2B,
						Siblings: proofs[i].Siblings,
						Value:    proofs[i].Value,
					},
				},
			}

		case models.CensusOrigin_OFF_CHAIN_CA:
			p := &models.ProofCA{}
			if err := proto.Unmarshal(proofs[i].Siblings, p); err != nil {
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

		if stx.Signature, err = s.SignVocdoniTx(stx.Tx); err != nil {
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
				// FIXME: once the req.Message are no longer in hex, remove this
				msg, err := hex.DecodeString(resp.Message)
				if err != nil {
					return 0, fmt.Errorf("resp.Message hex decode: %w", err)
				}
				return 0, fmt.Errorf("%s failed: %s", req.Method, msg)
			}
		}
		if (i+1)%100 == 0 {
			log.Infof("pre-register progress for %s: %d%%", c.Addr, ((i+1)*100)/(len(signers)))
		}

		// Try double preRegister.  The request will not fail but
		// that's OK.  For each user we will have 2 pre-register Txs in
		// the pool, only one of them will succeed (the first one
		// assuming that the node includes the transactions in the
		// mempool received order).  Later on we check that the Rolling
		// Census Size has the expected size, so we verify that there
		// were no more pre-registers than expected.  And finally we
		// vote with the first zkCensusKey for each user, making sure
		// those pre-register succeded (and thus the second ones
		// failed).
		if doublePreRegister {
			// We change the key in order to submit a different Tx
			// with the same pre-census proof.
			v.NewKey[1] = ^v.NewKey[1]
			stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_RegisterKey{RegisterKey: v}})
			if err != nil {
				return 0, err
			}
			if stx.Signature, err = s.SignVocdoniTx(stx.Tx); err != nil {
				return 0, err
			}
			if req.Payload, err = proto.Marshal(stx); err != nil {
				return 0, err
			}
			resp, err := c.Request(req, nil)
			if err != nil {
				return 0, err
			}
			// log.Warnf("DBG Double: %#v", resp)
			if resp.Ok {
				continue
				// We can't detect double pre-register here yet
				// because we're not caching the RegisterKeyTx
				// verification.  Nevertheless when the block
				// is mined, only one pre-register will
				// succeed.
				// Uncomment once we have a pre-regsiter
				// verification cache.
				// return 0, fmt.Errorf("double pre-register not detected")
			}
		}
	}

	tries := 10
	log.Infof("checking first pre-registered key")
	for ; tries >= 0; tries-- {
		weight, err := c.GetRollingCensusVoterWeight(pid, signers[0].Address())
		if err != nil {
			return 0, fmt.Errorf("the pre-register key cannot be verified: %w", err)
		}
		if weight.String() == registerKeyWeight {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if tries == 0 {
		return 0, fmt.Errorf("could not get pre-register key")
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
	proofs []*Proof,
	encrypted, doubleVoting, checkNullifiers bool,
	wg *sync.WaitGroup) (time.Duration, error) {

	var err error
	var keys []string
	// Generate merkle proofs
	if proofs == nil {
		switch censusOrigin {
		case models.CensusOrigin_OFF_CHAIN_TREE, models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED:
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
		case models.CensusOrigin_OFF_CHAIN_TREE, models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED:
			v.Proof = &models.Proof{
				Payload: &models.Proof_Arbo{
					Arbo: &models.ProofArbo{
						Type:     models.ProofArbo_BLAKE2B,
						Siblings: proofs[i].Siblings,
						Value:    proofs[i].Value,
					},
				},
			}

		case models.CensusOrigin_OFF_CHAIN_CA:
			p := &models.ProofCA{}
			if err := proto.Unmarshal(proofs[i].Siblings, p); err != nil {
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

		if stx.Signature, err = s.SignVocdoniTx(stx.Tx); err != nil {
			return 0, err
		}
		if req.Payload, err = proto.Marshal(stx); err != nil {
			return 0, err
		}
		pub, _ := s.HexString()
		log.Debugf("voting with pubKey:%s {%s}", pub, log.FormatProto(v))
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
	log.Infof("CircuitIndex: %v, CircuitPath: %+v", *circuitIndex, circuitConfig.CircuitPath)
	// Wait until all gateway connections are ready
	wg.Done()
	log.Infof("%s is waiting other gateways to be ready before it can start voting", c.Addr)
	c.WaitUntilBlock(startBlock)
	wg.Wait()

	log.Infof("Downloading Circuit Artifacts")
	circuitConfig.LocalDir = "/tmp"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := artifacts.DownloadCircuitFiles(ctx, *circuitConfig); err != nil {
		return 0, err
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
		if vpb, err = genVote(false, nil); err != nil {
			return 0, err
		}
		_, secretKey := testGetZKCensusKey(s)
		proof, nullifier, err := testGenSNARKProof(*circuitIndex, circuitConfig,
			root, proofs[i].Siblings, circuitConfig.Parameters[0], secretKey, vpb, pid)
		if err != nil {
			return 0, fmt.Errorf("cannot generate test SNARK proof: %w", err)
		}
		v := &models.VoteEnvelope{
			Nonce:       util.RandomBytes(32),
			ProcessId:   pid,
			VotePackage: vpb,
			Nullifier:   nullifier,
		}
		v.Proof = &models.Proof{
			Payload: &models.Proof_ZkSnark{
				ZkSnark: &models.ProofZkSNARK{
					CircuitParametersIndex: int32(*circuitIndex),
					A:                      proof.A,
					B:                      proof.B,
					C:                      proof.C,
					// PublicInputs is not used in the
					// anonymous-voting flow, as the only
					// needed public input from user's side
					// is the nullifier, which is already
					// in the VoteEnvelope struct
				},
			},
		}

		stx := &models.SignedTx{}
		stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: v}})
		if err != nil {
			return 0, err
		}

		if stx.Signature, err = s.SignVocdoniTx(stx.Tx); err != nil {
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
				// FIXME: once the req.Message are no longer in hex, remove this
				msg, err := hex.DecodeString(resp.Message)
				if err != nil {
					return 0, fmt.Errorf("resp.Message hex decode: %w", err)
				}
				return 0, fmt.Errorf("%s failed: %s", req.Method, msg)
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
				return 0, fmt.Errorf("double voting not detected")
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

func (c *Client) CreateAccount(signer *ethereum.SignKeys, infoURI string, nonce uint32) error {
	var req api.APIrequest
	var err error
	req.Method = "submitRawTx"

	tx := &models.SetAccountInfoTx{
		Txtype:  models.TxType_SET_ACCOUNT_INFO,
		Nonce:   nonce,
		InfoURI: infoURI,
		Account: signer.Address().Bytes(),
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

func (c *Client) SetAccountInfoURI(signer *ethereum.SignKeys, to common.Address, infoURI string, nonce uint32) error {
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
	return acc, nil
}

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

func (c *Client) CreateProcess(signer *ethereum.SignKeys,
	entityID, censusRoot []byte,
	censusURI string,
	pid []byte,
	envelopeType *models.EnvelopeType,
	mode *models.ProcessMode,
	censusOrigin models.CensusOrigin,
	startBlockIncrement int,
	duration int,
	maxCensusSize uint64) (uint32, error) {

	startBlock := uint32(0)
	if startBlockIncrement > 0 {
		current, err := c.GetCurrentBlock()
		if err != nil {
			return 0, err
		}
		startBlock = current + uint32(startBlockIncrement)
	}

	var req api.APIrequest
	req.Method = "submitRawTx"
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
		StartBlock:    startBlock,
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
	var err error
	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_NewProcess{NewProcess: p}})
	if err != nil {
		return 0, err
	}
	if stx.Signature, err = signer.SignVocdoniTx(stx.Tx); err != nil {
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
	if startBlockIncrement == 0 {
		for i := 0; i < 10; i++ {
			time.Sleep(2 * time.Second)
			p, err := c.GetProcessInfo(pid)
			if err == nil && p != nil {
				return p.StartBlock, nil
			}
		}
		return 0, fmt.Errorf("process was not created")
	}
	return startBlock, nil
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
// censusValues can be empty for a non-weighted census
func (c *Client) CreateCensus(signer *ethereum.SignKeys, censusSigners []*ethereum.SignKeys,
	censusPubKeys []string, censusValues []*types.BigInt) (root []byte, uri string, _ error) {
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
	if len(censusValues) > 0 && len(censusValues) != censusSize {
		return nil, "", fmt.Errorf("census keys and values are not the same lenght (%d != %d)",
			censusSize, len(censusValues))
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
		values := []*types.BigInt{}
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
			if len(censusValues) > 0 {
				values = append(values, censusValues[currentSize-1])
			}
			currentSize--
		}
		req.CensusKeys = claims
		req.Weights = values
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
	req.CensusKeys = nil
	req.Weights = nil

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
