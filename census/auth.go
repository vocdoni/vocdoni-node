package census

import (
	"encoding/json"
	"errors"
	"time"

	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

// CheckAuth check if a census request message is authorized
func (m *Manager) CheckAuth(rm *types.RequestMessage) error {
	if len(rm.Signature) < ethereum.SignatureLength || len(rm.CensusID) < 1 {
		return errors.New("signature or censusId not provided or invalid")
	}
	ns := new(Namespace)
	for _, n := range m.Census.Namespaces {
		if n.Name == rm.CensusID {
			ns = &n
		}
	}

	// Add root key, if method is addCensus
	if rm.Method == "addCensus" {
		if len(m.Census.RootKey) < ethereum.PubKeyLength {
			log.Warn("root key does not exist, considering addCensus valid for any request")
			return nil
		}
		ns.Keys = []string{m.Census.RootKey}
	}

	if ns == nil {
		return errors.New("censusId not valid")
	}

	// Check timestamp
	currentTime := int32(time.Now().Unix())
	if rm.Timestamp > currentTime+m.AuthWindow ||
		rm.Timestamp < currentTime-m.AuthWindow {
		return errors.New("timestamp is not valid")
	}

	// Check signature with existing namespace keys
	log.Debugf("namespace keys %s", ns.Keys)
	if len(ns.Keys) > 0 {
		if len(ns.Keys) == 1 && len(ns.Keys[0]) < ethereum.PubKeyLength {
			log.Warnf("namespace %s does have management public key configured, allowing all", ns.Name)
			return nil
		}
		valid := false
		msg, err := json.Marshal(rm.MetaRequest)
		if err != nil {
			return errors.New("cannot unmarshal")
		}
		for _, n := range ns.Keys {
			valid, err = ethereum.Verify(msg, rm.Signature, n)
			if err != nil {
				log.Warnf("verification error (%s)", err)
				valid = false
			} else if valid {
				return nil
			}
		}
		if !valid {
			return errors.New("unauthorized")
		}
	} else {
		log.Warnf("namespace %s does have management public key configured, allowing all", ns.Name)
	}
	return nil
}
