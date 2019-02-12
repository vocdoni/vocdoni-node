## Census HTTP service

Reference implementation of a voting census service running on the Vocdoni platform

## Compile

In a GO ready environment:

```
go get -u github.com/vocdoni/dvote-census/...
go build -o censusHttpService github.com/vocdoni/dvote-census/cmd/censushttp
```

## Usage

`./censusHttpService <port> <censusId>[:pubKey] [<censusId>[:pubKey] ...]`

Example

```
./censusHttpService 1500 Got_Favorite
2019/02/12 10:20:16 Starting process HTTP service on port 1500 for namespace GoT_Favorite
2019/02/12 10:20:16 Starting server in http mode
```

## API

A HTTP jSON endpoint is available with the following possible fields: `censusId`, `claimData`, `rootHash` and `proofData`.

If `pubKey` has been configured for a specific `censusId`, then two more methods are available (`timeStamp` and `signature`) to provide authentication.

The next table shows the available methods and its relation with the fields.

| method     | censusId  | claimData | rootHash | proofData | protected? | description |
|------------|-----------|-----------|----------|-----------|------------|------------|
| `addCLaim`   | mandatory | mandatory | none     | none      | yes | adds a new claim to the merkle tree       |
| `getRoot`    | mandatory | none      | none     | none      | no         | get the current merkletree root hash
| `genProof`   | mandatory | mandatory | optional | none      | no         | generate the merkle proof for a given claim
| `checkProof` | mandatory | mandatory | optional | mandatory | no         | check a claim and its merkle proof 
| `getIdx`     | mandatory | mandatory | optional | none      | no         | get the merkletree data index of a given claim
| `dump`       | mandatory | none      | optional | none      | yes        | list the contents of the census for a given hash


## Signature

The signature provides authentication by signing a concatenation of the following strings (even if empty) without spaces: `censusId rootHash claimData timeStamp`.

The `timeStamp` when received on the server side must not differ more than 10 seconds from the current UNIX time.

## Examples

#### add claims

Add two new claims, one for `Jon Snow` and another for `Tyrion`.
```
curl -d '{"censusID":"GoT_Favorite","claimData":"Jon Snow"}' http://localhost:1500/addClaim

{"error":false,"response":""}
```

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Tyrion"}' http://localhost:1500/addClaim

{"error":false,"response":""}
```

In case signature is enabled:

```
curl -d '{
"censusID":"GoT_Favorite",
"claimData":"Jon Snow", 
"timeStamp":"1547814675",
"signature":"a117c4ce12b29090884112ffe57e664f007e7ef142a1679996e2d34fd2b852fe76966e47932f1e9d3a54610d0f361383afe2d9aab096e15d136c236abb0a0d0e" }' http://localhost:1500/addClaim

{"error":false,"response":""}
```


#### generate proof

Generate a merkle proof for the claim `Jon Snow`

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Jon Snow"}' http://localhost:1500/genProof

{"error":false,"response":"0x000200000000000000000000000000000000000000000000000000000000000212f8134039730791388a9bd0460f9fbd0757327212a64b3a2b0f0841ce561ee3"}
```

If `rootHash` is specified, the proof will be calculated for the given root hash.

#### get root

The previous merkle proof is valid only for the current root hash. Let's get it

```
curl -d '{"censusID":"GoT_Favorite"}' http://localhost:1500/getRoot

{"error":false,"response":"0x2f0ddde5cb995eae23dc3b75a5c0333f1cc89b73f3a00b0fe71996fb90fef04b"}
```


#### check proof

Now let's check if the proof is valid

```
curl -d '{
"censusID":"GoT_Favorite","claimData":"Jon Snow",
"rootHash":"0x2f0ddde5cb995eae23dc3b75a5c0333f1cc89b73f3a00b0fe71996fb90fef04b",
"proofData":"0x000200000000000000000000000000000000000000000000000000000000000212f8134039730791388a9bd0460f9fbd0757327212a64b3a2b0f0841ce561ee3"}' http://localhost:1500/checkProof

{"error":false,"response":"valid"}
```

If `rootHash` is not specified, the current root hash is used.

#### dump

Dump contents of a specific censusId (values)

```
curl -d '{"censusID":"GoT_Favorite"}' http://localhost:1500/dump

{"error":false,"response":"[\"Tyrion\",\"Jon Snow\"]"}
```

If `rootHash` is specified, dump will return the values for the merkle tree with the given root hash.