Adds list of public keys/addresses and voting weights to an unpublished census. Returns the resulting 
[Merkle Root](https://en.wikipedia.org/wiki/Merkle_tree).  

Each addition will modify the census merkle root creating a new "snapshot" of the census at this moment. This root  identifies the census at this point and can be used to publish the census at this specific state.

For example, supposing a census with id `0x1234` (random hex string generated during census creation), adding 10 keys will generate specific root for this state, ex `0xabcd`. 

If we add 5 keys more, the resulting root changes, the keys are added, and the new census has the first 10 keys plus the 5 new ones, with a resulting root of `0xffff`. 

So, when [publishing](census-publish) the census, you could specify 'no root' to publish census on the last 
state (`0xffff`), which will publish a census of all 15 voters. Or you could specify the snapshot point at which you want to publish, for example `0x1234`, which will publish a census containing just the first 10 voters.

- Requires Bearer token 
- Adds a list of wallet public key or wallet addresses to a census with a specific weight
- If the weight parameter is missing, weight=1 is considered