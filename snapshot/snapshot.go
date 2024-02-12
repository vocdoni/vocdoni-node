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
	"math"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/state"
)

const (
	snapshotHeaderVersion = 1
	snapshotHeaderLenSize = 32
)

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

// DiskSnapshotInfo describes a file on disk
type DiskSnapshotInfo struct {
	Path    string
	ModTime time.Time
	Size    int64
	Hash    []byte
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
	log.Debugf("snapshot header size is %d bytes", hs)

	// write the blobs (by copying the tmpFile)
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	bs, err := io.Copy(finalFile, s.file)
	if err != nil {
		return err
	}
	log.Debugf("snapshot blobs size is %d bytes", bs)

	// close and remove the temporary file
	if err := s.file.Close(); err != nil {
		return err
	}
	if err := finalFile.Close(); err != nil {
		return err
	}
	return os.Remove(s.file.Name())
}

// Restore restores the State snapshot into a temp directory
// inside the passed dataDir, and returns the path. Caller is expected to move that tmpDir
// into the location normally used by statedb,
// or handle removing the directory, in case the returned err != nil
// It also restores the IndexerDB into dataDir/indexer (hardcoded)
func (s *Snapshot) Restore(dbType, dataDir string) (string, error) {
	log.Infof("installing snapshot %+v into dir %s using dbType %s", s.Header(), dataDir, dbType)

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

	// NoStateDB
	if err := snap.DumpNoStateDB(v); err != nil {
		return "", err
	}
	log.Debugf("dumped blob %d: %+v", len(snap.header.Blobs)-1, snap.header.Blobs[len(snap.header.Blobs)-1])

	// State
	list, err := v.DeepListStateTrees()
	if err != nil {
		return "", err
	}
	for _, treedesc := range list {
		if err := snap.DumpTree(treedesc.Name, treedesc.Parent, treedesc.Key, treedesc.Tree); err != nil {
			return "", err
		}
		log.Debugf("dumped blob %d: %+v", len(snap.header.Blobs)-1, snap.header.Blobs[len(snap.header.Blobs)-1])
	}

	// Indexer
	if FnExportIndexer() != nil {
		if err := snap.DumpIndexer(FnExportIndexer()); err != nil {
			return "", err
		}
		log.Debugf("dumped blob %d: %+v", len(snap.header.Blobs)-1, snap.header.Blobs[len(snap.header.Blobs)-1])
	}

	return snap.Path(), snap.Finish()
}

// New starts the creation of a new snapshot as a disk file.
//
// To open an existing snapshot file, use `Open` instead.
func (sm *SnapshotManager) New(height uint32) (*Snapshot, error) {
	filePath := filepath.Join(sm.dataDir, fmt.Sprintf("%d", height))
	file, err := os.Create(filePath + ".tmp")
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		path: filePath,
		file: file,
		header: SnapshotHeader{
			Version: snapshotHeaderVersion,
			hasher:  md5.New(),
		},
	}, nil
}

// Open reads an existing snapshot file, decodes the header and returns a Snapshot
// On the returned Snapshot, you are expected to call SeekToNextBlob(), read from CurrentBlobReader()
// and iterate until SeekToNextBlob() returns io.EOF.
//
// This method performs the opposite operation of `New`.
func (*SnapshotManager) Open(filePath string) (*Snapshot, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	headerSizeBytes := make([]byte, snapshotHeaderLenSize)
	if _, err = io.ReadFull(file, headerSizeBytes); err != nil {
		return nil, err
	}

	s := &Snapshot{
		path:        filePath,
		file:        file,
		headerSize:  binary.LittleEndian.Uint32(headerSizeBytes),
		currentBlob: -1, // in order for the first SeekToNextBlob seek to blob 0
	}
	decoder := gob.NewDecoder(io.LimitReader(s.file, int64(s.headerSize)))
	if err := decoder.Decode(&s.header); err != nil {
		return nil, fmt.Errorf("cannot decode header: %w", err)
	}
	if s.header.Version != snapshotHeaderVersion {
		return nil, fmt.Errorf("snapshot version not compatible")
	}
	return s, nil
}

// List returns the list of the current snapshots stored on disk, indexed by height
func (sm *SnapshotManager) List() map[uint32]DiskSnapshotInfo {
	files, err := os.ReadDir(sm.dataDir)
	if err != nil {
		log.Fatal(err)
	}
	dsi := make(map[uint32]DiskSnapshotInfo)
	for _, file := range files {
		if !file.IsDir() {
			if path.Ext(file.Name()) == ".tmp" {
				// ignore incomplete snapshots
				continue
			}
			fileInfo, err := file.Info()
			if err != nil {
				log.Errorw(err, "could not fetch snapshot file info")
				continue
			}
			s, err := sm.Open(filepath.Join(sm.dataDir, file.Name()))
			if err != nil {
				log.Errorw(err, fmt.Sprintf("could not open snapshot file %q", filepath.Join(sm.dataDir, file.Name())))
				continue
			}

			dsi[uint32(s.header.Height)] = DiskSnapshotInfo{
				Path:    filepath.Join(sm.dataDir, file.Name()),
				Size:    fileInfo.Size(),
				ModTime: fileInfo.ModTime(),
				Hash:    s.header.Hash,
			}
		}
	}
	return dsi
}

// SliceChunk returns a chunk of a snapshot
func (sm *SnapshotManager) SliceChunk(height uint64, format uint32, chunk uint32) ([]byte, error) {
	_ = format // TBD: we don't support different formats

	dsi := sm.List()

	snapshot, found := dsi[uint32(height)]
	if !found {
		return nil, fmt.Errorf("snapshot not found for height %d", height)
	}

	file, err := os.Open(snapshot.Path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	chunks := int(math.Ceil(float64(snapshot.Size) / float64(sm.ChunkSize)))
	partSize := int(math.Min(float64(sm.ChunkSize), float64(snapshot.Size-int64(chunk)*sm.ChunkSize)))
	partBuffer := make([]byte, partSize)
	if _, err := file.ReadAt(partBuffer, int64(chunk)*sm.ChunkSize); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	log.Debugf("splitting snapshot for height %d (size=%d, hash=%x), serving chunk %d of %d",
		height, snapshot.Size, snapshot.Hash, chunk, chunks)

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
