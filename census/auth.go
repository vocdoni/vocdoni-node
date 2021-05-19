package census

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

// CheckAuth check if a census request message is authorized
func (m *Manager) CheckAuth(reqOuter *api.RequestMessage, reqInner *api.MetaRequest) error {
	if len(reqOuter.Signature) < ethereum.SignatureLength || len(reqInner.CensusID) < 1 {
		return errors.New("signature or censusId not provided or invalid")
	}
	ns := new(Namespace)
	for _, n := range m.Census.Namespaces {
		if n.Name == reqInner.CensusID {
			*ns = n
			break
		}
	}

	if ns.Name == "" {
		return fmt.Errorf("censusId not valid")
	}

	// Add root key, if method is addCensus
	if reqInner.Method == "addCensus" {
		if len(m.Census.RootKey) < ethereum.PubKeyLengthBytes*2 {
			log.Warn("root key does not exist, considering addCensus valid for any request")
			return nil
		}
		ns.Keys = []string{m.Census.RootKey}
	}

	// Check timestamp
	currentTime := int32(time.Now().Unix())
	if reqInner.Timestamp > currentTime+m.AuthWindow ||
		reqInner.Timestamp < currentTime-m.AuthWindow {
		return fmt.Errorf("timestamp is not valid")
	}

	// Check signature with existing namespace keys
	log.Debugf("namespace keys %s", ns.Keys)
	if len(ns.Keys) > 0 {
		if len(ns.Keys) == 1 && len(ns.Keys[0]) < ethereum.PubKeyLengthBytes*2 {
			log.Warnf("namespace %s does not have management public key configured, allowing all", ns.Name)
			return nil
		}
		valid := false
		for _, keyHex := range ns.Keys {
			var err error
			key, err := hex.DecodeString(util.TrimHex(keyHex))
			if err != nil {
				return err
			}
			valid, err = ethereum.Verify(reqOuter.MetaRequest, reqOuter.Signature, key)
			if err != nil {
				log.Warnf("verification error (%s)", err)
				valid = false
			} else if valid {
				return nil
			}
		}
		if !valid {
			return fmt.Errorf("unauthorized")
		}
	} else {
		log.Warnf("namespace %s does now have management public key configured, allowing all", ns.Name)
	}
	return nil
}
