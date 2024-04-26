Submits a transaction. Depending on the transaction type, will return one of multiple response types:
- For a NewElection transaction, `response` will be the `newElectionId`
- For a Vote transaction, `response` will be the `voteID`

Once the transaction is mined on the Vochain you can use [`chain/transactions/reference/{hash}`](transaction-by-reference) to find the block height and its index on the block to get the transaction index using [`chain/transactions/{blockHeight}/{txIndex}`](transaction-by-block-index).
