Add list of public key/address with its weight to an unpublished census and returns the resulting 
[Merkle Root](https://en.wikipedia.org/wiki/Merkle_tree).  

Each addition will modify the census merkle root creating a new "snapshot" of the census at this moment. This root  identifies the census at this point and can be used to publish the census at this specific state.

For example, supposing a census with id `0x1234` (random hex string generated during census creation), add 10 keys will generate specific root for this state, ex `0xabcd`. 

If we add 5 keys more, the resulting root changes, the keys are added and the new census have the first 10 keys plus the last 5, with a resulting root of `0xffff`. 

So, at [publishing moment](census-publish), you could specify no root to publish census on the last 
state (`0xffff`), which will publish the first 10 plus the last 5. Or either specify the snapshot point which you want to publish the census, for example `0x1234`, which will publish just the first 10th.

- Requires Bearer token 
- Adds a list of wallet public key or wallet address to a census with a specific weight
- If the weight parameter is missing, weight=1 is considered