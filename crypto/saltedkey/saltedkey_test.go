package saltedkey

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	blind "github.com/arnaucube/go-blindsecp256k1"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	qt "github.com/frankban/quicktest"
)

// testProcessID builds a 32 byte processID with the real layout:
// chainID[6] + orgAddr[20] + censusOrigin[1] + envType[1] + nonce[4].
// Two elections of the same organization on the same chain differ only in the
// nonce, which lives in the last 4 bytes.
func testProcessID(nonce uint32) []byte {
	pid := make([]byte, 32)
	copy(pid[0:6], []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}) // chainID
	for i := 6; i < 26; i++ {
		pid[i] = byte(i) // organization address
	}
	pid[26] = 0x03 // census origin
	pid[27] = 0x00 // envelope type
	pid[28] = byte(nonce >> 24)
	pid[29] = byte(nonce >> 16)
	pid[30] = byte(nonce >> 8)
	pid[31] = byte(nonce)
	return pid
}

func TestSaltDeterministic(t *testing.T) {
	pid := testProcessID(1)
	first, err := Salt(pid, big.NewInt(42).Bytes())
	qt.Assert(t, err, qt.IsNil)
	second, err := Salt(pid, big.NewInt(42).Bytes())
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, first, qt.DeepEquals, second)
	qt.Assert(t, len(first), qt.Equals, SaltSize)
}

// TestSaltDiffersPerProcessID is the regression test for issue #1424: two
// elections of the same organization on the same chain, differing only in the
// nonce, must derive different salts. It also pins the legacy behaviour that
// made them collide, so the fork's two sides stay documented in one place.
func TestSaltDiffersPerProcessID(t *testing.T) {
	pidA, pidB := testProcessID(1), testProcessID(2)
	qt.Assert(t, pidA, qt.Not(qt.DeepEquals), pidB)

	// legacy derivation: the raw processID cropped to SaltSize. The nonce lives
	// past byte 20, so both elections collide.
	qt.Assert(t, pidA[:SaltSize], qt.DeepEquals, pidB[:SaltSize])

	saltA, err := Salt(pidA, nil)
	qt.Assert(t, err, qt.IsNil)
	saltB, err := Salt(pidB, nil)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, saltA, qt.Not(qt.DeepEquals), saltB)

	// and the new derivation must not accidentally reproduce the legacy one
	qt.Assert(t, saltA, qt.Not(qt.DeepEquals), pidA[:SaltSize])
}

func TestSaltDiffersPerWeight(t *testing.T) {
	pid := testProcessID(1)
	low, err := Salt(pid, big.NewInt(5).Bytes())
	qt.Assert(t, err, qt.IsNil)
	high, err := Salt(pid, big.NewInt(1000).Bytes())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, low, qt.Not(qt.DeepEquals), high)

	// adjacent weights must differ too
	next, err := Salt(pid, big.NewInt(6).Bytes())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, low, qt.Not(qt.DeepEquals), next)
}

// TestSaltAbsentWeightIsOne checks that an absent voteWeight and a voteWeight of
// 1 derive the same salt: they already yield the same on-chain weight.
func TestSaltAbsentWeightIsOne(t *testing.T) {
	pid := testProcessID(1)
	absent, err := Salt(pid, nil)
	qt.Assert(t, err, qt.IsNil)
	one, err := Salt(pid, big.NewInt(1).Bytes())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, absent, qt.DeepEquals, one)
}

// TestSaltRejectsOverflow covers the MaxVoteWeightBits cap: a weight that does
// not fit is rejected rather than panicking FillBytes.
func TestSaltRejectsOverflow(t *testing.T) {
	pid := testProcessID(1)
	twoTo160 := new(big.Int).Lsh(big.NewInt(1), MaxVoteWeightBits)

	// the largest representable weight is accepted
	max := new(big.Int).Sub(twoTo160, big.NewInt(1))
	_, err := Salt(pid, max.Bytes())
	qt.Assert(t, err, qt.IsNil)

	// exactly 2^160 is rejected (161 bits)
	_, err = Salt(pid, twoTo160.Bytes())
	qt.Assert(t, err, qt.Equals, ErrVoteWeightOverflow)

	// and anything larger
	_, err = Salt(pid, new(big.Int).Add(big.NewInt(5), twoTo160).Bytes())
	qt.Assert(t, err, qt.Equals, ErrVoteWeightOverflow)
}

// TestSaltCanonicalWeight pins the fixed-width canonicalization: every byte
// encoding of the same integer must derive the same salt, so a voter cannot
// re-encode a weight to shift the salt while the chain records the same value.
func TestSaltCanonicalWeight(t *testing.T) {
	pid := testProcessID(1)

	oneByte, err := Salt(pid, []byte{0x05})
	qt.Assert(t, err, qt.IsNil)
	padded, err := Salt(pid, []byte{0x00, 0x00, 0x05})
	qt.Assert(t, err, qt.IsNil)
	fromInt, err := Salt(pid, big.NewInt(5).Bytes())
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, padded, qt.DeepEquals, oneByte)
	qt.Assert(t, fromInt, qt.DeepEquals, oneByte)

	// a genuinely different integer still differs
	other, err := Salt(pid, []byte{0x06})
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, other, qt.Not(qt.DeepEquals), oneByte)
}

func TestSaltECDSAKeyPairRoundtrip(t *testing.T) {
	key, err := ethcrypto.GenerateKey()
	qt.Assert(t, err, qt.IsNil)
	salt, err := Salt(testProcessID(1), big.NewInt(42).Bytes())
	qt.Assert(t, err, qt.IsNil)

	saltedPriv, err := SaltECDSAPrivKey(key, salt)
	qt.Assert(t, err, qt.IsNil)
	saltedPub, err := SaltECDSAPubKey(&key.PublicKey, salt)
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, saltedPriv.PublicKey.X.String(), qt.Equals, saltedPub.X.String())
	qt.Assert(t, saltedPriv.PublicKey.Y.String(), qt.Equals, saltedPub.Y.String())
}

func TestSaltBlindKeyPairRoundtrip(t *testing.T) {
	key, err := blind.NewPrivateKey()
	qt.Assert(t, err, qt.IsNil)
	salt, err := Salt(testProcessID(1), big.NewInt(42).Bytes())
	qt.Assert(t, err, qt.IsNil)

	saltedPriv, err := SaltBlindPrivKey(key, salt)
	qt.Assert(t, err, qt.IsNil)
	saltedPub, err := SaltBlindPubKey(key.Public(), salt)
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, saltedPriv.Public().X.String(), qt.Equals, saltedPub.X.String())
	qt.Assert(t, saltedPriv.Public().Y.String(), qt.Equals, saltedPub.Y.String())
}

func TestSaltECDSAPubKeyDoesNotMutateInput(t *testing.T) {
	key, err := ethcrypto.GenerateKey()
	qt.Assert(t, err, qt.IsNil)
	wantX, wantY := key.PublicKey.X.String(), key.PublicKey.Y.String()

	_, err = SaltECDSAPubKey(&key.PublicKey, testProcessID(1))
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, key.PublicKey.X.String(), qt.Equals, wantX)
	qt.Assert(t, key.PublicKey.Y.String(), qt.Equals, wantY)
}

// TestSaltPubKeyLegacyTruncation pins the pre-fork behaviour: the salting
// primitives consume only the first SaltSize bytes, so passing a full 32 byte
// processID is identical to passing its first 20. Elections that started before
// the fork rely on this staying exactly as it is.
func TestSaltPubKeyLegacyTruncation(t *testing.T) {
	key, err := ethcrypto.GenerateKey()
	qt.Assert(t, err, qt.IsNil)
	pid := testProcessID(1)

	full, err := SaltECDSAPubKey(&key.PublicKey, pid)
	qt.Assert(t, err, qt.IsNil)
	cropped, err := SaltECDSAPubKey(&key.PublicKey, pid[:SaltSize])
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, full.X.String(), qt.Equals, cropped.X.String())
	qt.Assert(t, full.Y.String(), qt.Equals, cropped.Y.String())
}

func TestSaltRejectsShortSalt(t *testing.T) {
	key, err := ethcrypto.GenerateKey()
	qt.Assert(t, err, qt.IsNil)
	short := make([]byte, SaltSize-1)

	_, err = SaltECDSAPubKey(&key.PublicKey, short)
	qt.Assert(t, err, qt.IsNotNil)
	_, err = SaltECDSAPrivKey(key, short)
	qt.Assert(t, err, qt.IsNotNil)

	blindKey, err := blind.NewPrivateKey()
	qt.Assert(t, err, qt.IsNil)
	_, err = SaltBlindPubKey(blindKey.Public(), short)
	qt.Assert(t, err, qt.IsNotNil)
	_, err = SaltBlindPrivKey(blindKey, short)
	qt.Assert(t, err, qt.IsNotNil)
}

// TestSaltNilKeys guards the nil paths, which the salted proof verifier reaches
// when a census root fails to parse.
func TestSaltNilKeys(t *testing.T) {
	salt, err := Salt(testProcessID(1), nil)
	qt.Assert(t, err, qt.IsNil)

	_, err = SaltECDSAPubKey(nil, salt)
	qt.Assert(t, err, qt.IsNotNil)
	_, err = SaltBlindPubKey(nil, salt)
	qt.Assert(t, err, qt.IsNotNil)
	_, err = SaltECDSAPrivKey((*ecdsa.PrivateKey)(nil), salt)
	qt.Assert(t, err, qt.IsNotNil)
	_, err = SaltBlindPrivKey(nil, salt)
	qt.Assert(t, err, qt.IsNotNil)
}
