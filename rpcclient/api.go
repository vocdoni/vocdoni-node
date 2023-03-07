package rpcclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/log"
	api "go.vocdoni.io/dvote/rpctypes"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/state"
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

func (c *Client) GetResults(pid []byte) ([][]*types.BigInt, string, bool, error) {
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

func (c *Client) GetCircuitConfig(pid []byte) (*circuit.ZkCircuitConfig, error) {
	var req api.APIrequest
	req.Method = "getProcessCircuitConfig"
	req.ProcessID = pid
	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("cannot get circuitConfig for process %x: %s", pid, resp.Message)
	}
	return resp.CircuitConfig, nil
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
	return resp.Weight.MathBigInt(), nil
}

func (c *Client) TestResults(pid []byte, totalVotes int, withWeight uint64) (results [][]*types.BigInt, err error) {
	log.Infof("waiting for results...")
	var final bool
	for {
		c.WaitUntilNextBlock()
		results, _, final, err = c.GetResults(pid)
		if err != nil {
			return nil, err
		}
		if final {
			break
		}
		log.Infof("no results yet")
	}
	total := new(types.BigInt).SetUint64(uint64(totalVotes) * withWeight)
	if !results[0][1].Equal(total) ||
		!results[1][2].Equal(total) ||
		!results[2][3].Equal(total) ||
		!results[3][4].Equal(total) {
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
		log.Fatalf("Cannot calculate pre-register key with Poseidon: %v", err)
	}
	pubKey, err := hasher.Hash(secretKey)
	if err != nil {
		log.Fatalf("Cannot calculate pre-register key with Poseidon: %v", err)
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
	CircuitConfig *circuit.ZkCircuitConfig `json:"circuitConfig"`
	Inputs        SNARKProofInputs         `json:"inputs"`
}

func testGenSNARKProof(circuit *circuit.ZkCircuit,
	censusRoot, merkleProof []byte, privateKey, votePackage,
	processId []byte, weight *big.Int) (*prover.Proof, []byte, error) {
	if len(merkleProof) < 8 {
		return nil, nil, fmt.Errorf("merkleProof too short")
	}

	siblingsBytes := merkleProof[8:]
	levels := circuit.Config.Levels
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

	key := babyjub.PrivateKey{}
	_, err = hex.Decode(key[:], privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating babyjub key")
	}

	voteHash := sha256.Sum256(votePackage)
	strVoteHash := []string{
		new(big.Int).SetBytes(arbo.SwapEndianness(voteHash[:16])).String(),
		new(big.Int).SetBytes(arbo.SwapEndianness(voteHash[16:])).String(),
	}

	intProcessId := []*big.Int{
		new(big.Int).SetBytes(arbo.SwapEndianness(processId[:16])),
		new(big.Int).SetBytes(arbo.SwapEndianness(processId[16:])),
	}
	strProcessId := []string{intProcessId[0].String(), intProcessId[1].String()}

	nullifier, err := poseidon.Hash([]*big.Int{
		babyjub.SkToBigInt(&key),
		intProcessId[0],
		intProcessId[1],
	})
	if err != nil {
		return nil, nil, fmt.Errorf("poseidon: %w", err)
	}
	strNullifier := nullifier.String()

	inputs, err := json.Marshal(map[string]interface{}{
		"censusRoot":     arbo.BytesToBigInt(censusRoot).String(),
		"censusSiblings": siblingsStr,
		"weight":         weight.String(),
		"privateKey":     babyjub.SkToBigInt(&key).String(),
		"voteHash":       strVoteHash,
		"processId":      strProcessId,
		"nullifier":      strNullifier,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error encoding prover inputs: %w", err)
	}

	proof, err := prover.Prove(circuit.ProvingKey, circuit.Wasm, inputs)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating the proof: %w", err)
	}

	return proof, nullifier.Bytes(), nil
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
	_, err = c.WaitUntilProcessAvailable(pid)
	if err != nil {
		log.Fatal(err)
	}

	cb, err := c.GetCurrentBlock()
	if err != nil {
		return 0, err
	}
	log.Infof("Current block: %v", cb)

	// Send votes
	log.Infof("sending pre-register keys")
	timeDeadLine := time.Second * 400
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
			Nonce:     uint32(util.RandomInt(0, 1000000)), // TODO: @jordipainan change register key for account based tx
			ProcessId: pid,
			NewKey:    zkCensusKey,
			Weight:    registerKeyWeight,
		}
		switch censusOrigin {
		case models.CensusOrigin_OFF_CHAIN_TREE:
			v.Proof = &models.Proof{
				Payload: &models.Proof_Arbo{
					Arbo: &models.ProofArbo{
						Type:       models.ProofArbo_BLAKE2B,
						Siblings:   proofs[i].Siblings,
						LeafWeight: proofs[i].Value,
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
			log.Fatalf("censusOrigin %s not supported", censusOrigin.String())
		}

		stx := &models.SignedTx{}
		stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_RegisterKey{RegisterKey: v}})
		if err != nil {
			return 0, err
		}

		chainID, err := c.GetChainID()
		if err != nil {
			return 0, err
		}
		if stx.Signature, err = s.SignVocdoniTx(stx.Tx, chainID); err != nil {
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
			chainID, err := c.GetChainID()
			if err != nil {
				return 0, err
			}
			if stx.Signature, err = s.SignVocdoniTx(stx.Tx, chainID); err != nil {
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
		c.WaitUntilNextBlock()
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
	wg.Wait()

	// Wait for process to actually start
	_, err = c.WaitUntilProcessReady(pid)
	if err != nil {
		return 0, err
	}

	// Get encryption keys
	keyIndexes := []uint32{}
	if encrypted {
		keys, keyIndexes, err = c.WaitUntilProcessKeys(pid, eid)
		if err != nil {
			return 0, err
		}
	}

	// Send votes
	log.Infof("sending votes")
	timeDeadLine := time.Second * 400
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
						Type:       models.ProofArbo_BLAKE2B,
						Siblings:   proofs[i].Siblings,
						LeafWeight: proofs[i].Value,
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
			log.Fatalf("censusOrigin %s not supported", censusOrigin.String())
		}

		stx := &models.SignedTx{}
		stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: v}})
		if err != nil {
			return 0, err
		}
		chainID, err := c.GetChainID()
		if err != nil {
			return 0, err
		}
		if stx.Signature, err = s.SignVocdoniTx(stx.Tx, chainID); err != nil {
			return 0, err
		}
		if req.Payload, err = proto.Marshal(stx); err != nil {
			return 0, err
		}
		pub, _ := s.HexString()
		log.Debugf("voting with pubKey:%s {%s}", pub, log.FormatProto(v))
		resp, err := c.Request(req, nil)
		if err != nil {
			if strings.HasSuffix(err.Error(), "EOF") {
				// not fatal, just try again
				log.Warn(err)
				time.Sleep(1 * time.Second)
				i--
				continue
			}
			return 0, err
		}
		if !resp.Ok {
			if strings.Contains(resp.Message, "mempool is full") {
				log.Warnf("mempool is full, waiting and retrying")
				time.Sleep(1 * time.Second)
				i--
				continue
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
		h, err := c.GetEnvelopeHeight(pid)
		if err != nil {
			log.Warnf("error getting envelope height: %v", err)
			continue
		}
		if h >= uint32(len(signers)) {
			break
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
	circuitConfig, err := c.GetCircuitConfig(pid)
	if err != nil {
		return 0, err
	}
	log.Infof("CircuitPath: %+v", circuitConfig.CircuitPath)
	// Wait until all gateway connections are ready
	wg.Done()
	log.Infof("%s is waiting other gateways to be ready before it can start voting", c.Addr)
	wg.Wait()

	// Wait for process to actually start
	_, err = c.WaitUntilProcessReady(pid)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Downloading Circuit Artifacts")
	circuitConfig.LocalDir = "/tmp"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	circuit, err := circuit.LoadZkCircuit(ctx, *circuitConfig)
	if err != nil {
		return 0, err
	}

	// Send votes
	log.Infof("sending votes")
	timeDeadLine := time.Second * 400
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
		weight := new(big.Int).SetInt64(10)
		proof, nullifier, err := testGenSNARKProof(circuit, root,
			proofs[i].Siblings, secretKey, vpb, pid, weight)
		if err != nil {
			return 0, fmt.Errorf("cannot generate test SNARK proof: %w", err)
		}
		v := &models.VoteEnvelope{
			Nonce:       util.RandomBytes(32),
			ProcessId:   pid,
			VotePackage: vpb,
			Nullifier:   nullifier,
		}

		model, err := zk.ProverProofToProtobufZKProof(proof, v.ProcessId, root, v.Nullifier, weight)
		if err != nil {
			return 0, fmt.Errorf("error encoding the proof to protobuf: %w", err)
		}
		v.Proof = &models.Proof{
			Payload: &models.Proof_ZkSnark{ZkSnark: model},
		}

		stx := &models.SignedTx{}
		stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: v}})
		if err != nil {
			return 0, err
		}
		chainID, err := c.GetChainID()
		if err != nil {
			return 0, err
		}
		if stx.Signature, err = s.SignVocdoniTx(stx.Tx, chainID); err != nil {
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

	err = c.WaitUntilEnvelopeHeight(pid, uint32(len(signers)), timeDeadLine)
	if err != nil {
		return 0, err
	}

	votingElapsedTime := time.Since(start)

	if !checkNullifiers {
		return votingElapsedTime, nil
	}
	// If checkNullifiers, wait until all votes have been verified
	checkStart := time.Now()
	registered := 0
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

// CreateProcess creates a process, returning the resulting startBlock
// and the processID assigned
func (c *Client) CreateProcess(
	account *ethereum.SignKeys,
	entityID, censusRoot []byte,
	censusURI string,
	envelopeType *models.EnvelopeType,
	mode *models.ProcessMode,
	censusOrigin models.CensusOrigin,
	startBlockIncrement int,
	duration int,
	maxCensusSize uint64,
) (startBlock uint32, processID []byte, err error) {
	// if envelopeType.EncryptedVotes is true, process keys are going to be generated
	// and need to be added in a separate tx BEFORE the process is started.
	// a few more blocks avoid a race condition https://github.com/vocdoni/vocdoni-node/issues/299
	if startBlockIncrement < 5 && envelopeType.EncryptedVotes {
		startBlockIncrement = 5
	}

	if startBlockIncrement > 0 {
		current, err := c.GetCurrentBlock()
		if err != nil {
			return 0, nil, err
		}
		startBlock = current + uint32(startBlockIncrement)
	}
	//log.Warn(c.GetCurrentBlock()) //debug

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
		StartBlock:    startBlock,
		EnvelopeType:  envelopeType,
		Mode:          mode,
		Status:        models.ProcessStatus_READY,
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 8},
		MaxCensusSize: &maxCensusSize,
	}
	// get oracle account
	acc, err := c.GetAccount(account.Address())
	if err != nil {
		return 0, nil, fmt.Errorf("cannot get account")
	}
	if acc == nil {
		return 0, nil, state.ErrAccountNotExist
	}
	p := &models.NewProcessTx{
		Txtype:  models.TxType_NEW_PROCESS,
		Nonce:   acc.Nonce,
		Process: processData,
	}
	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_NewProcess{NewProcess: p}})
	if err != nil {
		return 0, nil, err
	}
	chainID, err := c.GetChainID()
	if err != nil {
		return 0, nil, err
	}
	if stx.Signature, err = account.SignVocdoniTx(stx.Tx, chainID); err != nil {
		return 0, nil, err
	}
	if req.Payload, err = proto.Marshal(stx); err != nil {
		return 0, nil, err
	}

	resp, err := c.Request(req, nil)
	if err != nil {
		return 0, nil, err
	}
	if !resp.Ok {
		return 0, nil, fmt.Errorf("%s failed: %s", req.Method, resp.Message)
	}
	processID, err = hex.DecodeString(resp.Payload)
	if err != nil {
		return 0, nil, fmt.Errorf("cannot decode process ID: %x", resp.Payload)
	}
	if startBlockIncrement == 0 {
		p, err := c.WaitUntilProcessAvailable(processID)
		if err != nil || p == nil {
			return 0, nil, fmt.Errorf("process not created: %w", err)
		}
		return p.StartBlock, processID, nil
	}
	return startBlock, processID, nil
}

// SetProcessStatus changes the status of a process using the oracle account
func (c *Client) SetProcessStatus(
	oracle *ethereum.SignKeys,
	pid []byte,
	status string,
) (err error) {
	s, ok := models.ProcessStatus_value[status]
	if !ok {
		return fmt.Errorf("invalid process status specified - refer to vochain.pb.go:ProcessStatus_name for valid statuses")
	}
	statusInt := models.ProcessStatus(s)
	var req api.APIrequest
	req.Method = "submitRawTx"
	// get oracle account
	acc, err := c.GetAccount(oracle.Address())
	if err != nil {
		return fmt.Errorf("cannot get account")
	}
	if acc == nil {
		return state.ErrAccountNotExist
	}
	p := &models.SetProcessTx{
		Txtype:    models.TxType_SET_PROCESS_STATUS,
		ProcessId: pid,
		Status:    &statusInt,
		Nonce:     acc.Nonce,
	}
	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetProcess{SetProcess: p}})
	if err != nil {
		return err
	}
	chainID, err := c.GetChainID()
	if err != nil {
		return err
	}
	if stx.Signature, err = oracle.SignVocdoniTx(stx.Tx, chainID); err != nil {
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

// EndProcess ends a process identified by pid, using the oracle account
func (c *Client) EndProcess(
	oracle *ethereum.SignKeys,
	pid []byte,
) (err error) {
	return c.SetProcessStatus(oracle, pid, "ENDED")
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

// GetTreasurer returns information about the treasurer
func (c *Client) GetTreasurer() (*models.Treasurer, error) {
	req := api.APIrequest{Method: "getTreasurer"}
	resp, err := c.Request(req, nil)
	treasurer := &models.Treasurer{}
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("cannot not get treasurer: %s", resp.Message)
	}
	if resp.EntityID != "" {
		treasurer.Address = common.HexToAddress(resp.EntityID).Bytes()
	}
	if resp.Nonce != nil {
		treasurer.Nonce = *resp.Nonce
	}
	return treasurer, nil
}

// GetTreasurer returns information about the treasurer
func (c *Client) GetTransactionCost(txType models.TxType) (uint64, error) {
	req := api.APIrequest{Method: "getTxCost", Type: vochain.TxTypeToCostName(txType)}
	resp, err := c.Request(req, nil)
	if err != nil {
		return 0, err
	}
	if !resp.Ok {
		return 0, fmt.Errorf("cannot get tx cost: %s", resp.Message)
	}
	return *resp.Amount, nil
}

// SetTransactionCost sets the transaction cost of a given transaction
// and returns the txHash of the transaction, or nil and the error
func (c *Client) SetTransactionCost(
	signer *ethereum.SignKeys,
	txType models.TxType,
	cost uint64,
	nonce uint32,
) (txHash types.HexBytes, err error) {
	tx := &models.SetTransactionCostsTx{
		Txtype: txType,
		Nonce:  nonce,
		Value:  cost,
	}
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetTransactionCosts{SetTransactionCosts: tx}})
	if err != nil {
		return nil, err
	}
	resp, err := c.SubmitRawTx(signer, &stx)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("submitRawTx failed: %s", resp.Message)
	}
	return resp.Hash, nil
}

// CreateOrSetAccount creates or sets the infoURI of a Vochain account
func (c *Client) SetAccount(
	signer *ethereum.SignKeys,
	to common.Address,
	infoURI string,
	nonce uint32,
	faucetPkg *models.FaucetPackage,
	create bool,
) (err error) {
	var req api.APIrequest
	req.Method = "submitRawTx"

	tx := &models.SetAccountTx{
		Txtype:        models.TxType_SET_ACCOUNT_INFO_URI,
		Nonce:         &nonce,
		InfoURI:       &infoURI,
		Account:       to.Bytes(),
		FaucetPackage: faucetPkg,
	}
	if create {
		tx.Txtype = models.TxType_CREATE_ACCOUNT
	}

	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccount{SetAccount: tx}})
	if err != nil {
		return err
	}
	resp, err := c.SubmitRawTx(signer, &stx)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("submitRawTx failed: %s", resp.Message)
	}
	return nil
}

// GetAccount returns information of a given account
func (c *Client) GetAccount(accountAddr common.Address) (*state.Account, error) {
	req := api.APIrequest{Method: "getAccount", EntityId: accountAddr.Bytes()}
	resp, err := c.Request(req, nil)
	acc := &state.Account{}
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

// GenerateFaucetPackage generates a faucet package
func (*Client) GenerateFaucetPackage(from *ethereum.SignKeys, to common.Address, value uint64) (*models.FaucetPackage, error) {
	return vochain.GenerateFaucetPackage(from, to, value)
}

// SubmitRawTx signs and sends a vochain transaction
func (c *Client) SubmitRawTx(signer *ethereum.SignKeys, stx *models.SignedTx) (*api.APIresponse, error) {
	var err error
	var req api.APIrequest
	req.Method = "submitRawTx"
	chainID, err := c.GetChainID()
	if err != nil {
		return nil, err
	}
	if stx.Signature, err = signer.SignVocdoniTx(stx.Tx, chainID); err != nil {
		return nil, err
	}
	if req.Payload, err = proto.Marshal(stx); err != nil {
		return nil, err
	}
	return c.Request(req, nil)
}

// GetTxByHash looks up a transaction given its hash
func (c *Client) GetTxByHash(txhash types.HexBytes) (*indexertypes.TxPackage, error) {
	req := api.APIrequest{Method: "getTxByHash", Hash: txhash}
	resp, err := c.Request(req, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("could not get tx: %s", resp.Message)
	}
	return resp.Tx, nil
}

// MintTokens sends a mint tokens transaction
func (c *Client) MintTokens(
	treasurer *ethereum.SignKeys,
	accountAddr common.Address,
	treasurerNonce uint32,
	amount uint64,
) (err error) {
	var req api.APIrequest
	req.Method = "submitRawTx"

	tx := &models.MintTokensTx{
		Txtype: models.TxType_MINT_TOKENS,
		Nonce:  treasurerNonce,
		To:     accountAddr.Bytes(),
		Value:  amount,
	}

	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_MintTokens{MintTokens: tx}})
	if err != nil {
		return err
	}
	resp, err := c.SubmitRawTx(treasurer, &stx)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("submitRawTx failed: %s", resp.Message)
	}
	return nil
}

// SendTokens sends a send tokens transaction
// and returns the txHash of the transaction, or nil and the error
func (c *Client) SendTokens(
	signer *ethereum.SignKeys,
	accountAddr common.Address,
	nonce uint32,
	amount uint64,
) (txHash types.HexBytes, err error) {
	var req api.APIrequest
	req.Method = "submitRawTx"

	tx := &models.SendTokensTx{
		Txtype: models.TxType_SEND_TOKENS,
		From:   signer.Address().Bytes(),
		Nonce:  nonce,
		To:     accountAddr.Bytes(),
		Value:  amount,
	}

	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SendTokens{SendTokens: tx}})
	if err != nil {
		return nil, err
	}
	resp, err := c.SubmitRawTx(signer, &stx)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("submitRawTx failed: %s", resp.Message)
	}
	return resp.Hash, nil
}

// SetAccountDelegate sends a set delegate transaction, if op == true adds a delegate, deletes a delegate otherwise
// and returns the txHash of the transaction, or nil and the error
func (c *Client) SetAccountDelegate(
	signer *ethereum.SignKeys,
	delegate common.Address,
	op bool,
	nonce uint32,
) (txHash types.HexBytes, err error) {
	var req api.APIrequest
	req.Method = "submitRawTx"

	tx := &models.SetAccountTx{
		Txtype:    models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
		Nonce:     &nonce,
		Delegates: [][]byte{delegate.Bytes()},
	}
	if !op {
		tx.Txtype = models.TxType_DEL_DELEGATE_FOR_ACCOUNT
	}

	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccount{SetAccount: tx}})
	if err != nil {
		return nil, err
	}
	resp, err := c.SubmitRawTx(signer, &stx)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("submitRawTx failed: %s", resp.Message)
	}
	return resp.Hash, nil
}

// CollectFaucet sends a collect faucet transaction
// and returns the txHash of the transaction, or nil and the error
func (c *Client) CollectFaucet(
	signer *ethereum.SignKeys,
	nonce uint32,
	faucetPkg *models.FaucetPackage,
) (txHash types.HexBytes, err error) {
	var req api.APIrequest
	req.Method = "submitRawTx"

	tx := &models.CollectFaucetTx{
		FaucetPackage: faucetPkg,
		Nonce:         nonce,
		TxType:        models.TxType_COLLECT_FAUCET,
	}

	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_CollectFaucet{CollectFaucet: tx}})
	if err != nil {
		return nil, err
	}
	resp, err := c.SubmitRawTx(signer, &stx)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("submitRawTx failed: %s", resp.Message)
	}
	return resp.Hash, nil
}

// SetOracle sends an Add/Remove Oracle transaction, if op == false -> remove, else add
func (c *Client) SetOracle(
	treasurer *ethereum.SignKeys,
	oracleAddress common.Address,
	treasurerNonce uint32,
	op bool,
) (err error) {
	var req api.APIrequest
	req.Method = "submitRawTx"

	tx := &models.AdminTx{
		Txtype:  models.TxType_ADD_ORACLE,
		Nonce:   treasurerNonce,
		Address: oracleAddress.Bytes(),
	}

	if !op {
		tx.Txtype = models.TxType_REMOVE_ORACLE
	}

	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Admin{Admin: tx}})
	if err != nil {
		return err
	}
	resp, err := c.SubmitRawTx(treasurer, &stx)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("submitRawTx failed: %s", resp.Message)
	}
	return nil
}
