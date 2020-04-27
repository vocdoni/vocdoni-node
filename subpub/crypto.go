package subpub

import (
	"crypto/rand"
	"encoding/base64"
	"io"

	"gitlab.com/vocdoni/go-dvote/log"
	"golang.org/x/crypto/nacl/secretbox"
)

// encrypt using symetric key
func (ps *SubPub) encrypt(msg []byte) string {
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		log.Error(err)
		return ""
	}
	return base64.StdEncoding.EncodeToString(secretbox.Seal(nonce[:], msg, &nonce, &ps.GroupKey))
}

// decrypt using symetric key
func (ps *SubPub) decrypt(b64msg string) ([]byte, bool) {
	msg, err := base64.StdEncoding.DecodeString(b64msg)
	if err != nil {
		return nil, false
	}
	if len(msg) < 25 {
		return nil, false
	}
	var decryptNonce [24]byte
	copy(decryptNonce[:], msg[:24])
	return secretbox.Open(nil, msg[24:], &decryptNonce, &ps.GroupKey)
}
