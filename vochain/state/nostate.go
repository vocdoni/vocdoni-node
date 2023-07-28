package state

import "go.vocdoni.io/dvote/db"

// NoState is a wrapper around the state database that allows to write and
// read data without affecting the state hash.
//
// It is safe to use concurrently also with other NoState instances if
// withTxLock parameter is set to true.
// Be aware that the NoState operations use the state.Tx mutex, so if the
// lock is acquired while calling NoState and withTxLock is set to true,
// it will deadlock.
//
// The NoState transaction is committed or discarted with the state
// transaction at Save() or Rollback().
func (s *State) NoState(withTxLock bool) *NoState {
	return &NoState{
		state:      s,
		withTxLock: withTxLock,
	}
}

// NoState is a wrapper around the state database for nostate operations.
type NoState struct {
	state      *State
	withTxLock bool
}

// Set sets a key-value pair in the nostate database.
func (ns *NoState) Set(key, value []byte) error {
	if ns.withTxLock {
		ns.state.tx.Lock()
		defer ns.state.tx.Unlock()
	}
	return ns.state.store.NoStateWriteTx.Set(key, value)
}

// Get retrieves a value from the nostate database.
func (ns *NoState) Get(key []byte) ([]byte, error) {
	if ns.withTxLock {
		ns.state.tx.RLock()
		defer ns.state.tx.RUnlock()
	}
	return ns.state.store.NoStateReadTx.Get(key)
}

// Has returns true if the nostate database contains the given key.
func (ns *NoState) Has(key []byte) (bool, error) {
	if ns.withTxLock {
		ns.state.tx.RLock()
		defer ns.state.tx.RUnlock()
	}
	if _, err := ns.state.store.NoStateReadTx.Get(key); err != nil {
		if err == db.ErrKeyNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delete deletes a key-value pair from the nostate database.
func (ns *NoState) Delete(key []byte) error {
	if ns.withTxLock {
		ns.state.tx.Lock()
		defer ns.state.tx.Unlock()
	}
	return ns.state.store.NoStateWriteTx.Delete(key)
}

// Iterate iterates over all the nostate database keys with the given prefix.
func (ns *NoState) Iterate(prefix []byte, callback func(k, v []byte) bool) error {
	if ns.withTxLock {
		ns.state.tx.RLock()
		defer ns.state.tx.RUnlock()
	}
	return ns.state.store.NoStateReadTx.Iterate(prefix, callback)
}
