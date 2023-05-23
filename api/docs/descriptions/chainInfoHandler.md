Return basic Vocdoni Blockchain (Vochain) information like blockTime, chainId, current height...

`blockTime`: each array position return average time for 1 minute, 10 minutes, 1 hour, 6 hours and 24 hours.

`blockTime`: every array position represents the average for 1 minute, 10m, 1h, 6h, 24h

`MaxCensusSize`: is a new feature introduced in the blockchain that is used to limit the number of  votes that can be registered for an election. This feature helps to prevent any potential overflow of the blockchain when the number of votes goes beyond the maximum limit. This is the maximum value  that an election creation can allow.

In order to create an election, the creator is required to set the `MaxCensusSize` parameter to a proper value. Typically, this value should be equal to the size of the census. If the MaxCensusSize parameter is set to 0, an error will occur and the election cannot be created.

The `MaxCensusSize` parameter determines the maximum number of votes that can be registered by the blockchain.  If the number of votes exceeds this limit, the vote transaction will fail (overwrite votes does not count).

See `MaxCensusSize` attribute on the VocdoniSDK  to add the maximum census size to a single election. Will throw an error if is superior than the allowed on the Vochain: `Max census size for the election is greater than allowed size`.

`networkCapacity`  indicates how many votes per block is the blockchain supposed to achieve. As bigger the capacity as cheaper the elections.