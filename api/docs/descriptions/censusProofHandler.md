Proves that the key and weight belong to the census root hash.

If the key exists on the census, returns Merkle root information. The `Proof` property refers to the key's siblings on the Merkle tree, the `value` points to the leaf of the Merkle Tree for this key (in this case, the weight), and weight is just the key's weight for this census. 

[Further reading](/protocol/Census/off-chain-tree)

- Requires Bearer token 
- Returns a merkle proof, proving the key and weight belongs to the census root hash