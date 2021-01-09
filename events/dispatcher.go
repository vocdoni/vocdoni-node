package events

import (
	"fmt"
	"time"

	"github.com/nsqio/go-diskqueue"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// SourceType represents the origin of an event (e.g. scrutinizerOnProcessResult)
type SourceType byte

const (
	// ScrutinizerOnprocessresult represents the event created once
	// the scrutinizer computed successfully the results of a process
	ScrutinizerOnprocessresult = SourceType(0)
)

// eventList ties each source type to the event callbacks of the type
var eventList = [5]map[SourceType]eventCallback{
	// scrutinizer
	{
		ScrutinizerOnprocessresult: onComputeResults,
	},
	// keykeeper
	// ethereum
	// vochain
	// census
}

// Event represents all the info required for an event to be processed
type Event struct {
	Data       proto.Message
	SourceType SourceType
}

// Dispatcher responsible for pulling work requests
// and distributing them to the next available worker
type Dispatcher struct {
	Signer *ethereum.SignKeys
	// Storage stores the unprocessed events
	Storage [len(types.EventSources)]diskqueue.Interface
	// VochainApp is a pointer to the Vochain BaseApplication allowing to call SendTx method
	VochainApp *vochain.BaseApplication
}

// NewDispatcher creates a new dispatcher
func NewDispatcher(
	signer *ethereum.SignKeys,
	vochain *vochain.BaseApplication,
	dbPath string,
	maxBytesPerFile int64,
	minMsgSize,
	maxMsgSize int32,
	syncEvery int64,
	syncTimeout time.Duration,
	logger diskqueue.AppLogFunc,
) *Dispatcher {
	d := &Dispatcher{Signer: signer, VochainApp: vochain}
	for idx := range d.Storage {
		d.Storage[idx] = diskqueue.New(
			types.EventSources[idx],
			dbPath,
			maxBytesPerFile,
			minMsgSize,
			maxMsgSize,
			syncEvery,
			syncTimeout,
			logger,
		)
	}
	return d
}

// StartDispatcher creates and starts the workers
func (d *Dispatcher) StartDispatcher() {
	// create all of our event workers.
	for i := 0; i < len(types.EventSources); i++ {
		log.Debugf("starting event worker", i+1)
		eventWorker := &eventWorker{
			storage:    d.Storage[i],
			signer:     d.Signer,
			vochainApp: d.VochainApp,
			handlers:   eventList[i],
		}
		go eventWorker.start()
	}
	log.Infof("events dispatcher started with %d workers", len(types.EventSources))
}

// Collect receives, check and push the events to the event queue
func (d *Dispatcher) Collect(eventData proto.Message, sourceType SourceType) {
	// Now, we take the delay, and the person's name, and make a WorkRequest out of them.
	if sourceType < SourceType(0) || sourceType > SourceType(types.MaxUint8) {
		log.Debugf("invalid source type: %d", sourceType)
	}
	// receive the event
	event, err := d.wrap(eventData, sourceType)
	if err != nil {
		log.Debugf("cannot receive event: %w", err)
		return
	}

	// add event to the database
	if err := d.add(event); err != nil {
		log.Debugf("cannot add event into database: %w", err)
		return
	}

	log.Infof("event %+v added for processing", event)
}

// wrap checks and wraps an event into an Event struct
func (d *Dispatcher) wrap(eventData proto.Message, sourceType SourceType) (*Event, error) {
	event := &Event{}
	switch sourceType {
	case ScrutinizerOnprocessresult:
		// check event data
		if err := checkResults(eventData.(*models.ProcessResult)); err != nil {
			return nil, fmt.Errorf("invalid event received: %w", err)
		}
	default:
		return nil, fmt.Errorf("cannot receive event, invalid event origin")
	}
	event.Data = eventData
	event.SourceType = sourceType
	return event, nil
}

// DB

// add adds an event to the events persistent storage.
// An event will be stored as: []byte where byte[0] encodes the event type.
// The other event bytes encode the event data itself
func (d *Dispatcher) add(event *Event) error {
	var eventDataBytes []byte
	var err error
	switch event.SourceType {
	case ScrutinizerOnprocessresult:
		switch t := event.Data.(type) {
		case *models.ProcessResult:
			eventDataBytes, err = proto.Marshal(t)
			if err != nil {
				return fmt.Errorf("cannot marshal event %+v data: %w", event, err)
			}
		default:
			return fmt.Errorf("cannot add scrutinizer event to persisten storage, invalid event type")
		}
	default:
		return fmt.Errorf(
			"cannot add event %+v to persistent storage, unsupported event origin: %d",
			event,
			event.SourceType,
		)
	}
	queueIdx := 0
	switch est := event.SourceType; {
	case est < types.ScrutinizerSourceTypeLimit:
		// nothing to do, 0 is fine
	case est >= types.ScrutinizerSourceTypeLimit && est < types.KeyKeeperSourceTypeLimit:
		queueIdx = 1
	case est >= types.KeyKeeperSourceTypeLimit && est < types.EthereumSourceTypeLimit:
		queueIdx = 2
	case est >= types.EthereumSourceTypeLimit && est < types.VochainSourceTypeLimit:
		queueIdx = 3
	case est >= types.VochainSourceTypeLimit && est < types.CensusSourceTypeLimit:
		queueIdx = 4
	default:
		return fmt.Errorf("cannot add event, invalid queue index")
	}
	// byte[0]  -> event type
	// byte[1:] -> event data
	err = d.Storage[queueIdx].Put(
		append(
			[]byte(fmt.Sprintf("%x", event.SourceType)),
			eventDataBytes...,
		),
	)
	if err != nil {
		return fmt.Errorf("cannot add event %+v to persistent storage: %w", event, err)
	}
	return nil
}
