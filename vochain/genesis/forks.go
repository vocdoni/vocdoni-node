package genesis

import "math"

/*
Soft-fork activation via block-height gating.

A soft fork here is a state-affecting protocol change activated at a fixed block
height, per ChainID, so every validator switches behavior on the same block. It
is lighter than the EndOfChain hard fork documented in genesis.go (no new ChainID
/ AppHash / blockstore reset): the chain and its ChainID continue across the fork.

Why the gate is safe. Before the activation height an upgraded node must produce
byte-for-byte the same state as a not-yet-upgraded node, so mixed binary versions
during the rollout window cannot diverge. The memo fork satisfies this: the memo
is a bytes field (decoded identically by nodes with and without it — no proto3
UTF-8 decode divergence), and before activation the memo is ignored (not stored,
not hashed). Only at the activation height do all upgraded nodes begin validating,
storing and hashing it together.

### Runbook: activating a height-gated soft fork on a network

  1. Pick the activation height H = current_height + margin, where margin is
     comfortably larger than the time needed for ALL full nodes (validators AND
     gateways) to upgrade: generous on lts/prod (days of blocks), short on
     dev/stage. The gate decouples "deploy the binary" from "activate the rule":
     any node may upgrade anytime in [release, H) with zero divergence.
  2. Set H for the target ChainID in the fork table below (replace forkNever);
     one-line commit.
  3. Release the binary: merge to the network's release branch (e.g. release-lts-1);
     CI (docker-release.yml) pushes the image to the tag validators track.
  4. Propagate and confirm: watchtower auto-pulls (~30s). Confirm out-of-band that
     every validator and gateway runs the new image — there is no in-protocol
     version negotiation. At least 2/3 of voting power must be upgraded before H
     for the chain to keep producing blocks; aim for 100% so no node is stranded.
  5. At H all upgraded nodes activate simultaneously; the AppHash now reflects the
     new rule.
  6. Monitor H-1..H+1: confirm blocks keep finalizing and validators agree on the
     AppHash. A non-upgraded node halts here on an AppHash mismatch (CometBFT
     panic) — the fail-safe: it stalls, it never follows a forked state.
  7. Stragglers: upgrade the image; the node resyncs past H under the new rules.
  8. Abort/rollback: BEFORE H, ship a binary moving H later (or forkNever) to
     cancel. AFTER H the change is committed consensus state — reverting needs a
     further coordinated fork.
*/

// forkNever is used as an activation height for a soft-fork that has not been
// scheduled yet on a given chain: the feature stays inactive at every height.
const forkNever = uint32(math.MaxUint32)

// voteMemoForkHeight holds, per ChainID, the block height at which the optional
// VoteEnvelope.memo field becomes active (validated against the max size,
// persisted in the StateDBVote and hashed into the vote). Before this height the
// memo is ignored, so an upgraded node produces exactly the same state as a
// pre-fork node and the rollout window stays safe.
//
// A height of 0 means active from genesis. A ChainID absent from the map (or set
// to forkNever) never activates the feature.
//
// TODO: set concrete activation heights for dev/stage/lts before release,
// coordinated with the validator upgrade so every validator runs this binary
// before the chosen height.
var voteMemoForkHeight = map[string]uint32{
	"vocdoni/TEST/1":   0,         // always on for local test/dev chains
	"vocdoni/DEV/36":   forkNever, // TODO: schedule
	"vocdoni/STAGE/12": forkNever, // TODO: schedule
	"vocdoni/LTS/1.2":  forkNever, // TODO: schedule (far-future, coordinated with ops)
}

// VoteMemoActive reports whether the optional VoteEnvelope.memo field is active
// for the given chain at the given block height.
func VoteMemoActive(chainID string, height uint32) bool {
	forkHeight, ok := voteMemoForkHeight[chainID]
	if !ok || forkHeight == forkNever {
		return false
	}
	return height >= forkHeight
}
