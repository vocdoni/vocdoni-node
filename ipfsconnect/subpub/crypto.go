package subpub

import (
	"crypto/rand"
	"io"

	"go.vocdoni.io/dvote/log"
	"golang.org/x/crypto/nacl/secretbox"
)

// encrypt using symetric key
func (ps *SubPub) encrypt(msg []byte) []byte {
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		log.Error(err)
		return nil
	}
	return secretbox.Seal(nonce[:], msg, &nonce, &ps.GroupKey)
}

// decrypt using symetric key
func (ps *SubPub) decrypt(msg []byte) ([]byte, bool) {
	if msg == nil {
		return nil, false
	}
	var decryptNonce [24]byte
	copy(decryptNonce[:], msg[:24])
	return secretbox.Open(nil, msg[24:], &decryptNonce, &ps.GroupKey)
}
