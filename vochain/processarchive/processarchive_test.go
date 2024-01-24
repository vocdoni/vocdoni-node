package processarchive

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
)

func TestJsonStorage(t *testing.T) {
	PID1 := testutil.Hex2byte(t, "6265b1acebd3f5fe203bff097460112984edb3481a5e5ee0f16051f6c1e22098")
	EID1 := common.BytesToAddress(PID1).Bytes()
	PID2 := testutil.Hex2byte(t, "fb05ff28e37bbdd0e51bc78838bbfa9b7aba46ec077d8dfed7fda2603801ce5b")
	EID2 := common.BytesToAddress(PID2).Bytes()
	PID3 := testutil.Hex2byte(t, "c42549adc01f463317dcd24ee5267390f8a65eb2247890d98fc98445cc8f452f")
	EID3 := common.BytesToAddress(PID3).Bytes()

	dir := t.TempDir()
	js, err := NewJsonStorage(dir)
	qt.Assert(t, err, qt.IsNil)
	p1 := &indexer.ArchiveProcess{ProcessInfo: &indexertypes.Process{ID: PID1, EntityID: EID1}}
	err = js.AddProcess(p1)
	qt.Assert(t, err, qt.IsNil)
	p2 := &indexer.ArchiveProcess{ProcessInfo: &indexertypes.Process{ID: PID2, EntityID: EID2}}
	err = js.AddProcess(p2)
	qt.Assert(t, err, qt.IsNil)
	exist, err := js.ProcessExist(PID3)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, exist, qt.IsFalse)
	exist, err = js.ProcessExist(PID1)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, exist, qt.IsTrue)

	// open a second storage and check
	js2, err := NewJsonStorage(dir)
	qt.Assert(t, err, qt.IsNil)
	pr, err := js2.GetProcess(PID2)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, string(pr.ProcessInfo.EntityID), qt.DeepEquals, string(EID2))

	// create a new process on second storage
	p3 := &indexer.ArchiveProcess{ProcessInfo: &indexertypes.Process{ID: PID3, EntityID: EID3}}
	err = js2.AddProcess(p3)
	qt.Assert(t, err, qt.IsNil)

	// check it does not exist on first storage index
	_, exist = js.index.Entities[fmt.Sprintf("%x", EID3)]
	qt.Assert(t, exist, qt.IsFalse)

	// creae new storage (rebuild the index) and check entity 3 exists
	js3, err := NewJsonStorage(dir)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(js3.index.Entities), qt.Equals, 3)
	pr, err = js.GetProcess(PID3)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, string(pr.ProcessInfo.EntityID), qt.DeepEquals, string(EID3))
}
