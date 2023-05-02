package ist

import (
	"encoding/binary"

	"go.vocdoni.io/dvote/crypto/ethereum"
)

func (c *Controller) storeToNoState(key, value []byte) error {
	tx, err := c.state.Store.BeginTx()
	if err != nil {
		return err
	}
	defer tx.Discard()
	if err := tx.NoState().Set(key, value); err != nil {
		return err
	}
	return tx.SaveWithoutCommit()
}

func (c *Controller) deleteFromNoState(key []byte) error {
	tx, err := c.state.Store.BeginTx()
	if err != nil {
		return err
	}
	defer tx.Discard()
	if err := tx.NoState().Delete(key); err != nil {
		return err
	}
	return tx.SaveWithoutCommit()
}

func (c *Controller) retrieveFromNoState(key []byte) ([]byte, error) {
	tx, err := c.state.Store.BeginTx()
	if err != nil {
		return nil, err
	}
	defer tx.Discard()
	return tx.NoState().Get(key)
}

// dbIndex returns the IST action index in the state database.
func dbIndex(height uint32) []byte {
	return binary.LittleEndian.AppendUint32([]byte(dbPrefix), height)
}

// dbResultsIndex returns the IST results index in the state database.
func dbResultsIndex(electionID []byte) []byte {
	return ethereum.HashRaw(append([]byte("results/"), electionID...))[:16]
}
