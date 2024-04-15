package ist

// this whole legacy.go code can be removed once LTS 1.2 chain is upgraded

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"

	"go.vocdoni.io/dvote/log"
)

// legacyActions was the model used (until 1a5a56e "vochain: adapt components and tests to timestamp based blockchain")
// to store the list of IST actions for a specific height into state.
type legacyActions map[string]legacyAction

// legacyAction is the model used to store the IST actions into state.
type legacyAction struct {
	ID                ActionTypeID
	ElectionID        []byte
	Attempts          uint32
	ValidatorVotes    [][]byte
	ValidatorProposer []byte
}

// encode performs the encoding of the IST action using Gob.
func (m *legacyActions) encode() []byte {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(m); err != nil {
		// should never happen
		panic(err)
	}
	return buf.Bytes()
}

// decode performs the decoding of the IST action using Gob.
func (m *legacyActions) decode(data []byte) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(m)
}

// parseLegacyActions will parse legacy ist actions and return them in a slice
func (c *Controller) parseLegacyActions(key, value []byte) ([]*Action, error) {
	// try to decode it using the old format
	lacts := &legacyActions{}
	if err := lacts.decode(value); err != nil {
		return nil, err
	}

	acts := []*Action{}
	for idStr, lact := range *lacts {
		acts = append(acts, &Action{
			Height:            legacyHeight(key),
			ID:                []byte(idStr),
			TypeID:            lact.ID,
			ElectionID:        lact.ElectionID,
			Attempts:          lact.Attempts,
			ValidatorVotes:    lact.ValidatorVotes,
			ValidatorProposer: lact.ValidatorProposer,
		})
	}
	return acts, nil
}

// handleLegacyActionsKVs will parse legacy actions, migrate them to the new format in NoStateDB and also return all of them as a list
func (c *Controller) handleLegacyActionsKVs(kvs map[string][]byte) []*Action {
	allActs := []*Action{}
	for keyStr, value := range kvs {
		key := []byte(keyStr)
		// try to decode it using the old format
		log.Warnf("ist action key %x decoding failed, assuming contains legacy ist action(s) scheduled for height %d, will try to parse and migrate", key, legacyHeight(key))
		acts, err := c.parseLegacyActions(key, value)
		if err != nil {
			log.Errorw(err, fmt.Sprintf("couldn't parse legacy ist action(s) (key: %x, value: %x)", key, value))
			continue
		}
		for _, act := range acts {
			err := c.state.NoState(true).Set(addPrefix(act.ID), act.encode())
			if err != nil {
				log.Errorw(err, fmt.Sprintf("couldn't migrate legacy ist action (%+v)", act))
			} else {
				log.Debugf("legacy ist action migrated, rescheduled using new format (%+v)", act)
			}
		}
		allActs = append(allActs, acts...)
		log.Debugf("legacy ist action(s) migrated, deleting key %x", addPrefix(key)) // debug
		if err := c.state.NoState(true).Delete(addPrefix(key)); err != nil {
			log.Errorw(err, fmt.Sprintf("error deleting legacy ist action(s) (key: %x)", addPrefix(key)))
		}
	}
	return allActs
}

// legacyDBIndex returns the IST action index in the state database.
func legacyDBIndex(height uint32) []byte {
	return binary.LittleEndian.AppendUint32([]byte(dbPrefix), height)
}

func legacyHeight(key []byte) uint32 {
	return binary.LittleEndian.Uint32(key)
}
