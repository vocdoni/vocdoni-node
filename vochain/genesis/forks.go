package genesis

import "math"

/*
Soft-fork activation via per-election gating.

A soft fork here is a state-affecting protocol change activated per ChainID, so
every validator switches behavior on the same elections. It is lighter than the
EndOfChain hard fork documented in genesis.go (no new ChainID / AppHash /
blockstore reset): the chain and its ChainID continue across the fork.

Why the gate is keyed on the election and not on the block height. Before the
activation point an upgraded node must produce byte-for-byte the same state as a
not-yet-upgraded node, so mixed binary versions during the rollout window cannot
diverge. Height gating cannot give that here, because a vote proof is verified at
CheckTx and then NOT re-verified at DeliverTx when it is served from the vote
cache (see vochain/transaction.VoteTxCheck). A vote checked at H-1 under the old
rule and delivered at H under the new one would be accepted by the proposer,
which skips re-verification, and rejected by every node that never saw it in its
mempool: an AppHash divergence, i.e. a chain halt.

Keying on immutable election data removes that class of bug entirely. The
predicate depends only on the ChainID and on fields fixed when the election was
created, so it returns the same value at CheckTx and at DeliverTx, on every node,
regardless of height, wall clock or mempool timing. An election never changes
rules mid-flight, and an off-chain signer (a CSP) can evaluate the same predicate
from public election data without polling the chain height.

### Runbook: activating a per-election soft fork on a network

  1. Merge with every production chain set to forkNever. Behavior is identical to
     the previous release, so the binary can be deployed at any time with zero
     divergence risk. This decouples "deploy the binary" from "activate the rule".
  2. Ship the mirrored change in every off-chain component that must agree on the
     rule (for the CSP fork below: vocdoni/blind-csp, vocdoni/saas-backend,
     vocdoni/vocdoni-sdk), behind the same predicate, with the activation point
     still unset. Both sides must be deployable before either activates.
  3. Confirm out-of-band that every validator AND gateway runs the new image;
     there is no in-protocol version negotiation. Watchtower auto-pulls (~30s).
  4. Pick the activation point comfortably after the last node upgrade, the last
     off-chain upgrade, and the longest lead time with which that network's
     organizers create elections ahead of their start date.
  5. Set it for the target ChainID in the fork table below (replace forkNever);
     one-line commit. Release the binary: merge to the network's release branch
     (e.g. release-lts-1); CI (docker-release.yml) pushes the image.
  6. Monitor the first election that activates the rule. Note the risk profile is
     inverted versus a height fork: node divergence is impossible because the
     predicate is election-local, but a stale off-chain signer issues proofs the
     chain rejects. That surfaces as voter-visible failures on one election, not
     as a chain halt. Watch the signer, not the AppHash.
  7. Abort/rollback: BEFORE the activation point, ship a binary moving it later
     (or to forkNever) to cancel. AFTER it, elections already running under the
     new rule are committed consensus state and reverting needs a further
     coordinated fork.
  8. Removing the legacy branch: only once every election predating the fork has
     ended on every chain in the table.

Note there is one older, inline height-gated soft fork at
vochain/transaction/transaction.go, guarding an ISTC reschedule on
vocdoni/LTS/1.2. It is deliberately left where it is: it is slated for deletion
once that chain forgets old blocks, not for migration into this table.
*/

// forkNever is used as an activation point for a soft-fork that has not been
// scheduled yet on a given chain: the feature stays inactive for every election.
const forkNever = uint32(math.MaxUint32)

// cspSaltedProofV2Time holds, per ChainID, the unix timestamp from which an
// election's StartTime activates the fixed CSP salted-proof derivation
// (crypto/saltedkey.Salt). Elections that started before it keep the legacy
// derivation, which cropped the processID to 20 bytes and so shared one salted
// key across every election of an organization (issue #1424).
//
// StartTime is the right anchor because it is always set (NewProcessTxCheck
// defaults it to the current block timestamp and rejects earlier values), it is
// immutable after creation (SetProcessDuration moves the end, never the start),
// it is part of the consensus state, and it is exposed off-chain as the
// election's startDate so a CSP can evaluate the same predicate.
//
// It also makes the rollout window self-enforcing: votes with
// currentTime < process.StartTime are rejected, so no vote governed by the new
// rule can be cast before the activation timestamp in wall-clock terms.
//
// A value of 0 means active for every election. A ChainID absent from the map
// (custom chains, vocone, and the "test" ChainID used by
// vochain.TestBaseApplication) or set to forkNever never activates it.
var cspSaltedProofV2Time = map[string]uint32{
	"vocdoni/TEST/1":   0,         // always on for the testsuite chain
	"vocdoni/DEV/36":   forkNever, // TODO: schedule
	"vocdoni/STAGE/12": forkNever, // TODO: schedule
	"vocdoni/LTS/1.2":  forkNever, // TODO: schedule (coordinated with the CSP operators)
}

// CSPSaltedProofV2Active reports whether the fixed salt derivation for
// ECDSA_PIDSALTED and ECDSA_BLIND_PIDSALTED proofs is active for an election
// with the given StartTime on the given chain.
func CSPSaltedProofV2Active(chainID string, electionStartTime uint32) bool {
	forkTime, ok := cspSaltedProofV2Time[chainID]
	if !ok || forkTime == forkNever {
		return false
	}
	return electionStartTime >= forkTime
}
