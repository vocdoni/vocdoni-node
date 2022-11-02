package test

//
//import (
//	"encoding/binary"
//	"fmt"
//	"testing"
//
//	qt "github.com/frankban/quicktest"
//	"go.vocdoni.io/dvote/api/censusdb"
//	"go.vocdoni.io/dvote/db"
//	"go.vocdoni.io/dvote/db/metadb"
//	"go.vocdoni.io/dvote/test/testcommon/testutil"
//	"go.vocdoni.io/dvote/vochain"
//	"go.vocdoni.io/dvote/vochain/offchaindatahandler"
//	models "go.vocdoni.io/proto/build/go/models"
//)

//func TestRollingCensus(t *testing.T) {
//	rng := testutil.NewRandom(0)
//	app := vochain.TestBaseApplication(t)
//	app.SetTestingMethods()
//
//	db, err := metadb.New(db.TypePebble, t.TempDir())
//	qt.Assert(t, err, qt.IsNil)
//	cdb := censusdb.NewCensusDB(db)
//	defer func() { qt.Assert(t, db.Close(), qt.IsNil) }()
//	dataHandler := offchaindatahandler.NewOffChainDataHandler(app, nil, cdb, false)
//
//	const numKeys = 128
//	pid := rng.RandomBytes(32)
//
//	// Block 1
//	app.State.Rollback()
//	app.State.SetHeight(1)
//
//	censusURI := "ipfs://foobar"
//	maxCensusSize := uint64(numKeys)
//	p := &models.Process{
//		EntityId:   rng.RandomBytes(32),
//		CensusURI:  &censusURI,
//		ProcessId:  pid,
//		StartBlock: 3,
//		Mode: &models.ProcessMode{
//			PreRegister: true,
//		},
//		EnvelopeType: &models.EnvelopeType{
//			Anonymous: true,
//		},
//		MaxCensusSize: &maxCensusSize,
//	}
//	qt.Assert(t, app.State.AddProcess(p), qt.IsNil)
//
//	_, err = app.State.Save()
//	qt.Assert(t, err, qt.IsNil)
//
//	// Block 2
//	app.State.Rollback()
//	app.State.SetHeight(2)
//
//	keys := make([][]byte, numKeys)
//	for i := 0; i < numKeys; i++ {
//		keys[i] = rng.RandomInZKField()
//		qt.Assert(t, app.State.AddToRollingCensus(pid, keys[i], nil), qt.IsNil)
//	}
//
//	_, err = app.State.Save()
//	qt.Assert(t, err, qt.IsNil)
//
//	// Block 3
//	app.State.Rollback()
//	app.State.SetHeight(3)
//
//	process, err := app.State.Process(pid, true)
//	qt.Assert(t, err, qt.IsNil)
//	censusID := fmt.Sprintf("%x", process.RollingCensusRoot)
//
//	census, ok := cm.Trees[censusID]
//	qt.Assert(t, ok, qt.Equals, true)
//	for i, key := range keys {
//		indexLE, err := cm.KeyToIndex(censusID, key)
//		qt.Assert(t, err, qt.IsNil)
//		censusKey, err := census.Get(indexLE)
//		qt.Assert(t, err, qt.IsNil)
//		qt.Assert(t, censusKey, qt.DeepEquals, key)
//		index := binary.LittleEndian.Uint64(indexLE)
//		qt.Assert(t, index, qt.Equals, uint64(i))
//	}
//
//	_, err = app.State.Save()
//	qt.Assert(t, err, qt.IsNil)
//}
//
