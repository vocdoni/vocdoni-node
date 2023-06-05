package censusdb

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/data/compressor"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	censusDBprefix          = "cs_"
	censusDBreferencePrefix = "cr_"
)

var (
	// ErrCensusNotFound is returned when a census is not found in the database.
	ErrCensusNotFound = fmt.Errorf("census not found in the local database")
	// ErrCensusAlreadyExists is returned by New() if the census already exists in the database.
	ErrCensusAlreadyExists = fmt.Errorf("census already exists in the local database")
	// ErrWrongAuthenticationToken is returned when the authentication token is invalid.
	ErrWrongAuthenticationToken = fmt.Errorf("wrong authentication token")
	// ErrCensusIsLocked is returned if the census does not allow write operations.
	ErrCensusIsLocked = fmt.Errorf("census is locked")
)

// CensusRef is a reference to a census. It holds the merkle tree which can be acceded
// by calling Tree().
type CensusRef struct {
	tree       *censustree.Tree // must be private to avoid gob serialization
	AuthToken  *uuid.UUID
	CensusType int32
	URI        string
	// MaxLevels is required to load the census with the original size because
	// it could be different according to the election (and census) type.
	MaxLevels int
}

// Tree returns the censustree.Tree object of the census reference.
func (cr *CensusRef) Tree() *censustree.Tree {
	return cr.tree
}

// SetTree sets the censustree.Tree object to the census reference.
func (cr *CensusRef) SetTree(tree *censustree.Tree) {
	cr.tree = tree
}

// CensusDump is a struct that contains the data of a census. It is used
// for import/export operations.
type CensusDump struct {
	Type     models.Census_Type `json:"type"`
	RootHash []byte             `json:"rootHash"`
	Data     []byte             `json:"data"`
	// MaxLevels is required to load the census with the original size because
	// it could be different according to the election (and census) type.
	MaxLevels int `json:"maxLevels"`
}

// CensusDB is a safe and persistent database of census trees.  It allows
// authentication control over the census if a UUID token is provided.
type CensusDB struct {
	db db.Database
}

// NewCensusDB creates a new CensusDB object.
func NewCensusDB(db db.Database) *CensusDB {
	return &CensusDB{db: db}
}

// New creates a new census and adds it to the database.
func (c *CensusDB) New(censusID []byte, censusType models.Census_Type,
	uri string, authToken *uuid.UUID, maxLevels int) (*CensusRef, error) {
	if c.Exists(censusID) {
		return nil, ErrCensusAlreadyExists
	}
	tree, err := censustree.New(censustree.Options{Name: censusName(censusID),
		ParentDB: c.db, MaxLevels: maxLevels, CensusType: censusType})
	if err != nil {
		return nil, err
	}
	ref, err := c.addCensusRefToDB(censusID, authToken, censusType, uri, maxLevels)
	if err != nil {
		return nil, err
	}
	ref.tree = tree
	if uri != "" {
		ref.tree.Publish()
	}
	return ref, nil
}

// Exists returns true if the censusID exists in the local database.
func (c *CensusDB) Exists(censusID []byte) bool {
	_, err := c.getCensusRefFromDB(censusID)
	return err == nil
}

// Load returns an already loaded census from memory or from the persistent kv database.
// Authentication is checked if authToken is not nil.
func (c *CensusDB) Load(censusID []byte, authToken *uuid.UUID) (*CensusRef, error) {
	ref, err := c.getCensusRefFromDB(censusID)
	if err != nil {
		return nil, err
	}
	// check authentication
	if authToken != nil {
		// if no token stored in the reference but the called provided a token, we don't allow
		if ref.AuthToken == nil {
			return nil, ErrCensusIsLocked
		}
		if !bytes.Equal(authToken[:], ref.AuthToken[:]) {
			return nil, ErrWrongAuthenticationToken
		}
	}

	ref.tree, err = censustree.New(censustree.Options{Name: censusName(censusID), ParentDB: c.db,
		MaxLevels: ref.MaxLevels, CensusType: models.Census_Type(ref.CensusType)})
	if err != nil {
		return nil, err
	}
	if ref.URI != "" {
		ref.tree.Publish()
	}
	log.Debugf("loaded tree %x of type %s", censusID, models.Census_Type_name[ref.CensusType])
	return ref, nil
}

// Del removes a census from the database and memory.
func (c *CensusDB) Del(censusID []byte) error {
	wtx := c.db.WriteTx()
	defer wtx.Discard()
	if err := wtx.Delete(append([]byte(censusDBreferencePrefix), censusID...)); err != nil {
		return err
	}
	// the removal of the tree from the disk is done in a separate goroutine.
	// This is because the tree is locked and we don't want to block the operations,
	// and depending on the size of the tree, it can take a while to delete it.
	go func() {
		_, err := censustree.DeleteCensusTreeFromDatabase(c.db, censusName(censusID))
		if err != nil {
			log.Warnf("error deleting census tree %x: %s", censusID, err)
		}
	}()
	return wtx.Commit()
}

// BuildExportDump builds a census serialization that can be used for import.
func BuildExportDump(root, data []byte, typ models.Census_Type, maxLevels int) ([]byte, error) {
	export := CensusDump{
		Type:      typ,
		RootHash:  root,
		Data:      compressor.NewCompressor().CompressBytes(data),
		MaxLevels: maxLevels,
	}
	exportData, err := json.Marshal(export)
	if err != nil {
		return nil, err
	}
	return exportData, nil
}

// ImportAsPublic imports a census from a dump and makes it public.
func (c *CensusDB) ImportAsPublic(data []byte) error {
	cdata := CensusDump{}
	if err := json.Unmarshal(data, &cdata); err != nil {
		return fmt.Errorf("could not unmarshal census: %w", err)
	}
	if cdata.Data == nil || cdata.RootHash == nil {
		return fmt.Errorf("missing dump or root parameters")
	}
	log.Debugw("importing census", "root", hex.EncodeToString(cdata.RootHash), "type", cdata.Type.String())
	if c.Exists(cdata.RootHash) {
		return ErrCensusAlreadyExists
	}
	uri := "ipfs://" + ipfs.CalculateCIDv1json(data)
	ref, err := c.New(cdata.RootHash, cdata.Type, uri, nil, cdata.MaxLevels)
	if err != nil {
		return err
	}
	if err := ref.Tree().ImportDump(compressor.NewCompressor().DecompressBytes(cdata.Data)); err != nil {
		return err
	}
	root, err := ref.Tree().Root()
	if err != nil {
		return err
	}
	if !bytes.Equal(root, cdata.RootHash) {
		if err := c.Del(cdata.RootHash); err != nil {
			log.Warnf("could not delete census %x: %v", cdata.RootHash, err)
		}
		return fmt.Errorf("root hash does not match after importing dump")
	}

	return nil
}

// addCensusRefToDB adds a censusRef to the database.
func (c *CensusDB) addCensusRefToDB(censusID []byte, authToken *uuid.UUID,
	t models.Census_Type, uri string, maxLevels int) (*CensusRef, error) {
	wtx := c.db.WriteTx()
	defer wtx.Discard()
	refData := bytes.Buffer{}
	enc := gob.NewEncoder(&refData)
	ref := &CensusRef{
		AuthToken:  authToken,
		CensusType: int32(t),
		URI:        uri,
		MaxLevels:  maxLevels,
	}
	if err := enc.Encode(ref); err != nil {
		return nil, err
	}
	if err := wtx.Set(append([]byte(censusDBreferencePrefix), censusID...),
		refData.Bytes()); err != nil {
		return nil, err
	}
	return ref, wtx.Commit()
}

// getCensusRefFromDB returns the censusRef from the database.
func (c *CensusDB) getCensusRefFromDB(censusID []byte) (*CensusRef, error) {
	b, err := c.db.Get(
		append(
			[]byte(censusDBreferencePrefix),
			censusID...,
		))
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, ErrCensusNotFound
		}
		return nil, err
	}
	dec := gob.NewDecoder(bytes.NewReader(b))
	ref := CensusRef{}
	return &ref, dec.Decode(&ref)
}

// censusName returns the name of the census tree in the database.
func censusName(censusID []byte) string {
	return fmt.Sprintf("%s%x", censusDBprefix, censusID)
}
