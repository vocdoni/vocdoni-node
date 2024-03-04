package state

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/tree/arbo"
)

const (
	snapshotHeaderVersion = 1
	snapshotHeaderLenSize = 32
)

// StateSnapshot is a copy in a specific point in time of the blockchain state.
// The state is supposed to be a list of nested merkle trees.
// The StateSnapshot contains the methods for building a single file snapshot of
// the state containing multiple trees.
// The implementation allows the encoding and decoding of snapshots.
//
// The structure of the snapshot encoded file is:
//
//	[headerLen][header][noState][tree1][tree2][treeN]
//
// - headerlen is a fixed 32 bytes little endian number indicating the size of the header.
//
// - header is the Gob encoded structure containing the information of the trees (size, name, etc.).
//
// - treeN is the raw bytes dump of all trees.
type StateSnapshot struct {
	path                  string
	file                  *os.File
	lock                  sync.Mutex
	header                SnapshotHeader
	headerSize            uint32
	currentTree           int       // indicates the current tree
	currentTreeReadBuffer io.Reader // the buffer for reading the current tree
}

// SnapshotHeader is the header structure of StateSnapshot containing the list of merkle trees.
type SnapshotHeader struct {
	Version     int
	Root        []byte
	ChainID     string
	Height      uint32
	NoStateSize uint32
	Trees       []SnapshotHeaderTree
}

// SnapshotHeaderTree represents a merkle tree of the StateSnapshot.
type SnapshotHeaderTree struct {
	Name   string
	Size   uint32
	Parent string
	Root   []byte
}

// SetMainRoot sets the root for the mail merkle tree of the state.
func (s *StateSnapshot) SetMainRoot(root []byte) {
	s.header.Root = root
}

// SetHeight sets the blockchain height for the snapshot.
func (s *StateSnapshot) SetHeight(height uint32) {
	s.header.Height = height
}

// SetChainID sets the blockchain identifier for the snapshot.
func (s *StateSnapshot) SetChainID(chainID string) {
	s.header.ChainID = chainID
}

// SetNoStateSize sets the noState database size
func (s *StateSnapshot) SetNoStateSize(size uint32) {
	s.header.NoStateSize = size
}

// Open reads an existing snapshot file and decodes the header.
// After calling this method everything is ready for reading the first
// merkle tree. No need to execute `FetchNextTree` until io.EOF is reached.
//
// This method performs the opposite operation of `Create`, one of both needs
// to be called (but not both).
func (s *StateSnapshot) Open(filePath string) error {
	var err error
	s.path = filePath
	s.file, err = os.Open(filePath)
	if err != nil {
		return err
	}
	headerSizeBytes := make([]byte, snapshotHeaderLenSize)
	if _, err = io.ReadFull(s.file, headerSizeBytes); err != nil {
		return err
	}
	s.headerSize = binary.LittleEndian.Uint32(headerSizeBytes)
	if _, err := s.file.Seek(snapshotHeaderLenSize, 0); err != nil {
		return err
	}
	decoder := gob.NewDecoder(io.LimitReader(s.file, int64(s.headerSize)))
	if err := decoder.Decode(&s.header); err != nil {
		return fmt.Errorf("cannot decode header: %w", err)
	}
	if s.header.Version != snapshotHeaderVersion {
		return fmt.Errorf("snapshot version not compatible")
	}
	// we call FetchNextTree which increase s.currentTree, so we set
	// currentTree to -1 value (thus the first tree with index 0 is loaded)
	s.currentTree--
	return s.FetchNextTree()
}

// FetchNextTree prepares everything for reading the next tree.
// Returns io.EOF when there are no more trees.
func (s *StateSnapshot) FetchNextTree() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// check if no more trees available
	if s.currentTree >= len(s.header.Trees)-1 {
		return io.EOF
	}
	s.currentTree++

	// move the pointer is in the start of the next tree
	seekPointer := snapshotHeaderLenSize + int(s.headerSize)
	for i := 0; i < s.currentTree; i++ {
		seekPointer += int(s.header.Trees[i].Size)
	}
	_, err := s.file.Seek(int64(seekPointer), io.SeekStart)

	// update the buffer Reader
	s.currentTreeReadBuffer = io.LimitReader(
		s.file,
		int64(s.header.Trees[s.currentTree].Size),
	)
	return err
}

// Read implements the io.Reader interface. Returns io.EOF error when no
// more bytes available in the current three.
func (s *StateSnapshot) Read(b []byte) (int, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.currentTreeReadBuffer.Read(b)
}

// ReadAll reads the full content of the current tree and returns its bytes.
// io.EOF error is returned if the bytes have been already read.
func (s *StateSnapshot) ReadAll() ([]byte, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	b, err := io.ReadAll(s.currentTreeReadBuffer)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, io.EOF
	}
	return b, nil
}

// Header returns the header for the snapshot containing the information
// about all the merkle trees.
func (s *StateSnapshot) Header() *SnapshotHeader {
	return &s.header
}

// TreeHeader returns the header for the current tree.
func (s *StateSnapshot) TreeHeader() *SnapshotHeaderTree {
	return &s.header.Trees[s.currentTree]
}

// Path returns the file path of the snapshot file currently used.
func (s *StateSnapshot) Path() string {
	return s.path
}

// Create starts the creation of a new snapshot as a disk file.
// This method must be called only once and its operation is opposed to `Open`.
func (s *StateSnapshot) Create(filePath string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	var err error
	s.header.Version = snapshotHeaderVersion
	s.file, err = os.Create(filePath + ".tmp")
	s.path = filePath
	return err
}

// AddTree adds a new tree to the snapshot. `Create` needs to be called first.
func (s *StateSnapshot) AddTree(name, parent string, root []byte) {
	s.lock.Lock() // only 1 tree at time is allowed
	s.header.Trees = append(s.header.Trees, SnapshotHeaderTree{
		Name:   name,
		Parent: parent,
		Root:   root,
		Size:   0,
	})
}

// EndTree finishes the addition of a tree. This method should be called after `AddTree`.
func (s *StateSnapshot) EndTree() {
	s.currentTree++
	s.lock.Unlock()
}

// Save builds the snapshot started with `Create` and stores in disk its contents.
// After calling this method the snapshot is finished.
// `EndTree` must be called before saving.
func (s *StateSnapshot) Save() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// create the final file
	finalFile, err := os.Create(s.path)
	if err != nil {
		return err
	}

	defer func() {
		if err := finalFile.Close(); err != nil {
			log.Warnf("error closing the file %v", err)
		}
	}()

	// build the header
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(s.header); err != nil {
		return err
	}

	// write the size of the header in the first 32 bytes
	headerSize := make([]byte, snapshotHeaderLenSize)
	binary.LittleEndian.PutUint32(headerSize, uint32(buf.Len()))
	if _, err := finalFile.Write(headerSize); err != nil {
		return err
	}

	// write the header
	hs, err := finalFile.Write(buf.Bytes())
	if err != nil {
		return err
	}
	log.Debugf("snapshot header size is %d bytes", hs)

	// write the trees (by copying the tmpFile)
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	bs, err := io.Copy(finalFile, s.file)
	if err != nil {
		return err
	}
	log.Debugf("snapshot trees size is %d bytes", bs)

	// close and remove the temporary file
	if err := s.file.Close(); err != nil {
		return err
	}
	if err := finalFile.Close(); err != nil {
		return err
	}
	return os.Remove(s.file.Name())
}

// Write implements the io.Writer interface.
// Writes a chunk of bytes as part of the current merkle tree.
func (s *StateSnapshot) Write(b []byte) (int, error) {
	n, err := s.file.Write(b)
	s.header.Trees[s.currentTree].Size += uint32(n)
	return n, err
}

// Snapshot performs a snapshot of the last committed state for all trees.
// The snapshot is stored in disk and the file path is returned.
func (v *State) Snapshot() (string, error) {
	t := v.MainTreeView()
	height, err := v.LastHeight()
	if err != nil {
		return "", err
	}
	root, err := t.Root()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Join(
		v.dataDir,
		storageDirectory,
		snapshotsDirectory), 0750); err != nil {
		return "", err
	}

	var snap StateSnapshot
	if err := snap.Create(filepath.Join(
		v.dataDir,
		storageDirectory,
		snapshotsDirectory,
		fmt.Sprintf("%d", height),
	)); err != nil {
		return "", err
	}
	snap.SetMainRoot(root)
	snap.SetHeight(height)
	snap.SetChainID(v.chainID)

	noStateSize, err := ExportNoStateDB(&snap, v.NoState(false))
	if err != nil {
		return "", err
	}
	snap.SetNoStateSize(noStateSize)

	dumpTree := func(name, parent string, tr statedb.TreeViewer) error {
		root, err := tr.Root()
		if err != nil {
			return err
		}
		snap.AddTree(name, parent, root)
		if err := tr.Dump(&snap); err != nil {
			return fmt.Errorf("cannot dump tree: %w", err)
		}
		snap.EndTree()
		return nil
	}

	// dump main tree
	if err := dumpTree("Main", "", v.mainTreeViewer(true)); err != nil {
		return "", err
	}

	// dump main subtrees
	for k := range MainTrees {
		t, err := v.mainTreeViewer(true).SubTree(StateTreeCfg(k))
		if err != nil {
			return "", err
		}
		if err := dumpTree(k, "", t); err != nil {
			return "", err
		}
	}

	// dump child trees that depend on process
	pids, err := v.ListProcessIDs(true)
	if err != nil {
		return "", err
	}
	log.Debugf("found %d processes", len(pids))
	for name := range ChildTrees {
		for _, p := range pids {
			childTreeCfg := StateChildTreeCfg(name)
			processTree, err := v.mainTreeViewer(true).SubTree(StateTreeCfg(TreeProcess))
			if err != nil {
				return "", fmt.Errorf("cannot load process tree: %w", err)
			}
			childTree, err := processTree.SubTree(childTreeCfg.WithKey(p))
			if err != nil {
				// key might not exist (i.e process does not have census)
				if !errors.Is(err, arbo.ErrKeyNotFound) &&
					!errors.Is(err, ErrProcessChildLeafRootUnknown) &&
					!errors.Is(err, statedb.ErrEmptyTree) {
					return "", fmt.Errorf("child tree (%s) cannot be loaded with key %x: %w", name, p, err)
				}
				continue
			}
			if err := dumpTree(name, TreeProcess, childTree); err != nil {
				return "", err
			}
		}
	}
	return snap.Path(), snap.Save()
}

/*
// TODO install Snapshot
func (v *State) installSnapshot(height uint32) error {
	var snap StateSnapshot
	if err := snap.Open(filepath.Join(
		v.dataDir,
		storageDirectory,
		snapshotsDirectory,
		fmt.Sprintf("%d", height),
	)); err != nil {
		return err
	}
	log.Infof("installing snapshot %+v", snap.Header())
	return fmt.Errorf("unimplemented")
}
*/

type DiskSnapshotInfo struct {
	ModTime time.Time
	Height  uint32
	Size    int64
}

// ListSnapshots returns the list of the current state snapshots stored in disk.
func (v *State) ListSnapshots() []DiskSnapshotInfo {
	files, err := os.ReadDir(filepath.Join(
		v.dataDir,
		storageDirectory,
		snapshotsDirectory))
	if err != nil {
		log.Fatal(err)
	}
	var list []DiskSnapshotInfo
	for _, file := range files {
		if !file.IsDir() {
			height, err := strconv.Atoi(file.Name())
			if err != nil {
				log.Errorw(err, "could not list snapshot file height")
				continue
			}
			fileInfo, err := file.Info()
			if err != nil {
				log.Errorw(err, "could not list snapshot file")
				continue
			}
			list = append(list, DiskSnapshotInfo{
				Size:    fileInfo.Size(),
				ModTime: fileInfo.ModTime(),
				Height:  uint32(height),
			})
		}
	}
	return list
}

// DBPair is a key value pair for the no state db.
type DBPair struct {
	Key   []byte
	Value []byte
}

// ExportNoStateDB exports the no state db to a gob encoder and writes it to the given writer.
func ExportNoStateDB(w io.Writer, reader *NoState) (uint32, error) {
	pairs := []DBPair{}
	err := reader.Iterate(nil, func(key []byte, value []byte) bool {
		pairs = append(pairs, DBPair{Key: bytes.Clone(key), Value: bytes.Clone(value)})
		return true
	})
	if err != nil {
		return 0, err
	}
	cw := &countingWriter{w: w}
	enc := gob.NewEncoder(cw)
	return cw.n, enc.Encode(pairs)
}

// ImportNoStateDB imports the no state db from a gob decoder and writes it to the given db updater.
func ImportNoStateDB(r io.Reader, db *NoState) error {
	dec := gob.NewDecoder(r)
	var pairs []DBPair
	if err := dec.Decode(&pairs); err != nil {
		return err
	}
	for _, pair := range pairs {
		if err := db.Set(pair.Key, pair.Value); err != nil {
			return err
		}
	}
	return nil
}

type countingWriter struct {
	w io.Writer
	n uint32
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += uint32(n)
	return n, err
}
