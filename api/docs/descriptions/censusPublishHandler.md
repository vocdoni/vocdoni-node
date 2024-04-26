Registers a census to storage (IPFS in our case). After this, the census can't be edited. 
        
You can optionally provide the census Merkle root to specify census publication at a specific snapshot. See [censuses/{censusId}/participants](census-add-participants-to-census)

- Requires Bearer token
- The census is copied to a new census identified by its Merkle Root
- The new census **cannot be modified**
- The census is published to the storage provided (IPFS in our case)
- The new census ID is returned and can be used for querying
- If a censusID with the same root has been already published, the request will fail
- If `root` is specified as path parameter, it publish the census at specific  root (append the census to existing one).