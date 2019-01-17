## dvot-process http service

#### start http server

```
processHttp.T.Init()
processHttp.Listen(1500, "http")
```

#### add claims

```
curl -d '{"censusID":"GoT_Favorite","claimData":"Jon Snow"}' http://localhost:1500/addClaim
{"error":false,"response":""}
```

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
