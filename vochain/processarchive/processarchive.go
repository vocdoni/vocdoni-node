package processarchive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
	"go.vocdoni.io/proto/build/go/models"
)

type ProcessArchive struct {
	indexer    *scrutinizer.Scrutinizer
	ipfs       *data.IPFSHandle
	storage    *jsonStorage
	publish    chan (bool)
	lastUpdate time.Time
	close      chan (bool)
}

type Process struct {
	ProcessInfo *indexertypes.Process `json:"process"`
	Results     *indexertypes.Results `json:"results"`
}

type jsonStorage struct {
	datadir string
	lock    sync.RWMutex
}

// NewJsonStorage opens a new jsonStorage file at the location provided by datadir
func NewJsonStorage(datadir string) (*jsonStorage, error) {
	err := os.MkdirAll(datadir, 0o750)
	if err != nil {
		return nil, err
	}
	return &jsonStorage{datadir: datadir}, nil
}

// AddProcess adds an entire process to js
func (js *jsonStorage) AddProcess(p *Process) error {
	if p == nil || p.ProcessInfo == nil || len(p.ProcessInfo.ID) != types.ProcessIDsize {
		return fmt.Errorf("process not valid")
	}
	data, err := json.MarshalIndent(p, "", "\t")
	if err != nil {
		return err
	}
	js.lock.Lock()
	defer js.lock.Unlock()
	// TO-DO: use https://github.com/google/renameio
	return os.WriteFile(filepath.Join(js.datadir, fmt.Sprintf("%x", p.ProcessInfo.ID)), data, 0o644)
}

// GetProcess retreives a process from the js storage
func (js *jsonStorage) GetProcess(pid []byte) (*Process, error) {
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
func (js *jsonStorage) ProcessExist(pid []byte) (bool, error) {
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

// NewProcessArchive creates a new instance of the process archiver.
// It will subscribe to Vochain events and perform the process archival.
// JSON files (one per process) will be stored within datadir.
// The key parameter must be either a valid IPFS base64 encoded private key
// or empty (a new key will be generated).
// If ipfs is nil, only JSON archive storage will be performed.
func NewProcessArchive(s *scrutinizer.Scrutinizer, ipfs *data.IPFSHandle,
	datadir, key string) (*ProcessArchive, error) {
	js, err := NewJsonStorage(datadir)
	if err != nil {
		return nil, fmt.Errorf("could not create process archive: %w", err)
	}
	ir := &ProcessArchive{
		indexer: s,
		ipfs:    ipfs,
		storage: js,
		publish: make(chan (bool), 1),
		close:   make(chan (bool), 1), // TO-DO: use a context
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
	pids, err := pa.indexer.ProcessList(nil, fromBlock,
		int(pa.indexer.ProcessCount(nil)), "", 0, "", "RESULTS", true)
	if err != nil {
		return err
	}
	log.Infof("scanning blockchain processes from block %d", fromBlock)
	added := 0
	startTime := time.Now()
	for _, p := range pids {
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
		results, err := pa.indexer.GetResults(p)
		if err != nil {
			return err
		}
		if err := pa.storage.AddProcess(&Process{ProcessInfo: procInfo, Results: results}); err != nil {
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
func (pa *ProcessArchive) OnComputeResults(results *indexertypes.Results,
	proc *indexertypes.Process, height uint32) {
	// Get the process (if exist)
	jsProc, err := pa.storage.GetProcess(results.ProcessID)
	if err != nil {
		if os.IsNotExist(err) { // if it does not exist yet, we create it
			jsProc = &Process{ProcessInfo: proc, Results: results}
		} else {
			log.Errorf("cannot get json store process: %v", err)
			return
		}
	}
	jsProc.Results = results
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

// OnOracleResults implements the indexer event callback.
// On this event the process status is set to Results.
func (pa *ProcessArchive) OnOracleResults(oracleResults *models.ProcessResult, pid []byte, height uint32) {
	jsProc, err := pa.storage.GetProcess(pid)
	if err != nil {
		if os.IsNotExist(err) { // if it does not exist yet, we create it
			proc, err := pa.indexer.ProcessInfo(pid)
			if err != nil {
				log.Errorf("cannot get process info %x from indexer: %v", pid, err)
				return
			}
			jsProc = &Process{ProcessInfo: proc, Results: nil}
		} else {
			log.Errorf("cannot get json store process: %v", err)
			return
		}
	}
	// Ensure the status is set to RESULTS since OnOracleResults event is called on setProcessResultsTx
	jsProc.ProcessInfo.Status = int32(models.ProcessStatus_RESULTS)
	jsProc.ProcessInfo.FinalResults = true
	// TODO: add signatures from oracles
	//jsProc.Results.Signatures = append(jsProc.results.Signatures, oracleResults.Signature)

	// Store the process
	if err := pa.storage.AddProcess(jsProc); err != nil {
		log.Errorf("cannot add json process: %v", err)
		return
	}
	log.Infof("stored json process %x for oracle results transaction event", pid)

	// Send publish signal
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
