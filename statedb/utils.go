package statedb

import "encoding/binary"

// SetUint64 stores an uint64 in little endian under the key.
func SetUint64(u Updater, key []byte, n uint64) error {
	value := [8]byte{}
	binary.LittleEndian.PutUint64(value[:], n)
	return u.Set(key, value[:])
}

// GetUint64 gets an uint64 from little endian under the key.
func GetUint64(v Viewer, key []byte) (uint64, error) {
	value, err := v.Get(key)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(value), nil
}
