package indexer

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/vochain"
)

func TestImportArchive(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app)

	archive := []*ArchiveProcess{}
	archiveProcess1 := &ArchiveProcess{}
	err := json.Unmarshal([]byte(testArchiveProcess1), archiveProcess1)
	qt.Assert(t, err, qt.IsNil)
	archive = append(archive, archiveProcess1)

	_, err = idx.ImportArchive(archive)
	qt.Assert(t, err, qt.IsNil)

	process1, err := idx.ProcessInfo(archiveProcess1.ProcessInfo.ID)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, process1.ID.String(), qt.Equals, archiveProcess1.ProcessInfo.ID.String())
	qt.Assert(t, process1.Results().Votes[0][0].MathBigInt().Int64(), qt.Equals, int64(342))
	qt.Assert(t, process1.Results().Votes[0][1].MathBigInt().Int64(), qt.Equals, int64(365))
	qt.Assert(t, process1.Results().Votes[0][2].MathBigInt().Int64(), qt.Equals, int64(21))
	qt.Assert(t, process1.StartDate, qt.DeepEquals, *archiveProcess1.StartDate)
	// check that endDate is set after startDate
	qt.Assert(t, process1.EndDate.After(process1.StartDate), qt.Equals, true)
}

// This is an old archive process format, we check backwards compatibility.
// At some point we should update this format to the new one.
var testArchiveProcess1 = `
{
  "chainId": "vocdoni-stage-8",
  "process": {
   "processId": "c5d2460186f7f7062f413f5be0fca0abf7082ad5c7728e365002020400000001",
   "entityId": "f7062f413f5be0fca0abf7082ad5c7728e365002",
   "startBlock": 236631,
   "endBlock": 240060,
   "blockCount": 3515,
   "voteCount": 728,
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
