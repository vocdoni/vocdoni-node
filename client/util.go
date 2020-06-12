package client

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/nacl"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

func (c *APIConnection) WaitUntilBlock(block int64) {
	log.Infof("waiting for block %d...", block)
	for cb := c.GetCurrentBlock(); cb <= block; cb = c.GetCurrentBlock() {
		time.Sleep(10 * time.Second)
		log.Infof("remaining blocks: %d", block-cb)
	}
}

func CreateEthRandomKeysBatch(n int) []*ethereum.SignKeys {
	s := make([]*ethereum.SignKeys, n)
	for i := 0; i < n; i++ {
		s[i] = new(ethereum.SignKeys)
		if err := s[i].Generate(); err != nil {
			log.Fatal(err)
		}
	}
	return s
}

func RandomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

func genVote(encrypted bool, keys []string) (string, error) {
	vp := &types.VotePackage{
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
					return "", fmt.Errorf("cannot decode encryption key with index %d: (%s)", i, err)
				}
				if first {
					vp.Nonce = RandomHex(rand.Intn(16) + 16)
					vpBytes, err = json.Marshal(vp)
					if err != nil {
						return "", err
					}
					first = false
				}
				if vpBytes, err = nacl.Anonymous.Encrypt(vpBytes, pub); err != nil {
					return "", fmt.Errorf("cannot encrypt: (%s)", err)
				}
			}
		}
	} else {
		vpBytes, err = json.Marshal(vp)
		if err != nil {
			return "", err
		}

	}
	return base64.StdEncoding.EncodeToString(vpBytes), nil
}
