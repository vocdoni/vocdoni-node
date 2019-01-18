## dvot-process http service

#### start http server

```
processHttp.T.Init()
processHttp.Listen(1500, "http", "")
```

To enable authentication (using pubKey signature):

```
pubK := "39f54ce5293520b689f6658ea7f3401f4ff931fa3d90dea21ff901cdf82bb8aa"
processHttp.Listen(1500, "http", pubK)
```


#### add claims

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Jon Snow"}' http://localhost:1500/addClaim
{"error":false,"response":""}
```

```
curl -d '{"censusID":"GoT_Favorite",
"claimData":"Jon Snow", 
"timeStamp":"1547814675", 
"signature":"a117c4ce12b29090884112ffe57e664f007e7ef142a1679996e2d34fd2b852fe76966e47932f1e9d3a54610d0f361383afe2d9aab096e15d136c236abb0a0d0e"}' http://localhost:1500/addClaim
{"error":false,"response":""}
```

The signature message is a concatenation of the following strings: `censusID, claimData, timeStamp`

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Tyrion"}' http://localhost:1500/addClaim
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
