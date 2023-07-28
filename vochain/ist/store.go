package ist

import (
	"encoding/binary"
)

func (c *Controller) storeToNoState(key, value []byte) error {
	return c.state.NoState(true).Set(key, value)
}

func (c *Controller) deleteFromNoState(key []byte) error {
	return c.state.NoState(true).Delete(key)
}

func (c *Controller) retrieveFromNoState(key []byte) ([]byte, error) {
	return c.state.NoState(true).Get(key)
}

// dbIndex returns the IST action index in the state database.
func dbIndex(height uint32) []byte {
	return binary.LittleEndian.AppendUint32([]byte(dbPrefix), height)
}
