// Package crypto contains cryptographic interfaces used in the vocdoni project.
// They are a simpler version of similar interfaces found in the standard
// library, such as hash.Hash and crypto.Signer.
//
// The implementations should all use crypto/rand.Reader as their source of
// secure randomness. Constructors are not covered by the interfaces, but
// KeyPair implementations will generally expose a top-level function of the
// form:
//
//    FromHex(privHex string) (*T, error)
//
// These interfaces are meant to encrypt, sign, or hash small chunks of bytes.
// Working with []byte directly can be simpler. However, be careful to not use
// chunks of bytes larger than a few megabytes, as that could significantly
// increase the memory and cpu overhead.
package crypto

// KeyPair is common to any public key cryptography algorithm, which at the
// moment are represented as Cipher or Signer.
type KeyPair interface {
	Public() []byte
	Private() []byte
}

// Cipher represents public key cryptography algorithm to encrypt and decrypt
// messages.
type Cipher interface {
	KeyPair

	Encrypt(message []byte) ([]byte, error)
	Decrypt(cipher []byte) ([]byte, error)
}

// Cipher represents public key cryptography algorithm to sign and verify
// messages.
type Signer interface {
	KeyPair

	Sign(message []byte) ([]byte, error)
	Verify(message, signature []byte) error
}

// Hash represents cryptographic hash algorihms.
type Hash interface {
	Hash(message []byte) []byte
}
