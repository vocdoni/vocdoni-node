Create a new census on the backend side. The census is still unpublished until [publish](publish-census) is called.  

To create the census it require a `Bearer token` created on the user side using a [UUID](https://en.wikipedia.org/wiki/Universally_unique_identifier). This token **should we stored for the user to perform operations to this census** such add participants or publish.

It return a new random censusID (a random 32 bytes hex string), which are used (along with the Bearer token) to [add participant keys](add-participants-to-census) to the census. Once the census is published no more keys can be added.

To use a census on an election, it **must be published**.

- Available types are: `weighted` and `zkindexed`
- Require header Bearer token created user side