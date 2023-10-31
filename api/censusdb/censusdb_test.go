package censusdb

import (
	"bytes"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid" // This is a helper library for cleaner test assertions.
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/proto/build/go/models"
)

func newDatabase(t *testing.T) db.Database {
	// For the sake of this test, we are creating an in-memory database.
	// This should be replaced with your actual database initialization.
	return metadb.NewTest(t)
}

func TestNewCensusDB(t *testing.T) {
	db := newDatabase(t)
	censusDB := NewCensusDB(db)
	qt.Assert(t, censusDB, qt.IsNotNil)
	qt.Assert(t, censusDB.db, qt.IsNotNil)
}

func TestCensusDBNew(t *testing.T) {
	censusDB := NewCensusDB(newDatabase(t))
	censusID := []byte("testCensus")
	censusType := models.Census_ARBO_BLAKE2B
	uri := "testURI"
	authToken, _ := uuid.NewRandom()
	maxLevels := 32

	censusRef, err := censusDB.New(censusID, censusType, uri, &authToken, maxLevels)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, censusRef, qt.IsNotNil)
	qt.Assert(t, censusRef.Tree(), qt.IsNotNil)
}

func TestCensusDBExists(t *testing.T) {
	censusDB := NewCensusDB(newDatabase(t))
	censusID := []byte("testCensus")

	existsBefore := censusDB.Exists(censusID)
	qt.Assert(t, existsBefore, qt.IsFalse)

	// Create a new census to test existence
	_, err := censusDB.New(censusID, models.Census_ARBO_BLAKE2B, "", nil, 32)
	qt.Assert(t, err, qt.IsNil)

	existsAfter := censusDB.Exists(censusID)
	qt.Assert(t, existsAfter, qt.IsTrue)
}

func TestCensusDBLoad(t *testing.T) {
	censusDB := NewCensusDB(newDatabase(t))
	censusID := []byte("testCensus")
	authToken, _ := uuid.NewRandom()

	// Attempting to load a non-existent census should return an error
	_, err := censusDB.Load(censusID, &authToken)
	censusDB.UnLoad()
	qt.Assert(t, err, qt.IsNotNil)

	// Create a new census for loading
	_, err = censusDB.New(censusID, models.Census_ARBO_BLAKE2B, "", &authToken, 32)
	qt.Assert(t, err, qt.IsNil)

	// Load the census with the correct auth token
	censusRef, err := censusDB.Load(censusID, &authToken)
	censusDB.UnLoad()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, censusRef, qt.IsNotNil)
}

func TestCensusDBDel(t *testing.T) {
	censusDB := NewCensusDB(newDatabase(t))
	censusID := []byte("testCensus")

	// Create a new census for deletion
	_, err := censusDB.New(censusID, models.Census_ARBO_BLAKE2B, "", nil, 32)
	qt.Assert(t, err, qt.IsNil)

	// Delete the census
	err = censusDB.Del(censusID)
	qt.Assert(t, err, qt.IsNil)

	time.Sleep(1 * time.Second) // Wait because deletion is async

	// Ensure it's deleted
	existsAfter := censusDB.Exists(censusID)
	qt.Assert(t, existsAfter, qt.IsFalse)
}

func TestImportCensusDB(t *testing.T) {
	censusDB := NewCensusDB(newDatabase(t))
	token1 := uuid.New()
	censusID1 := []byte("testCensus1")
	token2 := uuid.New()
	censusID2 := []byte("testCensus2")

	// Create census2
	censusRef, err := censusDB.New(censusID1, models.Census_ARBO_BLAKE2B, "", &token1, 32)
	qt.Assert(t, err, qt.IsNil)

	// Populate the census with mock data
	mockData1 := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
	}

	for k, v := range mockData1 {
		err = censusRef.Tree().Add([]byte(k), v)
		qt.Assert(t, err, qt.IsNil)
	}

	// Create census2
	censusRef, err = censusDB.New(censusID2, models.Census_ARBO_BLAKE2B, "", &token2, 32)
	qt.Assert(t, err, qt.IsNil)

	// Populate the census with mock data
	mockData2 := map[string][]byte{
		"key3": []byte("value3"),
		"key4": []byte("value4"),
	}

	for k, v := range mockData2 {
		err = censusRef.Tree().Add([]byte(k), v)
		qt.Assert(t, err, qt.IsNil)
	}

	// Ensure the data is in the database
	_, err = censusDB.Load(censusID1, &token1)
	censusDB.UnLoad()
	qt.Assert(t, err, qt.IsNil)
	_, err = censusDB.Load(censusID2, &token2)
	censusDB.UnLoad()
	qt.Assert(t, err, qt.IsNil)

	// Dump the census data to a byte buffer
	var buf bytes.Buffer
	err = censusDB.ExportCensusDB(&buf)
	qt.Assert(t, err, qt.IsNil)

	// Create a new database instance to simulate importing into a fresh database
	newDB := NewCensusDB(newDatabase(t))

	// Import the data from the buffer
	err = newDB.ImportCensusDB(&buf)
	qt.Assert(t, err, qt.IsNil)

	// Verify the data in the new database
	newCensusRef, err := newDB.Load(censusID1, &token1)
	qt.Assert(t, err, qt.IsNil)

	for k, v := range mockData1 {
		value, err := newCensusRef.Tree().Get([]byte(k))
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, v, qt.DeepEquals, value)
	}
	newDB.UnLoad()
	newCensusRef, err = newDB.Load(censusID2, &token2)
	qt.Assert(t, err, qt.IsNil)

	for k, v := range mockData2 {
		value, err := newCensusRef.Tree().Get([]byte(k))
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, v, qt.DeepEquals, value)
	}
	newDB.UnLoad()
}
