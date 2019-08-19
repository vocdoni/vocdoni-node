#!/bin/bash

C="JonSnow-$RANDOM"
#PUB="0347f650ea2adee1affe2fe81ee8e11c637d506da98dc16e74fc64ecb31e1bb2c1"
PUB=""
SIG="000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

addCensus() {
echo "addCensus"
curl -s http://localhost:1500 -d '{
"id":"req-'$RANDOM'",
"signature":"'$SIG'",
"request":{
	"timestamp":'$(date +%s)',
	"method":"addCensus",
	"censusId":"GoT_Favorite",
	"pubKeys":["'$PUB'"]}}' | jq .
}

addClaim() {
echo "addClaim"
curl -s http://localhost:1500 -d '{
"id":"req-'$RANDOM'",
"signature":"'$SIG'",
"request":{
	"timestamp":'$(date +%s)',
	"method":"addClaim",
	"censusId":"GoT_Favorite",
	"claimData":"'$C'"}}' | jq .
}

addClaimBulk() {
echo "addClaimBulk"
curl -s http://localhost:1500 -d '{
"id":"req-'$RANDOM'",
"signature":"'$SIG'",
"request":{
	"timestamp":'$(date +%s)',
	"method":"addClaimBulk",
	"censusId":"GoT_Favorite",
	"claimsData": ["Tyrion-'$RANDOM'","Arya-'$RANDOM'","Jaime-'$RANDOM'","Bran-'$RANDOM'"]}}' | jq .
}

fill() {
claims="["
i=0
while [ $i -lt 99 ]; do
	c=$(echo $RANDOM$RANDOM$RANDOM$RANDOM | sha256sum -z | awk '{print $1}')
	claims="${claims}\"${c::-2}\","
	i=$(($i+1))
done
c=$(echo $RANDOM$RANDOM$RANDOM$RANDOM | sha256sum -z | awk '{print $1}')
claims="${claims}\"${c::-2}\"]"

curl -s http://localhost:1500 -d '{
"id":"req-'$RANDOM'",
"signature":"'$SIG'",
"request":{
	"timestamp":'$(date +%s)',
	"method":"addClaimBulk",
	"censusId":"GoT_Favorite",
	"claimsData":'$claims'}}' | jq .
}

checkProof() {
echo "genProof"
proof=$(curl -s http://localhost:1500 -d '{
"id":"req-'$RANDOM'",
"signature":"'$SIG'",
"request":{
	"timestamp":'$(date +%s)',
	"method":"genProof",
	"censusId":"GoT_Favorite",
	"claimData":"'$C'"}}' | jq .request.siblings)
echo $proof

echo "checkProof"
curl -s http://localhost:1500 -d '{
"id":"req-'$RANDOM'",
"signature":"'$SIG'",
"request":{
	"timestamp":'$(date +%s)',
	"method":"checkProof",
	"censusId":"GoT_Favorite",
	"claimData":"'$C'",
	"proofData":'$proof'}}' | jq .
}

getRoot() {
echo "getRoot"
curl -s http://localhost:1500 -d '{
"id":"req-'$RANDOM'",
"signature":"'$SIG'",
"request":{
	"timestamp":'$(date +%s)',
	"method":"getRoot",
	"censusId":"GoT_Favorite"}}' | jq .
}

dump() {
curl -s http://localhost:1500 -d '{
"id":"req-'$RANDOM'",
"signature":"'$SIG'",
"request":{
	"timestamp":'$(date +%s)',
	"method":"dump",
	"censusId":"GoT_Favorite"}}' | jq .request.claimsData | tr -d '\n' | tr -d ' '
}

dumpPlain() {
echo "dumpPlain"
curl -s http://localhost:1500 -d '{
"id":"req-'$RANDOM'",
"signature":"'$SIG'",
"request":{
	"timestamp":'$(date +%s)',
	"method":"dumpPlain",
	"censusId":"GoT_Favorite"}}' | jq .
}

function import() {
echo "import claims from $1"
claims="$(cat $1)"
curl -s http://localhost:1500 -d '{
"id":"req-'$RANDOM'",
"signature":"'$SIG'",
"request":{
	"timestamp":'$(date +%s)',
	"method":"importDump",
	"claimsData":'$claims',
	"censusId":"GoT_Favorite"}}' | jq .
}

[ -z "$1" -o "$1" == "all" ] && {
addCensus
addClaim
checkProof
getRoot
dump
dumpPlain
exit
}

$@

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
