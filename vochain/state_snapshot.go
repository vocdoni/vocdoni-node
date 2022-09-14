package vochain

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
)

const (
	snapshotHeaderVersion = 1
	snapshotHeaderLenSize = 32
)

// ErrTreeNotYetFullyRead is returned if FetchNextTree is called before the current
// tree is fully read.
var ErrTreeNotYetFullyRead = fmt.Errorf("current tree is not yet fully read")

// A StateSnapshot is a copy in a specific point in time of the blockchain state.
// The state is supposed to be a list of nested merkle trees.
// The StateSnapshot contains the methods for building a single file snapshot of
// the state containing multiple trees.
// The implementation allows the encoding and decoding of snapshots.
//
// The structure of the snapshot encoded file is:
//
//  [headerLen][header][tree1][tree2][treeN]
//
// - headerlen is a fixed 32 bytes little endian number indicating the size of the header.
//
// - header is the Gob encoded structure containing the information of the trees (size, name, etc.).
//
// - treeN is the raw bytes dump of all trees.
type StateSnapshot struct {
	path               string
	file               *os.File
	lock               sync.Mutex
	header             SnapshotHeader
	headerSize         uint32
	currentTree        int // indicates the current tree
	treeReadPointer    int // points to the last read byte
	currentTreeReadEnd int // points to the last byte of the current tree
}

// SnapshotHeader is the header structure of StateSnapshot containing the list of merkle trees.
type SnapshotHeader struct {
	Version int
	Root    []byte
	ChainID string
	Height  uint32
	Trees   []SnapshotHeaderTree
	// checksum?
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

// Open reads an existing snapshot file and decodes the header.
// After calling this method everything is ready for reading the first
// merkle tree. No need to execute `FetchNextTree` until io.EOF is reached.
//
// This method performs the opposite operation of `Create`, one of both needs
// to be called (but not both).
func (s *StateSnapshot) Open(filePath string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	var err error
	s.path = filePath
	s.file, err = os.Open(filePath)
	if err != nil {
		return err
	}
	headerSizeBytes := make([]byte, 32)
	if _, err = s.file.Read(headerSizeBytes); err != nil {
		return err
	}
	s.headerSize = binary.LittleEndian.Uint32(headerSizeBytes)
	headerBytes := make([]byte, int(s.headerSize))
	if _, err = s.file.Read(headerBytes); err != nil {
		return err
	}
	decoder := gob.NewDecoder(bytes.NewBuffer(headerBytes))
	if err := decoder.Decode(&s.header); err != nil {
		return fmt.Errorf("cannot decode header: %w", err)
	}
	s.treeReadPointer = snapshotHeaderLenSize + int(s.headerSize)
	s.currentTree--
	return s.FetchNextTree()
}

// FetchNextTree prepares everything for reading the next tree.
// It returns ErrTreeNotYetFullyRead if the current tree has still
// pending bytes for read. Returns io.EOF when there are no more trees.
func (s *StateSnapshot) FetchNextTree() error {
	if s.treeReadPointer < s.currentTreeReadEnd {
		return ErrTreeNotYetFullyRead
	}
	s.currentTree++
	if len(s.header.Trees) == s.currentTree {
		return io.EOF
	}
	s.currentTreeReadEnd = s.treeReadPointer +
		int(s.header.Trees[s.currentTree].Size)
	_, err := s.file.Seek(int64(s.treeReadPointer), 0)
	return err
}

// Read implements the io.Reader interface. Returns io.EOF error when no
// more bytes available in the current three.
func (s *StateSnapshot) Read(b []byte) (int, error) {
	if s.currentTreeReadEnd == s.treeReadPointer {
		return 0, io.EOF
	}
	// shrink b if its too big an return io.EOF
	if s.treeReadPointer+len(b) > s.currentTreeReadEnd {
		b = append([]byte{}, b[:s.currentTreeReadEnd-s.treeReadPointer]...)
		n, err := s.file.Read(b)
		s.treeReadPointer += n
		if err == nil {
			err = io.EOF
		}
		return n, err
	}
	n, err := s.file.Read(b)
	s.treeReadPointer += n
	return n, err
}

// ReadAll reads the full content of the current tree and returns its bytes.
// io.EOF error is returned if the bytes have been already read.
func (s *StateSnapshot) ReadAll() ([]byte, error) {
	if s.currentTreeReadEnd == s.treeReadPointer {
		return nil, io.EOF
	}
	b := make([]byte, s.currentTreeReadEnd-s.treeReadPointer)
	n, err := s.file.Read(b)
	s.treeReadPointer += n
	return b, err
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
// This method must be called only once and its operation is oposed to `Open`.
func (s *StateSnapshot) Create(filePath string) error {
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

	finalFile, err := os.Create(s.path)
	defer finalFile.Close()
	if err != nil {
		return err
	}

	// build the header
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(s.header); err != nil {
		return err
	}
	// write the size of the header in the first 32 bytes
	headerSize := make([]byte, 32)
	binary.LittleEndian.PutUint32(headerSize, uint32(buf.Len()))
	_, err = finalFile.Write(headerSize)
	if err != nil {
		return err
	}
	// write the header
	hs, err := finalFile.Write(buf.Bytes())
	if err != nil {
		return err
	}
	log.Debugf("snapshot header size is %d bytes", hs)

	// write the trees (by copying the tmpFile)
	if _, err := s.file.Seek(io.SeekStart, 0); err != nil {
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
	if err := os.Remove(s.file.Name()); err != nil {
		return err
	}

	return nil
}

// Write implements the io.Writer interface.
// Writes a chunck of bytes as part of the current merkle tree.
func (s *StateSnapshot) Write(b []byte) (int, error) {
	n, err := s.file.Write(b)
	s.header.Trees[s.currentTree].Size += uint32(n)
	return n, err
}

// snapshot performs a snapshot of the last commited state for all trees.
// The snapshot is stored in disk and the file path is returned.
func (v *State) snapshot() (string, error) {
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

type diskSnapshotInfo struct {
	ModTime time.Time
	Height  uint32
	Size    int64
}

// listSnapshots returns the list of the current state snapshots stored in disk.
func (v *State) listSnapshots() []diskSnapshotInfo {
	files, err := ioutil.ReadDir(filepath.Join(
		v.dataDir,
		storageDirectory,
		snapshotsDirectory))
	if err != nil {
		log.Fatal(err)
	}
	var list []diskSnapshotInfo
	for _, file := range files {
		if !file.IsDir() {
			height, err := strconv.Atoi(file.Name())
			if err != nil {
				continue
			}
			list = append(list, diskSnapshotInfo{
				Size:    file.Size(),
				ModTime: file.ModTime(),
				Height:  uint32(height),
			})
		}
	}
	return list
}
