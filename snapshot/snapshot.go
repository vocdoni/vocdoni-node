package snapshot

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/state"
)

const (
	// Version is the version that is accepted
	Version               = 2
	snapshotHeaderLenSize = 32
)

const snapshotKeepNRecent = 10

const (
	snapshotBlobType_Tree = iota
	snapshotBlobType_NoStateDB
	snapshotBlobType_IndexerDB
)

const chunksDir = "chunks"

// Snapshot is a copy in a specific point in time of the blockchain state.
// The state is supposed to be a list of nested merkle trees.
// The Snapshot contains the methods for building a single file snapshot of
// the state containing multiple trees.
// The implementation allows the encoding and decoding of snapshots.
//
// The structure of the snapshot encoded file is:
//
//	[headerLen][header][blob1][blob2][blobN]
//
// - headerlen is a fixed 32 bytes little endian number indicating the size of the header.
//
// - header is the Gob encoded structure containing the metadata of the blobs (type, size, name, etc.).
//
// - blobN is the raw bytes dump of all trees and databases.
type Snapshot struct {
	path              string
	size              int64
	file              *os.File
	lock              sync.Mutex
	header            SnapshotHeader
	headerSize        uint32
	currentBlob       int       // index of the current blob being read
	currentBlobReader io.Reader // the io.Reader for reading the current blob
}

var _ io.Writer = (*Snapshot)(nil)

// SnapshotHeader is the header structure of StateSnapshot containing the list of blobs.
type SnapshotHeader struct {
	Version int
	Root    []byte
	ChainID string
	Height  uint32
	Blobs   []SnapshotBlobHeader
	Hash    []byte
	hasher  hash.Hash
}

// SnapshotBlobHeader represents a blob of data of the StateSnapshot, for example a merkle tree or a database.
type SnapshotBlobHeader struct {
	Type   int
	Name   string
	Size   uint32
	Parent string
	Key    []byte
	Root   []byte
}

func (h *SnapshotHeader) String() string {
	return fmt.Sprintf("version=%d root=%s chainID=%s height=%d blobs=%+v",
		h.Version, hex.EncodeToString(h.Root), h.ChainID, h.Height, h.Blobs)
}

// Write implements the io.Writer interface.
// Writes a chunk of bytes, updates the Size of the last s.header.Blobs[] item,
// and updates the s.header.Hash
func (s *Snapshot) Write(b []byte) (int, error) {
	if _, err := s.header.hasher.Write(b); err != nil {
		return 0, fmt.Errorf("error calculating hash: %w", err)
	}
	s.header.Hash = s.header.hasher.Sum(nil)

	n, err := s.file.Write(b)
	s.header.Blobs[len(s.header.Blobs)-1].Size += uint32(n)
	return n, err
}

// SetMainRoot sets the root for the main merkle tree of the state.
func (s *Snapshot) SetMainRoot(root []byte) {
	s.header.Root = root
}

// SetHeight sets the blockchain height for the snapshot.
func (s *Snapshot) SetHeight(height uint32) {
	s.header.Height = height
}

// SetChainID sets the blockchain identifier for the snapshot.
func (s *Snapshot) SetChainID(chainID string) {
	s.header.ChainID = chainID
}

// SeekToNextBlob seeks over the size of the current blob, leaving everything ready
// for reading the next blob.
//
// Returns io.EOF when there are no more blobs.
func (s *Snapshot) SeekToNextBlob() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.currentBlob++

	if s.currentBlob >= len(s.header.Blobs) {
		// no more blobs available
		return io.EOF
	}

	// update the buffer Reader
	s.currentBlobReader = io.LimitReader(
		s.file,
		int64(s.header.Blobs[s.currentBlob].Size),
	)
	return nil
}

// CurrentBlobReader returns the s.currentBlobReader
func (s *Snapshot) CurrentBlobReader() io.Reader {
	return s.currentBlobReader
}

// Header returns the header for the snapshot containing the information
// about all the blobs (merkle trees and DBs)
func (s *Snapshot) Header() *SnapshotHeader {
	return &s.header
}

// BlobHeader returns the header for the current blob.
func (s *Snapshot) BlobHeader() *SnapshotBlobHeader {
	return &s.header.Blobs[s.currentBlob]
}

// Path returns the file path of the snapshot file currently used.
func (s *Snapshot) Path() string {
	return s.path
}

// Path returns the size of the snapshot file.
func (s *Snapshot) Size() int64 {
	return s.size
}

// Finish builds the snapshot started with `New` and stores in disk its contents.
// After calling this method the snapshot is finished.
func (s *Snapshot) Finish() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// create the final file
	finalFile, err := os.Create(s.path)
	if err != nil {
		return err
	}

	defer func() {
		if err := finalFile.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
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

	// write the blobs (by copying the tmpFile)
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	bs, err := io.Copy(finalFile, s.file)
	if err != nil {
		return err
	}
	log.Debugw("snapshot finished", "ĥeight", s.header.Height, "headerSize", hs, "blobsSize", bs,
		"snapHash", hex.EncodeToString(s.header.Hash), "appRoot", hex.EncodeToString(s.header.Root))

	// close and remove the temporary file
	if err := s.Close(); err != nil {
		return err
	}
	if err := finalFile.Close(); err != nil {
		return err
	}
	return os.Remove(s.file.Name())
}

// Close closes the file descriptor used by the snapshot
func (s *Snapshot) Close() error {
	return s.file.Close()
}

// Restore restores the State snapshot into a temp directory
// inside the passed dataDir, and returns the path. Caller is expected to move that tmpDir
// into the location normally used by statedb,
// or handle removing the directory, in case the returned err != nil
// It also restores the IndexerDB into dataDir/indexer (hardcoded)
func (s *Snapshot) Restore(dbType, dataDir string) (string, error) {
	log.Infow("installing snapshot",
		"dataDir", dataDir, "dbType", dbType,
		"version", s.Header().Version, "blobs", len(s.Header().Blobs),
		"chainID", s.Header().ChainID, "height", s.Header().Height,
		"root", fmt.Sprintf("%x", s.Header().Root))

	tmpDir, err := os.MkdirTemp(dataDir, "newState")
	if err != nil {
		return tmpDir, fmt.Errorf("error creating temp dir: %w", err)
	}

	newState, err := state.New(dbType, tmpDir)
	if err != nil {
		return tmpDir, fmt.Errorf("error creating newState: %w", err)
	}

	for s.SeekToNextBlob() == nil {
		switch h := s.BlobHeader(); h.Type {
		case snapshotBlobType_Tree:
			log.Debugw("restoring snapshot, found a Tree blob",
				"name", h.Name,
				"root", hex.EncodeToString(h.Root),
				"parent", h.Parent,
				"key", hex.EncodeToString(h.Key))

			err := newState.RestoreStateTree(s.CurrentBlobReader(),
				state.TreeDescription{
					Name:   h.Name,
					Parent: h.Parent,
					Key:    h.Key,
				})
			if err != nil {
				return tmpDir, err
			}
		case snapshotBlobType_NoStateDB:
			log.Debug("restoring snapshot, found a NoStateDB blob, will import")
			if err := newState.ImportNoStateDB(s.CurrentBlobReader()); err != nil {
				return tmpDir, err
			}
		case snapshotBlobType_IndexerDB:
			if FnImportIndexer() == nil {
				log.Debug("restoring snapshot, found a IndexerDB blob, but there's no indexer on this node, skipping...")
				// TODO: we need to io.ReadAll because s.SeekToNextBlob() does not actually Seek, despite its name,
				// it only assumes the Seek happened because the blob was read.
				_, _ = io.ReadAll(s.CurrentBlobReader())
				continue
			}
			log.Debug("restoring snapshot, found a IndexerDB blob, will import")
			if err := FnImportIndexer()(s.CurrentBlobReader()); err != nil {
				return tmpDir, err
			}
		}
	}

	if err := newState.Commit(uint32(s.Header().Height)); err != nil {
		return tmpDir, fmt.Errorf("error doing newState.Commit(%d): %w", s.Header().Height, err)
	}

	if err := newState.Close(); err != nil {
		return tmpDir, fmt.Errorf("error closing newState: %w", err)
	}

	if importOffChainData := FnImportOffChainData(); importOffChainData != nil {
		newState, err := state.New(dbType, tmpDir)
		if err != nil {
			return tmpDir, fmt.Errorf("error reopening newState: %w", err)
		}

		if err := importOffChainData(newState); err != nil {
			return tmpDir, fmt.Errorf("error importing offchain data: %w", err)
		}

		if err := newState.Close(); err != nil {
			return tmpDir, fmt.Errorf("error reclosing newState: %w", err)
		}
	}

	return tmpDir, nil
}

// SnapshotManager is the manager
type SnapshotManager struct {
	// dataDir is the path for storing some files
	dataDir string
	// ChunkSize is the chunk size for slicing snapshots
	ChunkSize int64
}

// NewManager creates a new SnapshotManager.
func NewManager(dataDir string, chunkSize int64) (*SnapshotManager, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Join(dataDir, chunksDir), 0o750); err != nil {
		return nil, err
	}

	return &SnapshotManager{
		dataDir:   dataDir,
		ChunkSize: chunkSize,
	}, nil
}

// Do performs a snapshot of the last committed state for all trees and dbs.
// The snapshot is stored in disk and the file path is returned.
// If the snapshot finishes successfully, it will trigger a prune of old snapshots from dataDir
func (sm *SnapshotManager) Do(v *state.State) (string, error) {
	height, err := v.LastHeight()
	if err != nil {
		return "", err
	}

	snap, err := sm.New(height)
	if err != nil {
		return "", err
	}

	t := v.MainTreeView()
	root, err := t.Root()
	if err != nil {
		return "", err
	}
	snap.SetMainRoot(root)
	snap.SetHeight(height)
	snap.SetChainID(v.ChainID())

	logLastDumpedBlob := func() {
		b := snap.header.Blobs[len(snap.header.Blobs)-1]
		log.Debugw("dumped blob", "index", len(snap.header.Blobs)-1, "type", b.Type, "name", b.Name, "size", b.Size,
			"parent", b.Parent, "key", fmt.Sprintf("%x", b.Key), "root", fmt.Sprintf("%x", b.Root))
	}

	// NoStateDB
	if err := snap.DumpNoStateDB(v); err != nil {
		return "", err
	}
	logLastDumpedBlob()

	// State
	list, err := v.DeepListStateTrees()
	if err != nil {
		return "", err
	}
	// In order for the snapshot to be deterministic,
	// sort list by Name, then by Parent, and finally by Key
	sort.Slice(list, func(i, j int) bool {
		if list[i].Name != list[j].Name {
			return list[i].Name < list[j].Name
		}
		if list[i].Parent != list[j].Parent {
			return list[i].Parent < list[j].Parent
		}
		return string(list[i].Key) < string(list[j].Key)
	})
	for _, treedesc := range list {
		if err := snap.DumpTree(treedesc.Name, treedesc.Parent, treedesc.Key, treedesc.Tree); err != nil {
			return "", err
		}
		logLastDumpedBlob()
	}

	// Indexer
	if FnExportIndexer() != nil {
		if err := snap.DumpIndexer(FnExportIndexer()); err != nil {
			return "", err
		}
		logLastDumpedBlob()
	}

	if err := snap.Finish(); err != nil {
		return "", fmt.Errorf("couldn't finish snapshot: %w", err)
	}

	// Prune old snapshots
	defer func() {
		if err := sm.Prune(snapshotKeepNRecent); err != nil {
			log.Warnf("couldn't prune snapshots: %s", err)
		}
	}()

	return snap.Path(), nil
}

// New starts the creation of a new snapshot as a disk file.
//
// To open an existing snapshot file, use `Open` instead.
func (sm *SnapshotManager) New(height uint32) (*Snapshot, error) {
	filePath := filepath.Join(sm.dataDir, fmt.Sprintf("%d", height))
	tmpFile, err := os.Create(filePath + ".tmp")
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		path: filePath,
		file: tmpFile,
		header: SnapshotHeader{
			Version: Version,
			hasher:  md5.New(),
		},
	}, nil
}

// Open reads an existing snapshot file, decodes the header and returns a Snapshot.
// On the returned Snapshot, you are expected to call SeekToNextBlob(), read from CurrentBlobReader()
// and iterate until SeekToNextBlob() returns io.EOF.
//
// When done, caller should call Close()
// This method performs the opposite operation of `New`.
func (*SnapshotManager) Open(filePath string) (*Snapshot, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not fetch snapshot file info: %w", err)
	}
	headerSizeBytes := make([]byte, snapshotHeaderLenSize)
	if _, err = io.ReadFull(file, headerSizeBytes); err != nil {
		return nil, err
	}

	s := &Snapshot{
		path:        filePath,
		file:        file,
		size:        fileInfo.Size(),
		headerSize:  binary.LittleEndian.Uint32(headerSizeBytes),
		currentBlob: -1, // in order for the first SeekToNextBlob seek to blob 0
	}
	decoder := gob.NewDecoder(io.LimitReader(s.file, int64(s.headerSize)))
	if err := decoder.Decode(&s.header); err != nil {
		return nil, fmt.Errorf("cannot decode header: %w", err)
	}
	if s.header.Version > Version {
		return nil, fmt.Errorf("snapshot version %d unsupported (must be <=%d)", s.header.Version, Version)
	}
	return s, nil
}

// OpenByHeight reads an existing snapshot file, decodes the header and returns a Snapshot.
func (sm *SnapshotManager) OpenByHeight(height int64) (*Snapshot, error) {
	return sm.Open(filepath.Join(sm.dataDir, fmt.Sprintf("%d", height)))
}

// Prune removes old snapshots stored on disk, keeping the N most recent ones
func (sm *SnapshotManager) Prune(keepRecent int) error {
	files, err := os.ReadDir(sm.dataDir)
	if err != nil {
		return fmt.Errorf("cannot read dataDir: %w", err)
	}

	// Convert fs.DirEntry to FileInfo and filter out directories.
	var fileInfos []fs.FileInfo
	for _, file := range files {
		if file.IsDir() {
			continue // Skip directories
		}
		info, err := file.Info()
		if err != nil {
			return fmt.Errorf("cannot read file %s: %w", file.Name(), err)
		}
		fileInfos = append(fileInfos, info)
	}

	// Sort files by modification time, newest first.
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].ModTime().After(fileInfos[j].ModTime())
	})

	// Determine which files to delete.
	if len(fileInfos) > keepRecent {
		// Delete the older files.
		for _, file := range fileInfos[keepRecent:] {
			err := os.Remove(filepath.Join(sm.dataDir, file.Name()))
			if err != nil {
				return fmt.Errorf("cannot delete file %s: %w", file.Name(), err)
			}
			log.Debugf("pruned old snapshot %s", file.Name())
		}
	}
	return nil
}

// List returns the list of the current snapshots stored on disk, indexed by height
func (sm *SnapshotManager) List() map[uint32]*Snapshot {
	files, err := os.ReadDir(sm.dataDir)
	if err != nil {
		log.Fatal(err)
	}
	snaps := make(map[uint32]*Snapshot)
	for _, file := range files {
		if !file.IsDir() {
			if path.Ext(file.Name()) == ".tmp" {
				// ignore incomplete snapshots
				continue
			}
			s, err := sm.Open(filepath.Join(sm.dataDir, file.Name()))
			if err != nil {
				log.Errorw(err, fmt.Sprintf("could not open snapshot file %q", filepath.Join(sm.dataDir, file.Name())))
				continue
			}
			// for the list we don't need the file descriptors open
			if err := s.Close(); err != nil {
				log.Error(err)
			}
			snaps[uint32(s.header.Height)] = s
		}
	}
	return snaps
}

// SliceChunk returns a chunk of a snapshot
func (sm *SnapshotManager) SliceChunk(height uint64, format uint32, chunk uint32) ([]byte, error) {
	_ = format // TBD: we don't support different formats

	s, err := sm.OpenByHeight(int64(height))
	if err != nil {
		return nil, fmt.Errorf("snapshot not found for height %d", height)
	}
	defer s.Close()

	chunks := int(math.Ceil(float64(s.Size()) / float64(sm.ChunkSize)))
	partSize := int(math.Min(float64(sm.ChunkSize), float64(s.Size()-int64(chunk)*sm.ChunkSize)))
	partBuffer := make([]byte, partSize)
	if _, err := s.file.ReadAt(partBuffer, int64(chunk)*sm.ChunkSize); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	log.Debugf("splitting snapshot for height %d (size=%d, hash=%x), serving chunk %d of %d",
		height, s.Size(), s.header.Hash, chunk, chunks)

	return partBuffer, nil
}

// JoinChunks joins all chunkFilenames in order, and returns the resulting Snapshot
func (sm *SnapshotManager) JoinChunks(chunks int32, height int64) (*Snapshot, error) {
	snapshotFilename := filepath.Join(sm.dataDir, fmt.Sprintf("%d", height))

	log.Debugf("joining %d chunks into snapshot (height %d) file %s", chunks, height, snapshotFilename)
	// Create or truncate the destination file
	if _, err := os.Create(snapshotFilename); err != nil {
		return nil, err
	}

	// Open for APPEND
	snapFile, err := os.OpenFile(snapshotFilename, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return nil, err
	}

	// according to https://socketloop.com/tutorials/golang-recombine-chunked-files-example
	// we shouldn't defer a file.Close when opening a file for APPEND mode, but no idea why
	defer snapFile.Close()

	// we can't use range chunkFilenames since that would restore in random order
	for i := int32(0); i < chunks; i++ {
		file := filepath.Join(sm.dataDir, chunksDir, fmt.Sprintf("%d", i))
		// open a chunk
		chunk, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("error reading chunk file: %w", err)
		}

		if _, err := snapFile.Write(chunk); err != nil {
			return nil, fmt.Errorf("error writing chunk bytes: %w", err)
		}

		// flush to disk
		if err := snapFile.Sync(); err != nil {
			return nil, fmt.Errorf("error flushing chunk to disk: %w", err)
		}

		// remove the chunk file
		if err := os.Remove(file); err != nil {
			return nil, fmt.Errorf("error removing chunk file: %w", err)
		}
	}

	s, err := sm.Open(snapFile.Name())
	if err != nil {
		return nil, fmt.Errorf("error opening resulting snapshot: %w", err)
	}

	log.Debugf("snapshot file %s has header.Hash=%x", snapFile.Name(), s.header.Hash)
	// TODO: hash the contents and check it matches the s.header.Hash, confirming no corruption

	return s, nil
}

// WriteChunkToDisk writes the chunk bytes into a file (named according to index) inside the chunk storage dir
//
// When all chunks are on disk, you are expected to call sm.JoinChunks
func (sm *SnapshotManager) WriteChunkToDisk(index uint32, chunk []byte) error {
	f, err := os.Create(filepath.Join(sm.dataDir, chunksDir, fmt.Sprintf("%d", index)))
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(chunk); err != nil {
		if err := os.Remove(f.Name()); err != nil {
			log.Warnf("couldn't remove temp file %s: %s", f.Name(), err)
		}
		return err
	}
	return nil
}

// CountChunksInDisk counts how many files are present inside the chunk storage dir
func (sm *SnapshotManager) CountChunksInDisk() int {
	files, err := os.ReadDir(filepath.Join(sm.dataDir, chunksDir))
	if err != nil {
		return 0
	}
	return len(files)
}
