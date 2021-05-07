package client

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
)

func (c *Client) WaitUntilBlock(block uint32) {
	log.Infof("waiting for block %d...", block)
	for {
		cb, err := c.GetCurrentBlock()
		if err != nil {
			log.Error(err)
			time.Sleep(5 * time.Second)
			continue
		}
		if cb >= block {
			break
		}
		time.Sleep(5 * time.Second)
		log.Infof("remaining blocks: %d", block-cb)
	}
}

func CreateEthRandomKeysBatch(n int) []*ethereum.SignKeys {
	s := make([]*ethereum.SignKeys, n)
	for i := 0; i < n; i++ {
		s[i] = ethereum.NewSignKeys()
		if err := s[i].Generate(); err != nil {
			log.Fatal(err)
		}
	}
	return s
}

type keysBatch struct {
	Keys      []signKey      `json:"keys"`
	CensusID  types.HexBytes `json:"censusId"`
	CensusURI string         `json:"censusUri"`
}

type signKey struct {
	PrivKey string `json:"privKey"`
	PubKey  string `json:"pubKey"`
	Proof   []byte `json:"proof"`
}

func SaveKeysBatch(filepath string, censusID []byte, censusURI string, keys []*ethereum.SignKeys, proofs [][]byte) error {
	if proofs != nil && (len(proofs) != len(keys)) {
		return fmt.Errorf("length of Proof is different from length of Signers")
	}
	var kb keysBatch
	for i, k := range keys {
		pub, priv := k.HexString()
		if proofs != nil {
			kb.Keys = append(kb.Keys, signKey{PrivKey: priv, PubKey: pub, Proof: proofs[i]})
		} else {
			kb.Keys = append(kb.Keys, signKey{PrivKey: priv, PubKey: pub})
		}
	}
	kb.CensusID = censusID
	kb.CensusURI = censusURI
	j, err := json.Marshal(kb)
	if err != nil {
		return err
	}
	log.Infof("saved census cache file has %d bytes, got %d keys", len(j), len(keys))
	return os.WriteFile(filepath, j, 0o644)
}

func LoadKeysBatch(filepath string) ([]*ethereum.SignKeys, [][]byte, []byte, string, error) {
	jb, err := os.ReadFile(filepath)
	if err != nil {
		return nil, nil, nil, "", err
	}

	var kb keysBatch
	if err = json.Unmarshal(jb, &kb); err != nil {
		return nil, nil, nil, "", err
	}

	if len(kb.Keys) == 0 || len(kb.CensusID) == 0 || kb.CensusURI == "" {
		return nil, nil, nil, "", fmt.Errorf("keybatch file is empty or missing data")
	}

	keys := make([]*ethereum.SignKeys, len(kb.Keys))
	proofs := [][]byte{}
	for i, k := range kb.Keys {
		s := ethereum.NewSignKeys()
		if err = s.AddHexKey(k.PrivKey); err != nil {
			return nil, nil, nil, "", err
		}
		proofs = append(proofs, k.Proof)
		keys[i] = s
	}
	return keys, proofs, kb.CensusID, kb.CensusURI, nil
}

func Random(n int) []byte {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return bytes
}

func RandomHex(n int) string {
	return hex.EncodeToString(Random(n))
}

func genVote(encrypted bool, keys []string) ([]byte, error) {
	vp := &indexertypes.VotePackage{
		Votes: []int{1, 2, 3, 4, 5, 6},
	}
	var vpBytes []byte
	var err error
	if encrypted {
		first := true
		for i, k := range keys {
			if len(k) > 0 {
				log.Debugf("encrypting with key %s", k)
				pub, err := nacl.DecodePublic(k)
				if err != nil {
					return nil, fmt.Errorf("cannot decode encryption key with index %d: (%s)", i, err)
				}
				if first {
					vp.Nonce = RandomHex(rand.Intn(16) + 16)
					vpBytes, err = json.Marshal(vp)
					if err != nil {
						return nil, err
					}
					first = false
				}
				if vpBytes, err = nacl.Anonymous.Encrypt(vpBytes, pub); err != nil {
					return nil, fmt.Errorf("cannot encrypt: (%s)", err)
				}
			}
		}
	} else {
		vpBytes, err = json.Marshal(vp)
		if err != nil {
			return nil, err
		}

	}
	return vpBytes, nil
}
