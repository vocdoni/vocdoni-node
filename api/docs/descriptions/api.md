
The Vocdoni API is a REST API that substitutes the previous RPCs in order to make it easier for  developers/integrators to build on top of the voting protocol. This API facilitates the creation of voting processes with Vocdoni, without the hassle of integrating with a complex distributed stack with blockchain components. The API allows integrators to perform all the features enabled by the Vocdoni voting protocol, such as creating accounts, organizations, voting processes, and censuses, as well as casting votes. The API is designed to abstract away the complexity of the Vocdoni protocol as much as possible, offering a simple and straightforward way to performing these actions.

Vocdoni API URLs: 

- **Production**: https://api.vocdoni.io/v2
- **Staging**: https://api-stg.vocdoni.net/v2
- **Development**: https://api-dev.vocdoni.net/v2

The API contains the following endpoints: 

- [**Chain**](chain): The Vocdoni blockchain is named Vochain. It is a Byzantine fault-tolerant network based on Tendermint that executes the Vocdoni Protocol logic represented as a state machine. Its main purpose is to register votes to a decentralized data store that is able to guarantee univeral verifiability. The chain endpoints allow you to consult the state of the chain, estimate transactions costs, list organizations, and get more Vochain info.
- [**Accounts**](accounts): Identified by an Ethereum-like address. An account can create and manage elections, transfer tokens, give power to other accounts on its behalf (delegation) and manage its metadata. This endpoint allows users to set the metadata associated with an existing account and to query for information related to existing accounts.
- [**Elections**](elections): The elections endpoint serves information related to elections such as basic election information, election keys, and submitted votes, as well as enabling users to create a new election and modify existing ones. There is a set of [options, specifications, and lifecycle states](https://developer.vocdoni.io/protocol#elections) that determine the behavior of an election and how votes are counted. 
- [**Censuses**](censuses): The census is a key component of any voting process. It specifies the set of users (each identified by a public key or address) eligible to participate in an election. The various types of census are documented [here](https://developer.vocdoni.io/protocol/census). This endpoint provides census information like the Merkle root, type, total weight, and size of a census. It also allows you to import/export censuses and create new ones.
- [**Votes**](votes): This endpoint serves all the information associated with any specific vote, including its validity. It is also how users can cast votes.
- [**Wallet**](wallet): The wallet endpoint facilitates the creation of accounts on the Vochain. This endpoint fulfills requests relating to the token balance held by a given account. 
- [**SIK**](sik): The Secret Identity Key is a user-generated piece of information that proves the user's identity without revealing it. It is the hash of the user's address, the signature of a public message, and an optional secret part. It is used to ensure anonymous voting. All registered accounts or anonymous voters must register a SIK, and these keys are all stored in a Merkle tree. The `/siks` endpoints helps to generate a proof of membership, get the current valid SIK roots, or check if an account has a valid SIK.


### Errors 

Backend error messages list are defined here: https://github.com/vocdoni/vocdoni-node/blob/master/api/errors.go

About the **204 no content** error: this message will be returned only if the asset being queried cannot be found but no other errors have occurred. This response is commonly used to prevent Javascript errors that may arise when a client is waiting for a  transaction to be published. During this waiting period, the client can repeatedly query the endpoint until a  successful response with a status code of 200 is received, thereby avoiding any errors that may occur due to the transaction not being published yet.