package processarchive

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/proto/build/go/models"
)

type ProcessArchive struct {
	indexer    *indexer.Indexer
	ipfs       *ipfs.Handler
	storage    *JsonStorage
	publish    chan bool
	lastUpdate time.Time
	close      chan bool
}

type Process struct {
	ChainID     string                `json:"chainId,omitempty"`
	ProcessInfo *indexertypes.Process `json:"process"`
	Results     *results.Results      `json:"results"`
	StartDate   *time.Time            `json:"startDate,omitempty"`
	EndDate     *time.Time            `json:"endDate,omitempty"`
}

type Index struct {
	Entities map[string][]*IndexProcess `json:"entities"`
}

type IndexProcess struct {
	ProcessID types.HexBytes `json:"processId"`
}

type JsonStorage struct {
	datadir string
	lock    sync.RWMutex
	index   *Index
}

// NewJsonStorage opens a new jsonStorage file at the location provided by datadir
func NewJsonStorage(datadir string) (*JsonStorage, error) {
	err := os.MkdirAll(datadir, 0o750)
	if err != nil {
		return nil, err
	}
	i, err := BuildIndex(datadir)
	return &JsonStorage{datadir: datadir, index: i}, err
}

// AddProcess adds an entire process to js
func (js *JsonStorage) AddProcess(p *Process) error {
	if p == nil || p.ProcessInfo == nil || len(p.ProcessInfo.ID) != types.ProcessIDsize {
		return fmt.Errorf("process not valid")
	}
	procData, err := json.MarshalIndent(p, " ", " ")
	if err != nil {
		return err
	}
	js.lock.Lock()
	defer js.lock.Unlock()
	procPath := filepath.Join(js.datadir, fmt.Sprintf("%x", p.ProcessInfo.ID))

	// If process file does not exist it means it is a new process, so we add it to the index
	if _, err := os.Stat(procPath); errors.Is(err, os.ErrNotExist) {
		eid := fmt.Sprintf("%x", p.ProcessInfo.EntityID)
		if js.index.Entities[eid] == nil {
			js.index.Entities[eid] = []*IndexProcess{}
		}
		js.index.Entities[eid] = append(js.index.Entities[eid], &IndexProcess{ProcessID: p.ProcessInfo.ID})
		indexData, err := json.MarshalIndent(js.index, " ", " ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(js.datadir, "index.json"), indexData, 0o644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	// TO-DO: use https://github.com/google/renameio
	return os.WriteFile(procPath, procData, 0o644)
}

// GetProcess retrieves a process from the js storage
func (js *JsonStorage) GetProcess(pid []byte) (*Process, error) {
	if len(pid) != types.ProcessIDsize {
		return nil, fmt.Errorf("process not valid")
	}
	js.lock.Lock()
	defer js.lock.Unlock()
	data, err := os.ReadFile(filepath.Join(js.datadir, fmt.Sprintf("%x", pid)))
	if err != nil {
		return nil, err
	}
	p := &Process{}
	return p, json.Unmarshal(data, p)
}

// ProcessExist returns true if a process already existin in the storage
func (js *JsonStorage) ProcessExist(pid []byte) (bool, error) {
	js.lock.Lock()
	defer js.lock.Unlock()
	if _, err := os.Stat(filepath.Join(js.datadir, fmt.Sprintf("%x", pid))); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// BuildIndex scans the archive directory and builds the JSON storage index
func BuildIndex(datadir string) (*Index, error) {
	i := &Index{
		Entities: make(map[string][]*IndexProcess),
	}
	count := 0
	if err := filepath.Walk(datadir, func(path string, info os.FileInfo, err error) error {
		if len(info.Name()) == 64 {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if content == nil {
				log.Warnf("archive file %s is empty", path)
				return nil
			}
			count++
			p := &Process{}
			if err := json.Unmarshal(content, p); err != nil {
				return err
			}
			eid := fmt.Sprintf("%x", p.ProcessInfo.EntityID)
			i.Entities[eid] = append(i.Entities[eid], &IndexProcess{ProcessID: p.ProcessInfo.ID})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	indexData, err := json.MarshalIndent(i, " ", " ")
	if err != nil {
		return nil, err
	}
	log.Infof("archive index build with %d processes", count)
	return i, os.WriteFile(filepath.Join(datadir, "index.json"), indexData, 0o644)
}

// NewProcessArchive creates a new instance of the process archiver.
// It will subscribe to Vochain events and perform the process archival.
// JSON files (one per process) will be stored within datadir.
// The key parameter must be either a valid IPFS base64 encoded private key
// or empty (a new key will be generated).
// If ipfs is nil, only JSON archive storage will be performed.
func NewProcessArchive(s *indexer.Indexer, ipfs *ipfs.Handler,
	datadir, key string) (*ProcessArchive, error) {
	js, err := NewJsonStorage(datadir)
	if err != nil {
		return nil, fmt.Errorf("could not create process archive: %w", err)
	}
	ir := &ProcessArchive{
		indexer: s,
		ipfs:    ipfs,
		storage: js,
		publish: make(chan bool, 1),
		close:   make(chan bool, 1), // TO-DO: use a context
	}

	// Perform an initial scan to add previous processes
	// This might not be required since fast-sync should be able to find all processes,
	// but for security reasons we perform this redundant scan over the previous processes.
	// For instance, if the process archive format is changed, without this initial scan
	// the node should synchronize the blockchain from scratch.
	if err := ir.ProcessScan(0); err != nil {
		return nil, err
	}

	if ipfs != nil {
		if err := ir.AddKey(key); err != nil {
			return nil, err
		}
		if pk, err := ir.GetKey(); err != nil {
			return nil, err
		} else {
			log.Infof("using IPNS privkey: %s", pk)
		}
		ir.lastUpdate = time.Unix(1, 0)
		ir.publish <- true
		go ir.publishLoop()
	}
	// Subscribe to events for new processes
	s.AddEventListener(ir)

	return ir, nil
}

// ProcessScan search for previous (not added) processes to the archive and adds them.
// This method is build for being executed only once (when bootstraping) since new
// processes will automatically be added to the archive by event callbacks.
func (pa *ProcessArchive) ProcessScan(fromBlock int) error {
	startTime := time.Now()
	// Processes with on-chain results
	pids, err := pa.indexer.ProcessList(nil, fromBlock,
		int(pa.indexer.CountTotalProcesses()), "", 0, 0, "RESULTS", true)
	if err != nil {
		return err
	}
	// Processes finished by transaction without (yet?) on-chain results
	pids2, err := pa.indexer.ProcessList(nil, fromBlock,
		int(pa.indexer.CountTotalProcesses()), "", 0, 0, "ENDED", true)
	if err != nil {
		return err
	}
	// Processes finished by endBlock without (yet?) on-chain results
	pids3, err := pa.indexer.ProcessList(nil, fromBlock,
		int(pa.indexer.CountTotalProcesses()), "", 0, 0, "READY", true)
	if err != nil {
		return err
	}

	log.Infof("scanning blockchain processes from block %d (ended:%d results:%d ready:%d)",
		fromBlock, len(pids2), len(pids), len(pids3))
	added := 0
	for _, p := range append(append(pids, pids2...), pids3...) {
		exists, err := pa.storage.ProcessExist(p)
		if err != nil {
			log.Warnf("processScan: %v", err)
			continue
		}
		if exists {
			continue
		}
		procInfo, err := pa.indexer.ProcessInfo(p)
		if err != nil {
			return err
		}
		// If status is READY but the process is not yet finished, ignore
		if procInfo.Status == int32(models.ProcessStatus_READY) {
			if pa.indexer.App.Height() < procInfo.EndBlock {
				continue
			}
		}
		results := procInfo.Results()
		if err := pa.storage.AddProcess(&Process{
			ProcessInfo: procInfo,
			Results:     results,
			StartDate:   pa.indexer.App.TimestampFromBlock(int64(procInfo.StartBlock)),
			EndDate:     pa.indexer.App.TimestampFromBlock(int64(results.BlockHeight)),
			ChainID:     pa.indexer.App.ChainID(),
		}); err != nil {
			log.Warnf("processScan: %v", err)
		}
		added++
	}
	log.Infof("archive scan added %d archive processes, took %s", added, time.Since(startTime))
	return nil
}

// OnComputeResults implements the indexer event callback.
// On this event the results are set always and the process info only if it
// does not exist yet in the json storage.
func (pa *ProcessArchive) OnComputeResults(results *results.Results,
	proc *indexertypes.Process, height uint32) {
	// Get the process (if exist)
	jsProc, err := pa.storage.GetProcess(results.ProcessID)
	if err != nil {
		if os.IsNotExist(err) { // if it does not exist yet, we create it
			jsProc = &Process{
				ProcessInfo: proc,
				Results:     results,
				ChainID:     pa.indexer.App.ChainID(),
			}
		} else {
			log.Errorf("cannot get json store process: %v", err)
			return
		}
	}
	jsProc.Results = results
	jsProc.StartDate = pa.indexer.App.TimestampFromBlock(int64(proc.StartBlock))
	jsProc.EndDate = pa.indexer.App.TimestampFromBlock(int64(results.BlockHeight))
	if err := pa.storage.AddProcess(jsProc); err != nil {
		log.Errorf("cannot add json process: %v", err)
		return
	}
	log.Infof("stored json process %x for compute results event", proc.ID)

	// send publish signal
	log.Debugf("sending archive publish signal for height %d", height)
	select {
	case pa.publish <- true:
	default: // do nothing
	}
}

// Close closes the process archive
func (pa *ProcessArchive) Close() {
	pa.close <- true
}
