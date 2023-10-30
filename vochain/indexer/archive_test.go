package indexer

import (
	"encoding/json"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/vochain"
)

func TestImportArchive(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app, true)

	archive := []*ArchiveProcess{}
	archiveProcess1 := &ArchiveProcess{}
	archiveProcess2 := &ArchiveProcess{}
	err := json.Unmarshal([]byte(testArchiveProcess1), archiveProcess1)
	qt.Assert(t, err, qt.IsNil)
	err = json.Unmarshal([]byte(testArchiveProcess2), archiveProcess2)
	qt.Assert(t, err, qt.IsNil)
	archive = append(archive, archiveProcess1)
	archive = append(archive, archiveProcess2)

	err = idx.ImportArchive(archive)
	qt.Assert(t, err, qt.IsNil)

	process1, err := idx.ProcessInfo(archiveProcess1.ProcessInfo.ID)
	qt.Assert(t, err, qt.IsNil)
	fmt.Printf("process1: %+v\n", process1)
}

var testArchiveProcess1 = `
{
  "chainId": "vocdoni-stage-8",
  "process": {
   "processId": "c5d2460186f7f7062f413f5be0fca0abf7082ad5c7728e365002020400000001",
   "entityId": "f7062f413f5be0fca0abf7082ad5c7728e365002",
   "startBlock": 236631,
   "endBlock": 240060,
   "blockCount": 3515,
   "censusRoot": "09397c5b65efc0e95b338b610ef38394312e94418d4a828f8820008378758451",
   "censusURI": "ipfs://bafybeifduki6huacmtufg5avyo6g2pxxefwohsks3fp63w3dd733ixzecu",
   "metadata": "ipfs://bafybeiel2cvsrg7d4frqxziberrpnjj4yxzs5peyxkgsvamylpxxdn4m7q",
   "censusOrigin": 2,
   "status": 5,
   "namespace": 0,
   "envelopeType": {
    "encryptedVotes": true
   },
   "processMode": {
    "interruptible": true
   },
   "voteOptions": {
    "maxCount": 1,
    "maxValue": 2,
    "costExponent": 10000
   },
   "questionIndex": 0,
   "creationTime": "2023-10-20T07:08:43Z",
   "haveResults": true,
   "finalResults": true,
   "sourceBlockHeight": 0,
   "sourceNetworkId": "UNKNOWN",
   "maxCensusSize": 1028
  },
  "results": {
   "processId": "c5d2460186f7f7062f413f5be0fca0abf7082ad5c7728e365002020400000001",
   "votes": [
    [
     "342",
     "365",
     "21"
    ]
   ],
   "weight": null,
   "envelopeType": {
    "encryptedVotes": true
   },
   "voteOptions": {
    "maxCount": 1,
    "maxValue": 2,
    "costExponent": 10000
   },
   "blockHeight": 0
  },
  "startDate": "2023-10-20T07:59:49.977338317Z"
 }
 `

var testArchiveProcess2 = `
{
	"chainId": "vocdoni-stage-8",
	"process": {
	 "processId": "c5d2460186f7d8fcfaa76192aa69cceddaec554b1d82b0166dc9020000000031",
	 "entityId": "d8fcfaa76192aa69cceddaec554b1d82b0166dc9",
	 "startBlock": 287823,
	 "endBlock": 287858,
	 "blockCount": 35,
	 "censusRoot": "8b840a7ddbadfbc98145bf86cf3e8f53f3670a703b1b92f9d0ee4b065eae970b",
	 "censusURI": "ipfs://bafybeigff4k3pqxzokvixov6s34yaw6dje474ghnehz6duaxryyf2hd5ke",
	 "metadata": "ipfs://bafybeicyi4o3hmp27ejkakzqrdk37e2n3julvmwg3txwyyqfub3pwz6ora",
	 "censusOrigin": 2,
	 "status": 5,
	 "namespace": 0,
	 "envelopeType": {},
	 "processMode": {
	  "autoStart": true,
	  "interruptible": true
	 },
	 "voteOptions": {
	  "maxCount": 1,
	  "maxValue": 2,
	  "costExponent": 10000
	 },
	 "questionIndex": 0,
	 "creationTime": "2023-10-26T13:24:25Z",
	 "haveResults": true,
	 "finalResults": true,
	 "sourceBlockHeight": 0,
	 "sourceNetworkId": "UNKNOWN",
	 "maxCensusSize": 2
	},
	"results": {
	 "processId": "c5d2460186f7d8fcfaa76192aa69cceddaec554b1d82b0166dc9020000000031",
	 "votes": [
	  [
	   "0",
	   "1000000000000000000000",
	   "0"
	  ]
	 ],
	 "weight": null,
	 "envelopeType": {},
	 "voteOptions": {
	  "maxCount": 1,
	  "maxValue": 2,
	  "costExponent": 10000
	 },
	 "blockHeight": 0
	},
	"startDate": "2023-10-26T13:24:56.609443588Z"
   }
`
