package arbo

import (
	"math/big"
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db/metadb"
)

func TestCheckProofBatch(t *testing.T) {
	database := metadb.NewTest(t)
	c := qt.New(t)

	keyLen := 1
	maxLevels := keyLen * 8
	tree, err := NewTree(Config{
		Database: database, MaxLevels: maxLevels,
		HashFunction: HashFunctionBlake3,
	})
	c.Assert(err, qt.IsNil)

	censusRoot := []byte("01234567890123456789012345678901")
	ballotMode := []byte("1234")

	err = tree.Add(BigIntToBytesLE(keyLen, big.NewInt(0x01)), censusRoot)
	c.Assert(err, qt.IsNil)

	err = tree.Add(BigIntToBytesLE(keyLen, big.NewInt(0x02)), ballotMode)
	c.Assert(err, qt.IsNil)

	var oldProofs, newProofs []*CircomVerifierProof

	for i := int64(0x00); i <= int64(0x04); i++ {
		proof, err := tree.GenerateCircomVerifierProof(BigIntToBytesLE(keyLen, big.NewInt(i)))
		c.Assert(err, qt.IsNil)
		oldProofs = append(oldProofs, proof)
	}

	censusRoot[0] = byte(0x02)
	ballotMode[0] = byte(0x02)

	err = tree.Update(BigIntToBytesLE(keyLen, big.NewInt(0x01)), censusRoot)
	c.Assert(err, qt.IsNil)

	err = tree.Update(BigIntToBytesLE(keyLen, big.NewInt(0x02)), ballotMode)
	c.Assert(err, qt.IsNil)

	err = tree.Add(BigIntToBytesLE(keyLen, big.NewInt(0x03)), ballotMode)
	c.Assert(err, qt.IsNil)

	for i := int64(0x00); i <= int64(0x04); i++ {
		proof, err := tree.GenerateCircomVerifierProof(BigIntToBytesLE(keyLen, big.NewInt(i)))
		c.Assert(err, qt.IsNil)
		newProofs = append(newProofs, proof)
	}

	// passing all proofs should be OK:
	// proof 1 + 2 + 3 are required
	// proof 0 and 4 are of unchanged keys, but the new siblings are explained by the other proofs
	err = CheckProofBatch(HashFunctionBlake3, oldProofs, newProofs)
	c.Assert(err, qt.IsNil)

	// omitting proof 0 and 4 (unchanged keys) should also be OK
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[1:4], newProofs[1:4])
	c.Assert(err, qt.IsNil)

	// providing an empty batch should not pass
	err = CheckProofBatch(HashFunctionBlake3, []*CircomVerifierProof{}, []*CircomVerifierProof{})
	c.Assert(err, qt.ErrorMatches, "empty batch")

	// length mismatch
	err = CheckProofBatch(HashFunctionBlake3, oldProofs, newProofs[:1])
	c.Assert(err, qt.ErrorMatches, "batch of proofs incomplete")

	// providing just proof 0 (unchanged key) should not pass since siblings can't be explained
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[:1], newProofs[:1])
	c.Assert(err, qt.ErrorMatches, ".*changed but there's no proof why.*")

	// providing just proof 0 (unchanged key) and an add, should fail
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[:1], newProofs[3:4])
	c.Assert(err, qt.ErrorMatches, ".*changed but there's no proof why.*")

	// omitting proof 3 should fail (since changed siblings in other proofs can't be explained)
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[:3], newProofs[:3])
	c.Assert(err, qt.ErrorMatches, ".*changed but there's no proof why.*")

	// the next 4 are mangling proofs to simulate other unexplained changes in the tree, all of these should fail
	badProofs := deepClone(oldProofs)
	badProofs[0].Root = []byte("01234567890123456789012345678900")
	err = CheckProofBatch(HashFunctionBlake3, badProofs, newProofs)
	c.Assert(err, qt.ErrorMatches, "old proof invalid: root doesn't match")

	badProofs = deepClone(oldProofs)
	badProofs[0].Siblings[0] = []byte("01234567890123456789012345678900")
	err = CheckProofBatch(HashFunctionBlake3, badProofs, newProofs)
	c.Assert(err, qt.ErrorMatches, "old proof invalid: root doesn't match")

	badProofs = deepClone(newProofs)
	badProofs[0].Root = []byte("01234567890123456789012345678900")
	err = CheckProofBatch(HashFunctionBlake3, oldProofs, badProofs)
	c.Assert(err, qt.ErrorMatches, "new proof invalid: root doesn't match")

	badProofs = deepClone(newProofs)
	badProofs[0].Siblings[0] = []byte("01234567890123456789012345678900")
	err = CheckProofBatch(HashFunctionBlake3, oldProofs, badProofs)
	c.Assert(err, qt.ErrorMatches, "new proof invalid: root doesn't match")

	// also test exclusion proofs:
	// exclusion proof of key 0x04 can't be used to prove exclusion of 0x01, 0x03 or 0x05 obviously
	badProofs = deepClone(oldProofs)
	badProofs[4].Key = []byte{0x01}
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[4:], badProofs[4:])
	c.Assert(err, qt.ErrorMatches, "new proof invalid: root doesn't match")
	badProofs[4].Key = []byte{0x03}
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[4:], badProofs[4:])
	c.Assert(err, qt.ErrorMatches, "new proof invalid: root doesn't match")
	badProofs[4].Key = []byte{0x05}
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[4:], badProofs[4:])
	c.Assert(err, qt.ErrorMatches, "new proof invalid: root doesn't match")
	// also can't prove key 0x02 exclusion (since that leaf exists and is indeed the starting point of the proof)
	badProofs[4].Key = []byte{0x02}
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[4:], badProofs[4:])
	c.Assert(err, qt.ErrorMatches, "new proof invalid: exclusion proof invalid, key and oldKey are equal")
	// but exclusion proof of key 0x04 can also prove exclusion of the whole prefix (0x00, 0x08, 0x0c, 0x10, etc)
	badProofs[4].Key = []byte{0x00}
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[4:], badProofs[4:])
	c.Assert(err, qt.IsNil)
	badProofs[4].Key = []byte{0x08}
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[4:], badProofs[4:])
	c.Assert(err, qt.IsNil)
	badProofs[4].Key = []byte{0x0c}
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[4:], badProofs[4:])
	c.Assert(err, qt.IsNil)
	badProofs[4].Key = []byte{0x10}
	err = CheckProofBatch(HashFunctionBlake3, oldProofs[4:], badProofs[4:])
	c.Assert(err, qt.IsNil)
}

func deepClone(src []*CircomVerifierProof) []*CircomVerifierProof {
	dst := slices.Clone(src)
	for i := range src {
		proof := *src[i]
		dst[i] = &proof

		dst[i].Siblings = slices.Clone(src[i].Siblings)
	}
	return dst
}
