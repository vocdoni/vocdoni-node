package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	flag "github.com/spf13/pflag"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/nacl"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/service"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

const (
	commitmentKeySize = 32
	MaxQuestions      = 64
	MaxOptions        = 64
)

type processKeys struct {
	pubKey        []byte
	privKey       []byte
	revealKey     []byte
	commitmentKey []byte
	index         int8
}

type ProcessVotes [][]uint32

func main() {
	log.Init("info", "stdout")

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	dev := flag.Bool("dev", false, "enable dev mode")
	dataDir := flag.String("dataDir", fmt.Sprintf("%s/.dvote", home), "datadir")
	oracles := flag.String("oracles", "", "comma separated list of oracleKey:index")
	pid := flag.String("pid", "", "process ID")
	flag.Parse()

	if *dev {
		*dataDir = *dataDir + "/dev"
	}

	vconfig := config.VochainCfg{
		Dev:         *dev,
		P2PListen:   "0.0.0.0:26656",
		RPCListen:   "127.0.0.1:26657",
		MempoolSize: 5000,
		DataDir:     *dataDir + "/vochain",
		LogLevel:    "error",
	}

	// Parse the oracle keys
	var keys []*processKeys
	log.Infof("importing oracle keys")
	for _, o := range strings.Split(*oracles, ",") {
		osp := strings.Split(o, ":")
		if len(osp) != 2 {
			log.Fatalf("oracle key malformed (%s)", o)
		}
		index, err := strconv.Atoi(osp[1])
		if err != nil {
			log.Fatal(err)
		}
		signer := ethereum.NewSignKeys()
		if err = signer.AddHexKey(osp[0]); err != nil {
			log.Fatal(err)
		}
		pk, err := generateKeys(*pid, int8(index), signer)
		if err != nil {
			log.Fatal(err)
		}
		keys = append(keys, pk)
		log.Infof("process key: %x", pk.privKey)
	}

	// Create Vochain service
	vnode, _, _, err := service.Vochain(&vconfig, vconfig.Dev, false, true, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		vnode.Node.Stop()
		vnode.Node.Wait()
	}()

	// Wait for Vochain to be ready
	var h, hPrev int64
	for vnode.Node == nil {
		hPrev = h
		time.Sleep(time.Second * 5)
		if header := vnode.State.Header(true); header != nil {
			h = header.Height
		}
		log.Infof("[vochain info] replaying block %d at %d b/s",
			h, (h-hPrev)/5)
	}

	process, err := vnode.State.Process(*pid, true)
	if err != nil {
		log.Fatal(err)
	}
	for _, k := range keys {
		process.EncryptionPrivateKeys[k.index] = fmt.Sprintf("%x", k.privKey)
	}

	log.Infof("computing results for %s", *pid)
	votes, err := computeNonLiveResults(*pid, process, vnode.State)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("results: %v", votes)
}

func generateKeys(pid string, index int8, signer *ethereum.SignKeys) (*processKeys, error) {
	// Generate keys
	pb, err := hex.DecodeString(pid)
	if err != nil {
		return nil, err
	}
	// Add the index in order to win some extra entropy
	pb = append(pb, byte(index))
	// Private ed25519 key
	priv, err := nacl.DecodePrivate(fmt.Sprintf("%x", ethereum.HashRaw(append(signer.Private.D.Bytes()[:], pb[:]...))))
	if err != nil {
		return nil, fmt.Errorf("cannot generate encryption key: (%s)", err)
	}
	// Reveal and commitment keys
	ckb := snarks.Poseidon.Hash(priv.Bytes())
	ck := ckb[:commitmentKeySize]
	ckhash := snarks.Poseidon.Hash(ckb)[:commitmentKeySize]

	pk := &processKeys{
		privKey:       priv.Bytes(),
		pubKey:        priv.Public().Bytes(),
		revealKey:     ck,
		commitmentKey: ckhash,
		index:         index,
	}
	return pk, nil
}

func emptyProcess() ProcessVotes {
	pv := make(ProcessVotes, MaxQuestions)
	for i := range pv {
		pv[i] = make([]uint32, MaxOptions)
	}
	return pv
}

func computeNonLiveResults(processID string, p *types.Process, s *vochain.State) (pv ProcessVotes, err error) {
	pv = emptyProcess()
	var nvotes int
	for _, e := range s.EnvelopeList(processID, 0, 32<<18, false) { // 8.3M seems enough for now
		v, err := s.Envelope(fmt.Sprintf("%s_%s", processID, e), false)
		if err != nil {
			log.Warn(err)
			continue
		}
		var vp *types.VotePackage
		err = nil
		if p.IsEncrypted() {
			if len(p.EncryptionPrivateKeys) < len(v.EncryptionKeyIndexes) {
				err = fmt.Errorf("encryptionKeyIndexes has too many fields")
			} else {
				keys := []string{}
				for _, k := range v.EncryptionKeyIndexes {
					if k >= types.MaxKeyIndex {
						err = fmt.Errorf("key index overflow")
						break
					}
					keys = append(keys, p.EncryptionPrivateKeys[k])
				}
				if len(keys) == 0 || err != nil {
					err = fmt.Errorf("no keys provided or wrong index")
				} else {
					vp, err = unmarshalVote(v.VotePackage, keys)
				}
			}
		} else {
			vp, err = unmarshalVote(v.VotePackage, []string{})
		}
		if err != nil {
			log.Warn(err)
			continue
		}
		for question, opt := range vp.Votes {
			if opt > MaxOptions {
				log.Warn("option overflow on computeResult, skipping vote...")
				continue
			}
			pv[question][opt]++
		}
		nvotes++
	}
	pruneVoteResult(&pv)
	log.Infof("computed results for process %s with %d votes", processID, nvotes)
	return
}

func unmarshalVote(votePackage string, keys []string) (*types.VotePackage, error) {
	rawVote, err := base64.StdEncoding.DecodeString(votePackage)
	if err != nil {
		return nil, err
	}
	var vote types.VotePackage
	// if encryption keys, decrypt the vote
	if len(keys) > 0 {
		for i := len(keys) - 1; i >= 0; i-- {
			priv, err := nacl.DecodePrivate(keys[i])
			if err != nil {
				log.Warnf("cannot create private key cipher: (%s)", err)
				continue
			}
			if rawVote, err = priv.Decrypt(rawVote); err != nil {
				log.Warnf("cannot decrypt vote with index key %d", i)
			}
		}
	}
	if err := json.Unmarshal(rawVote, &vote); err != nil {
		return nil, err
	}
	return &vote, nil
}

func pruneVoteResult(pv *ProcessVotes) {
	pvv := *pv
	var pvc ProcessVotes
	min := MaxQuestions - 1
	for ; min >= 0; min-- { // find the real size of first dimension (questions with some answer)
		j := 0
		for ; j < MaxOptions; j++ {
			if pvv[min][j] != 0 {
				break
			}
		}
		if j < MaxOptions {
			break
		} // we found a non-empty question, this is the min. Stop iteration.
	}

	for i := 0; i <= min; i++ { // copy the options for each question but pruning options too
		pvc = make([][]uint32, i+1)
		for i2 := 0; i2 <= i; i2++ { // copy only the first non-zero values
			j2 := MaxOptions - 1
			for ; j2 >= 0; j2-- {
				if pvv[i2][j2] != 0 {
					break
				}
			}
			pvc[i2] = make([]uint32, j2+1)
			copy(pvc[i2], pvv[i2])
		}
	}
	*pv = pvc
}
