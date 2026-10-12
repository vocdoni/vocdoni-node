Register an account to the blockchain.

- Requires a signed protobuf transaction on the body to perform the operation.
- The account metadata is referenced by the `infoURI` of the transaction and is hosted by the account owner; the node does not store it. Accounts without an `infoURI` won't be listed on the [/chain/organizations](organizations-list) endpoint. Check there to read about differences between an account and a organization.