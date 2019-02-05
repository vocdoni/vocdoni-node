## Census HTTP service

Reference implementation of a voting census service running on the Vocdoni platform

#### Compile

In a GO ready environment:

```
go get -u github.com/vocdoni/dvote-census/...
go build -o censusHttpService github.com/vocdoni/dvote-census/cmd/censushttp
```

#### Usage

`./censusHttpService <port> [publicKey]`

Example:

```
./censusHttpService 1500
2018/12/17 09:54:20 Starting process HTTP service on port 1500
2018/12/17 09:54:20 Starting server in http mode
```

#### add claims

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Jon Snow"}' http://localhost:1500/addClaim

{"error":false,"response":""}
```

If public key authentication enabled, `signature` field is required using Nancl signature.

The signed message is expected to be a concatenation of the following fields: `censusID, claimData, timeStamp`

```
curl -d '{"censusID":"GoT_Favorite",
"claimData":"Jon Snow", 
"timeStamp":"1547814675", 
"signature":"a117c4ce12b29090884112ffe57e664f007e7ef142a1679996e2d34fd2b852fe76966e47932f1e9d3a54610d0f361383afe2d9aab096e15d136c236abb0a0d0e"}' http://localhost:1500/addClaim

{"error":false,"response":""}
```


#### generate proof

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Jon Snow"}' http://localhost:1500/genProof

{"error":false,"response":"0x000000000000000000000000000000000000000000000000000000000000000352f3ca2aaf635ec2ae4452f6a65be7bca72678287a8bb08ad4babfcccd76c2fef1aac7675261bf6d12c746fb7907beea6d1f1635af93ba931eec0c6a747ecc37"}
```

#### check proof

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Jon Snow", "proofData": "0x0000000000000000000000000000000000000000000000000000000000000000000123"}' http://localhost:1500/checkProof

{"error":false,"response":"invalid"}
```

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Jon Snow", "proofData": "0x000000000000000000000000000000000000000000000000000000000000000352f3ca2aaf635ec2ae4452f6a65be7bca72678287a8bb08ad4babfcccd76c2fef1aac7675261bf6d12c746fb7907beea6d1f1635af93ba931eec0c6a747ecc37"}' http://localhost:1500/checkProof

{"error":false,"response":"valid"}
```

#### make snapshot

Snapshots are static and unmutable copies of a specific census

```
curl -d '{"censusID":"GoT_Favorite"}' http://localhost:1500/snapshot

{"error":false,"response":"0x8647692e073a10980d821764c65ca3fddbc606bb936f9812a7a806bfa97df152"}
```

The name for the snapshot is "0x8647692e073a10980d821764c65ca3fddbc606bb936f9812a7a806bfa97df152" which represents the Root Hash.

Now you can use it as censusID for checkProof, getRoot, genProof and dump. But addClaim is not longer allowed.

#### dump

Dump contents of a specific censusID (values)

```
curl -d '{"censusID":"GoT_Favorite"}' http://localhost:1500/dump
{"error":false,"response":"[\"Tyrion\",\"Jon Snow\"]"}
```












##### add claims

```
curl -d '{"processID":"GoT_Favorite","claimData":"Jon Snow"}' http://localhost:1500/addClaim
{"error":false,"response":""}
```

```
curl -d '{"processID":"GoT_Favorite","claimData":"Tyrion"}' http://localhost:1500/addClaim
{"error":false,"response":""}
```

##### generate proof

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Jon Snow"}' http://localhost:1500/genProof
{"error":false,"response":"0x000000000000000000000000000000000000000000000000000000000000000352f3ca2aaf635ec2ae4452f6a65be7bca72678287a8bb08ad4babfcccd76c2fef1aac7675261bf6d12c746fb7907beea6d1f1635af93ba931eec0c6a747ecc37"}
```

##### check proof

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Jon Snow", "proofData": "0x0000000000000000000000000000000000000000000000000000000000000000000123"}' http://localhost:1500/checkProof
{"error":false,"response":"invalid"}
```

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Jon Snow", "proofData": "0x000000000000000000000000000000000000000000000000000000000000000352f3ca2aaf635ec2ae4452f6a65be7bca72678287a8bb08ad4babfcccd76c2fef1aac7675261bf6d12c746fb7907beea6d1f1635af93ba931eec0c6a747ecc37"}' http://localhost:1500/checkProof
{"error":false,"response":"valid"}
```

