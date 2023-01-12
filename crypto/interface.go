// Package crypto contains cryptographic interfaces used in the vocdoni project.
// They are a simpler version of similar interfaces found in the standard
// library, such as hash.Hash and crypto.Signer.
//
// The implementations should all use crypto/rand.Reader as their source of
// secure randomness. Constructors are not covered by the interfaces, but
// KeyPair implementations will generally expose a top-level function of the
// form:
//
//	FromHex(privHex string) (*T, error)
//
// These interfaces are meant to encrypt, sign, or hash small chunks of bytes.
// Working with []byte directly can be simpler. However, be careful to not use
// chunks of bytes larger than a few megabytes, as that could significantly
// increase the memory and cpu overhead.
package crypto

// PublicKey represents a single public key.
type PublicKey interface {
	// Bytes represents a public key as a byte slice, which can be useful to
	// then print it in hex or base64.
	Bytes() []byte
}

// PrivateKey represents a single private key, along with its derived public
// key. Note that the public key is derived when the private key is created.
type PrivateKey interface {
	// Bytes represents a private key as a byte slice, which can be useful to
	// then print it in hex or base64.
	Bytes() []byte

	// Public returns the public key that was derived from this private key.
	Public() PublicKey
}

// Cipher represents a private key which can encrypt and decrypt messages.
type Cipher interface {
	PrivateKey

	// Encrypt encrypts message for a given recipient. If recipient is nil,
	// the message is encrypted for the encryption key itself.
	Encrypt(message []byte, recipient PublicKey) ([]byte, error)

	// Decrypt decryptes a message, assuming that it was encrypted for this
	// key.
	Decrypt(cipher []byte) ([]byte, error)
}

// Cipher represents a private key which can sign messages.
type Signer interface {
	PrivateKey

	Sign(message []byte) ([]byte, error)
}

// Verifier represents a public key which can verify signed messages.
type Verifier interface {
	PublicKey

	Verify(message, signature []byte) error
}

// Hash represents cryptographic hash algorihms.
type Hash interface {
	Hash(message []byte) []byte
}
