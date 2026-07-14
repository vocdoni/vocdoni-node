Submits several pre-signed transactions in a single request, **in order**.

The transactions are submitted sequentially and **fail-fast**: the first one that fails to be submitted is placed in `failed` and submission stops, leaving the remaining transactions in `pending`. Every input transaction is returned in exactly one group:

- `submitted`: accepted by the mempool (broadcast).
- `failed`: the first transaction that failed to be submitted (its `error` is set).
- `pending`: the transactions after the failed one, which were not sent.

For a `NewProcess` (create election) transaction, each item includes its predicted `processId`.

> **Note:** `submitted` means the mempool accepted the broadcast, **not** that the transaction is included in a block. A mempool-accepted transaction can still be discarded at commit (for example, a stale or contended account nonce, which is not checked at mempool admission). The caller must confirm each `submitted` item on-chain (e.g. via [`chain/transactions/reference/{hash}`](transaction-by-reference)) and resubmit any that did not land, together with the `failed` and `pending` items, reusing the same account nonces.

Intended for creating multiple dependent processes at once: build and sign all transactions up front with contiguous account nonces (and, for elections, predicted `processId`s), then submit them together so they land in the same block in order.
