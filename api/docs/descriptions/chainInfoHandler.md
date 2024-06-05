Returns basic Vocdoni Blockchain (Vochain) information like blockTime, chainId, current height, and more.

`blockTime`: each array position return average time for 1 minute, 10 minutes, 1 hour, 6 hours and 24 hours.

`blockTime`: every array position represents the average for 1 minute, 10m, 1h, 6h, 24h

`MaxCensusSize`: is used to limit the number of voters that can be registered to a given census. This feature helps to prevent any potential overflow of the blockchain when the number of votes goes beyond the maximum limit. This is the maximum value that an election creation can allow.

In order to create an election, the creator is required to set the `MaxCensusSize` parameter to a proper value. Typically, this value should be equal to the size of the census. If the MaxCensusSize parameter is set to 0, an error will occur and the election cannot be created. If the `MaxCensusSize` is greater than allowed by the blockchain, an error will be returned.


`networkCapacity`  indicates how many votes per block is the blockchain expected to achieve. Larger capacity translates to cheaper elections.