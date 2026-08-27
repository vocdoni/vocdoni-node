package cspproof

import (
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/saltedkey"
	"go.vocdoni.io/dvote/test/testcommon/testcsp"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

// legacyOrigin and v2Origin are the two census origins the CSP verifier serves.
// Legacy keeps the raw-processID salt; V2 derives it with saltedkey.Salt.
const (
	legacyOrigin = models.CensusOrigin_OFF_CHAIN_CA
	v2Origin     = models.CensusOrigin_OFF_CHAIN_CA_V2
)

var bothOrigins = []models.CensusOrigin{legacyOrigin, v2Origin}

var saltedTypes = []models.ProofCA_Type{
	models.ProofCA_ECDSA_PIDSALTED,
	models.ProofCA_ECDSA_BLIND_PIDSALTED,
}

// siblingPIDs returns two processIDs of the same organization on the same chain,
// differing only in the nonce. Their first 20 bytes — all the legacy salt
// consumed — are identical, which is the root of issue #1424.
func siblingPIDs(t *testing.T) (pidA, pidB []byte) {
	t.Helper()
	pidA = util.RandomBytes(32)
	pidB = append([]byte{}, pidA...)
	pidB[31] ^= 0xff // bump the nonce, which lives in bytes 28..31
	qt.Assert(t, pidA[:saltedkey.SaltSize], qt.DeepEquals, pidB[:saltedkey.SaltSize])
	return pidA, pidB
}

func newProcess(pid, censusRoot []byte, origin models.CensusOrigin) *models.Process {
	return &models.Process{
		ProcessId:    pid,
		CensusRoot:   censusRoot,
		CensusOrigin: origin,
		EnvelopeType: &models.EnvelopeType{},
		Status:       models.ProcessStatus_READY,
	}
}

// verify runs the CSP verifier as the census origin of process selects it,
// mirroring the dispatch in vochain/transaction.VerifyProof.
func verify(process *models.Process, pid []byte,
	proof *models.Proof, voter *ethereum.SignKeys,
) (bool, *big.Int, error) {
	verifier := &ProofVerifierCSP{V2: process.CensusOrigin == v2Origin}
	return verifier.Verify(process,
		&models.VoteEnvelope{ProcessId: pid, Proof: proof},
		state.NewVoterID(state.VoterIDTypeECDSA, voter.PublicKey()))
}

// newVoter returns a fresh voter key.
func newVoter(t *testing.T) *ethereum.SignKeys {
	t.Helper()
	k := ethereum.NewSignKeys()
	qt.Assert(t, k.Generate(), qt.IsNil)
	return k
}

// expectedSalt is the salt the verifier derives for pid/weight on an election
// of the given census origin.
func expectedSalt(t *testing.T, origin models.CensusOrigin, pid []byte, weight *big.Int) []byte {
	t.Helper()
	if origin == v2Origin {
		var weightBytes []byte
		if weight != nil {
			weightBytes = weight.Bytes()
		}
		salt, err := saltedkey.Salt(pid, weightBytes)
		qt.Assert(t, err, qt.IsNil)
		return salt
	}
	return pid // legacy: raw processID, cropped to 20 bytes by the primitives
}

// TestCSPProofValid is the baseline: for every proof type, on both census
// origins, a proof salted the way the verifier expects must verify and return
// the declared weight.
func TestCSPProofValid(t *testing.T) {
	allTypes := []models.ProofCA_Type{
		models.ProofCA_ECDSA,
		models.ProofCA_ECDSA_PIDSALTED,
		models.ProofCA_ECDSA_BLIND,
		models.ProofCA_ECDSA_BLIND_PIDSALTED,
	}
	for _, proofType := range allTypes {
		for _, origin := range bothOrigins {
			t.Run(proofType.String()+"/"+origin.String(), func(t *testing.T) {
				signer, err := testcsp.NewSigner()
				qt.Assert(t, err, qt.IsNil)
				root, err := signer.CensusRoot(proofType)
				qt.Assert(t, err, qt.IsNil)

				pid := util.RandomBytes(32)
				voter := newVoter(t)
				weight := big.NewInt(42)
				bundle := testcsp.Bundle(pid, voter.Address().Bytes(), weight)

				proof, err := signer.SignProof(proofType, bundle, expectedSalt(t, origin, pid, weight))
				qt.Assert(t, err, qt.IsNil)

				valid, gotWeight, err := verify(newProcess(pid, root, origin), pid, proof, voter)
				qt.Assert(t, err, qt.IsNil)
				qt.Assert(t, valid, qt.IsTrue)
				qt.Assert(t, gotWeight.String(), qt.Equals, weight.String())
			})
		}
	}
}

// TestCSPCrossElectionRejected is the regression test for the core of issue
// #1424. The CSP authorizes election A; the voter presents a bundle for its
// sibling election B. On the legacy origin both elections derive the same
// salted key, so the proof verifies on B — in the blind flow, where the CSP
// cannot read the message it signs, that is a forgery. On the V2 origin it is
// rejected.
func TestCSPCrossElectionRejected(t *testing.T) {
	for _, proofType := range saltedTypes {
		t.Run(proofType.String(), func(t *testing.T) {
			signer, err := testcsp.NewSigner()
			qt.Assert(t, err, qt.IsNil)
			root, err := signer.CensusRoot(proofType)
			qt.Assert(t, err, qt.IsNil)

			pidA, pidB := siblingPIDs(t)
			voter := newVoter(t)
			weight := big.NewInt(42)
			// the bundle names election B, but the CSP salted for election A
			bundle := testcsp.Bundle(pidB, voter.Address().Bytes(), weight)

			t.Run("legacy accepts it (the vulnerability)", func(t *testing.T) {
				proof, err := signer.SignProof(proofType, bundle, expectedSalt(t, legacyOrigin, pidA, weight))
				qt.Assert(t, err, qt.IsNil)
				valid, _, err := verify(newProcess(pidB, root, legacyOrigin), pidB, proof, voter)
				qt.Assert(t, err, qt.IsNil)
				qt.Assert(t, valid, qt.IsTrue)
			})

			t.Run("v2 rejects it", func(t *testing.T) {
				proof, err := signer.SignProof(proofType, bundle, expectedSalt(t, v2Origin, pidA, weight))
				qt.Assert(t, err, qt.IsNil)
				valid, _, err := verify(newProcess(pidB, root, v2Origin), pidB, proof, voter)
				qt.Assert(t, err, qt.IsNotNil)
				qt.Assert(t, valid, qt.IsFalse)
			})
		})
	}
}

// TestCSPAdaptiveWeightForgeryRejected is the regression test for the adaptive
// forgery that an additive derivation (salt = keccak256(pid)[:20] + w) would
// allow. The attacker holds a blind signature the CSP made for election A
// weight 1, and picks the weight that, under addition, would make election B
// derive the same salt: w' = (saltA - keccak256(pidB)[:20]) mod 2^160. Against
// the hashed derivation the salts do not coincide and the proof is rejected; if
// the derivation were ever changed to additive this test would fail, because
// the forged proof would verify.
//
// Unlike TestCSPCrossElectionRejected, the attacker here chooses the weight
// *after* seeing the salt — the only case that distinguishes a real binding from
// a malleable one.
func TestCSPAdaptiveWeightForgeryRejected(t *testing.T) {
	for _, proofType := range saltedTypes {
		t.Run(proofType.String(), func(t *testing.T) {
			signer, err := testcsp.NewSigner()
			qt.Assert(t, err, qt.IsNil)
			root, err := signer.CensusRoot(proofType)
			qt.Assert(t, err, qt.IsNil)

			// unrelated elections: the attacker only needs them to share the CSP
			// root, so random pids (not siblings) are the general case
			pidA, pidB := util.RandomBytes(32), util.RandomBytes(32)

			// the CSP authorizes the voter on election A for weight 1
			saltA, err := saltedkey.Salt(pidA, big.NewInt(1).Bytes())
			qt.Assert(t, err, qt.IsNil)

			// solve for the weight that maps election B onto saltA under an
			// additive derivation
			mod := new(big.Int).Lsh(big.NewInt(1), saltedkey.MaxVoteWeightBits)
			hB := new(big.Int).SetBytes(ethereum.HashRaw(pidB)[:saltedkey.SaltSize])
			forged := new(big.Int).Mod(new(big.Int).Sub(new(big.Int).SetBytes(saltA), hB), mod)

			voter := newVoter(t)
			proof, err := signer.SignProof(proofType,
				testcsp.Bundle(pidB, voter.Address().Bytes(), forged), saltA)
			qt.Assert(t, err, qt.IsNil)

			valid, _, err := verify(newProcess(pidB, root, v2Origin), pidB, proof, voter)
			qt.Assert(t, err, qt.IsNotNil)
			qt.Assert(t, valid, qt.IsFalse)
		})
	}
}

// TestCSPWeightBinding covers the second half of the issue: the CSP authorizes
// weight 5, the voter declares 1000. On the legacy origin the weight is outside
// the salt, so the chain records whatever the voter asked for. On the V2 origin
// the voter signs under a key the chain does not derive and the proof fails.
func TestCSPWeightBinding(t *testing.T) {
	const authorized, declared = 5, 1000
	for _, proofType := range saltedTypes {
		t.Run(proofType.String(), func(t *testing.T) {
			signer, err := testcsp.NewSigner()
			qt.Assert(t, err, qt.IsNil)
			root, err := signer.CensusRoot(proofType)
			qt.Assert(t, err, qt.IsNil)

			pid := util.RandomBytes(32)
			voter := newVoter(t)
			bundle := testcsp.Bundle(pid, voter.Address().Bytes(), big.NewInt(declared))

			t.Run("legacy accepts the inflated weight", func(t *testing.T) {
				proof, err := signer.SignProof(proofType, bundle, expectedSalt(t, legacyOrigin, pid, nil))
				qt.Assert(t, err, qt.IsNil)
				valid, weight, err := verify(newProcess(pid, root, legacyOrigin), pid, proof, voter)
				qt.Assert(t, err, qt.IsNil)
				qt.Assert(t, valid, qt.IsTrue)
				qt.Assert(t, weight.Int64(), qt.Equals, int64(declared))
			})

			t.Run("v2 rejects it", func(t *testing.T) {
				// CSP salts for the weight it authorized, not the one declared
				proof, err := signer.SignProof(proofType, bundle,
					expectedSalt(t, v2Origin, pid, big.NewInt(authorized)))
				qt.Assert(t, err, qt.IsNil)
				valid, _, err := verify(newProcess(pid, root, v2Origin), pid, proof, voter)
				qt.Assert(t, err, qt.IsNotNil)
				qt.Assert(t, valid, qt.IsFalse)
			})

			t.Run("v2 accepts the authorized weight", func(t *testing.T) {
				honest := testcsp.Bundle(pid, voter.Address().Bytes(), big.NewInt(authorized))
				proof, err := signer.SignProof(proofType, honest,
					expectedSalt(t, v2Origin, pid, big.NewInt(authorized)))
				qt.Assert(t, err, qt.IsNil)
				valid, weight, err := verify(newProcess(pid, root, v2Origin), pid, proof, voter)
				qt.Assert(t, err, qt.IsNil)
				qt.Assert(t, valid, qt.IsTrue)
				qt.Assert(t, weight.Int64(), qt.Equals, int64(authorized))
			})
		})
	}
}

// TestCSPWeightOverflowRejected covers the MaxVoteWeightBits guard: a sanity
// cap that keeps FillBytes from panicking on a weight wider than its fixed
// 32-byte encoding (see the Salt doc in crypto/saltedkey). It is not an
// anti-wraparound measure — the fixed-width encoding already makes w and
// w + 2^160 distinct preimages with distinct salts. The guard lives in the V2
// derivation, so the legacy path is unchanged.
func TestCSPWeightOverflowRejected(t *testing.T) {
	twoTo160 := new(big.Int).Lsh(big.NewInt(1), saltedkey.MaxVoteWeightBits)
	huge := new(big.Int).Add(big.NewInt(5), twoTo160)

	for _, proofType := range saltedTypes {
		t.Run(proofType.String(), func(t *testing.T) {
			signer, err := testcsp.NewSigner()
			qt.Assert(t, err, qt.IsNil)
			root, err := signer.CensusRoot(proofType)
			qt.Assert(t, err, qt.IsNil)

			pid := util.RandomBytes(32)
			voter := newVoter(t)
			bundle := testcsp.Bundle(pid, voter.Address().Bytes(), huge)
			proof, err := signer.SignProof(proofType, bundle, pid)
			qt.Assert(t, err, qt.IsNil)

			t.Run("v2 rejects the oversized weight", func(t *testing.T) {
				valid, _, err := verify(newProcess(pid, root, v2Origin), pid, proof, voter)
				qt.Assert(t, err, qt.IsNotNil)
				qt.Assert(t, valid, qt.IsFalse)
			})

			t.Run("legacy is unchanged", func(t *testing.T) {
				valid, weight, err := verify(newProcess(pid, root, legacyOrigin), pid, proof, voter)
				qt.Assert(t, err, qt.IsNil)
				qt.Assert(t, valid, qt.IsTrue)
				qt.Assert(t, weight.String(), qt.Equals, huge.String())
			})
		})
	}
}

// TestCSPSaltDerivationsAreMutuallyExclusive pins the cross-derivation replay
// down: a proof salted under one origin's rule must not verify on an election
// of the other origin. The same pid is deliberately reused for both processes —
// the strongest form of the claim, since real legacy and V2 processIDs always
// differ at the censusOrigin byte (26).
func TestCSPSaltDerivationsAreMutuallyExclusive(t *testing.T) {
	for _, proofType := range saltedTypes {
		t.Run(proofType.String(), func(t *testing.T) {
			signer, err := testcsp.NewSigner()
			qt.Assert(t, err, qt.IsNil)
			root, err := signer.CensusRoot(proofType)
			qt.Assert(t, err, qt.IsNil)

			pid := util.RandomBytes(32)
			voter := newVoter(t)
			weight := big.NewInt(7)
			bundle := testcsp.Bundle(pid, voter.Address().Bytes(), weight)

			legacyProof, err := signer.SignProof(proofType, bundle, expectedSalt(t, legacyOrigin, pid, weight))
			qt.Assert(t, err, qt.IsNil)
			v2Proof, err := signer.SignProof(proofType, bundle, expectedSalt(t, v2Origin, pid, weight))
			qt.Assert(t, err, qt.IsNil)

			// a legacy proof is rejected on a V2 election
			valid, _, err := verify(newProcess(pid, root, v2Origin), pid, legacyProof, voter)
			qt.Assert(t, err, qt.IsNotNil)
			qt.Assert(t, valid, qt.IsFalse)

			// and a V2 proof is rejected on a legacy election
			valid, _, err = verify(newProcess(pid, root, legacyOrigin), pid, v2Proof, voter)
			qt.Assert(t, err, qt.IsNotNil)
			qt.Assert(t, valid, qt.IsFalse)
		})
	}
}

// TestCSPUnsaltedUnaffected checks the V2 origin does not touch the unsalted
// proof types: one and the same proof verifies on both origins.
func TestCSPUnsaltedUnaffected(t *testing.T) {
	for _, proofType := range []models.ProofCA_Type{models.ProofCA_ECDSA, models.ProofCA_ECDSA_BLIND} {
		t.Run(proofType.String(), func(t *testing.T) {
			signer, err := testcsp.NewSigner()
			qt.Assert(t, err, qt.IsNil)
			root, err := signer.CensusRoot(proofType)
			qt.Assert(t, err, qt.IsNil)

			pid := util.RandomBytes(32)
			voter := newVoter(t)
			// a weight that would be rejected by the salted path's guard, to show
			// the guard really is confined to it
			weight := new(big.Int).Lsh(big.NewInt(1), saltedkey.MaxVoteWeightBits)
			bundle := testcsp.Bundle(pid, voter.Address().Bytes(), weight)
			proof, err := signer.SignProof(proofType, bundle, nil)
			qt.Assert(t, err, qt.IsNil)

			for _, origin := range bothOrigins {
				valid, gotWeight, err := verify(newProcess(pid, root, origin), pid, proof, voter)
				qt.Assert(t, err, qt.IsNil, qt.Commentf("origin %s", origin))
				qt.Assert(t, valid, qt.IsTrue)
				qt.Assert(t, gotWeight.String(), qt.Equals, weight.String())
			}
		})
	}
}

// TestCSPAbsentWeightDefaultsToOne preserves the contract that an absent
// voteWeight means a weight of 1, on both origins.
func TestCSPAbsentWeightDefaultsToOne(t *testing.T) {
	for _, proofType := range saltedTypes {
		for _, origin := range bothOrigins {
			t.Run(proofType.String()+"/"+origin.String(), func(t *testing.T) {
				signer, err := testcsp.NewSigner()
				qt.Assert(t, err, qt.IsNil)
				root, err := signer.CensusRoot(proofType)
				qt.Assert(t, err, qt.IsNil)

				pid := util.RandomBytes(32)
				voter := newVoter(t)
				bundle := testcsp.Bundle(pid, voter.Address().Bytes(), nil)

				proof, err := signer.SignProof(proofType, bundle, expectedSalt(t, origin, pid, nil))
				qt.Assert(t, err, qt.IsNil)

				valid, weight, err := verify(newProcess(pid, root, origin), pid, proof, voter)
				qt.Assert(t, err, qt.IsNil)
				qt.Assert(t, valid, qt.IsTrue)
				qt.Assert(t, weight.Int64(), qt.Equals, int64(1))
			})
		}
	}
}
