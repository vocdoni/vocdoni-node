package state

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNoStatePrefixedKey(t *testing.T) {
	c := qt.New(t)

	prefix := []byte("vocdoni_")
	key := "4627757194c9bc174ae21c004388aba722f49052449ca0db90920ab4447b8dba"
	c.Assert(key, qt.Equals, string(fromPrefixKey(prefix, toPrefixKey(prefix, []byte(key)))))
}
