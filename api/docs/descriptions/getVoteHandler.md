Returns the content of an existing Vote. If the election is encrypted, returns the `encryptionKeys` indexes and codifies the package.


Each Vote is identified by its `voteId`, also called `nullifier`. The `nullifier` is deterministic and its hash can be computed with the following (using `Keccak256`):

- For signature based elections, the nullifier is the hash of the `voterAddress` + `processId`
- For anonymous elections, the nullifier is the hash of the `privateKey` + `processId`


If an election is anonymous, the `voterId` will not be returned.
If an election is encrypted, the `encryptionKeyIndexes` will only be returned once the election is complete.

`Height` and `txIndex` refer to the block height and the index of the transaction where vote is registered.


The `overwriteCount` refers to the number of vote overwrites already executed by the user. At election creation time, you can specify the `maxVoteOverwrites` parameter, which defines how many times a voter can submit a vote. Only the most recent vote for any voter will be taken into an election's final results.