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
	lock    sync.Mutex
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
	entries, err := os.ReadDir(datadir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(datadir, name)
		if len(entry.Name()) != 64 {
			continue
		}
		content, err := os.ReadFile(filepath.Join(datadir, name))
		if err != nil {
			return nil, err
		}
		if content == nil {
			log.Warnf("archive file %s is empty", path)
			continue
		}
		count++
		p := &Process{}
		if err := json.Unmarshal(content, p); err != nil {
			return nil, err
		}
		eid := fmt.Sprintf("%x", p.ProcessInfo.EntityID)
		i.Entities[eid] = append(i.Entities[eid], &IndexProcess{ProcessID: p.ProcessInfo.ID})
	}
	indexData, err := json.MarshalIndent(i, " ", " ")
	if err != nil {
		return nil, err
	}
	log.Infow("archive index build", "processes", count)
	return i, os.WriteFile(filepath.Join(datadir, "index.json"), indexData, 0o644)
}

// NewProcessArchive creates a new instance of the process archiver.
// It will subscribe to Vochain events and perform the process archival.
// JSON files (one per process) will be stored within datadir.
// The key parameter must be either a valid IPFS base64 encoded private key
// or empty (a new key will be generated).
// If ipfs is nil, only JSON archive storage will be performed.
func NewProcessArchive(s *indexer.Indexer, ipfs *ipfs.Handler, datadir, key string) (*ProcessArchive, error) {
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
	log.Infow("archive scan started", "fromBlock", fromBlock, "ended", len(pids2), "results", len(pids), "ready", len(pids3))
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
		// If status is not results, we skip it.
		if procInfo.Status != int32(models.ProcessStatus_RESULTS) {
			continue
		}
		if err := pa.storage.AddProcess(&Process{
			ProcessInfo: procInfo,
			Results:     procInfo.Results(),
		}); err != nil {
			log.Warnf("processScan: %v", err)
		}
		added++
	}
	log.Infow("archive scan finished", "added", added, "took", time.Since(startTime).String())
	return nil
}

// OnComputeResults implements the indexer event callback.
// On this event the results are set always and the process info only if it
// does not exist yet in the json storage.
func (pa *ProcessArchive) OnComputeResults(results *results.Results, proc *indexertypes.Process, _ uint32) {
	// Get the process (if exist)
	jsProc, err := pa.storage.GetProcess(results.ProcessID)
	if err != nil {
		if os.IsNotExist(err) { // if it does not exist yet, we create it
			jsProc = &Process{
				ProcessInfo: proc,
			}
		} else {
			log.Errorw(err, "cannot get json store process")
			return
		}
	}
	jsProc.Results = results
	if err := pa.storage.AddProcess(jsProc); err != nil {
		log.Errorf("cannot add json process: %v", err)
		return
	}
	log.Infow("stored json archive process", "id", proc.ID.String())

	// send publish signal
	select {
	case pa.publish <- true:
	default: // do nothing
	}
}

// Close closes the process archive
func (pa *ProcessArchive) Close() {
	pa.close <- true
}
