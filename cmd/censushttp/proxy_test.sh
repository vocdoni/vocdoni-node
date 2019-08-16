#!/bin/bash

C="JonSnow-$RANDOM"
PUB="0347f650ea2adee1affe2fe81ee8e11c637d506da98dc16e74fc64ecb31e1bb2c1"
#PUB=""
SIG="000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

echo "addCensus"
echo '{ "id":"req-'$RANDOM'", "signature":"'$SIG'", "request":{ 	"timestamp":'$(date +%s)', 	"method":"addCensus", 	"censusId":"GoT_Favorite", 	"censusUri":"http://localhost:1500", 	"pubKeys":["'$PUB'"]}}' | websocat ws://localhost:9090/census

echo "addClaim"
echo '{ "id":"req-'$RANDOM'", "signature":"'$SIG'", "request":{ "timestamp":'$(date +%s)', "method":"addClaim",	"censusId":"GoT_Favorite", "censusUri":"http://localhost:1500",	"claimData":"'$C'"}}' | websocat ws://localhost:9090/census

echo "addClaimBulk"
echo '{ "id":"req-'$RANDOM'", "signature":"'$SIG'", "request":{	"timestamp":'$(date +%s)', "method":"addClaimBulk", "censusId":"GoT_Favorite", "censusUri":"http://localhost:1500", "claimsData": ["Tyrion-'$RANDOM'","Arya-'$RANDOM'","Jaime-'$RANDOM'","Bran-'$RANDOM'"]}}' | websocat ws://localhost:9090/census

echo "genProof"
proof=$(echo '{ "id":"req-'$RANDOM'", "signature":"'$SIG'", "request":{	"timestamp":'$(date +%s)', "method":"genProof",	"censusId":"GoT_Favorite", "censusUri":"http://localhost:1500",	"claimData":"'$C'"}}' | websocat ws://localhost:9090/census)
echo $proof

echo "checkProof"
echo '{"id":"req-'$RANDOM'", "signature":"'$SIG'", "request":{"timestamp":'$(date +%s)', "method":"checkProof",	"censusId":"GoT_Favorite", "censusUri":"http://localhost:1500",	"claimData":"'$C'", "proofData":'$proof'}}' | websocat ws://localhost:9090/census

echo "getRoot"
echo '{ "id":"req-'$RANDOM'", "signature":"'$SIG'", "request":{	"timestamp":'$(date +%s)', "method":"getRoot", "censusUri":"http://localhost:1500", "censusId":"GoT_Favorite"}}' | websocat ws://localhost:9090/census

echo "dump"
echo '{"id":"req-'$RANDOM'", "signature":"'$SIG'", "request":{ "timestamp":'$(date +%s)', "method":"dump", "censusUri":"http://localhost:1500", "censusId":"GoT_Favorite"}}' | websocat ws://localhost:9090/census

#curl -d '{
#"method":"addClaim",
#"censusId":"GoT_Favorite",
#"claimData":"Jon Snow",
#"timeStamp":"1547814675",
#"signature":"a117c4ce12b29090884112ffe57e664f007e7ef142a1679996e2d34fd2b852fe76966e47932f1e9d3a54610d0f361383afe2d9aab096e15d136c236abb0a0d0e" }' http://localhost:1500
#curl -d '{"method":"genProof","censusId":"GoT_Favorite","claimData":"Jon Snow"}' http://localhost:1500
#curl -d '{"method":"getRoot","censusId":"GoT_Favorite"}' http://localhost:1500
#curl -d '{
#"method":"checkProof",
#"censusId":"GoT_Favorite","claimData":"Jon Snow",
#"rootHash":"0x2f0ddde5cb995eae23dc3b75a5c0333f1cc89b73f3a00b0fe71996fb90fef04b",
#"proofData":"0x000200000000000000000000000000000000000000000000000000000000000212f8134039730791388a9bd0460f9fbd0757327212a64b3a2b0f0841ce561ee3"}' http://localhost:1500
#curl -d '{"method":"dump","censusId":"GoT_Favorite"}' http://localhost:1500
