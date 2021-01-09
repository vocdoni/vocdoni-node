package events

import (
	"time"

	diskqueue "github.com/nsqio/go-diskqueue"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// eventCallback represents any callback function tied to an event source
type eventCallback func(
	app *vochain.BaseApplication,
	signer *ethereum.SignKeys,
	m proto.Message,
) error

// eventWorker is responsible for performing a unit of work given an event
type eventWorker struct {
	// storage is the disk queue
	storage diskqueue.Interface
	// signer
	signer *ethereum.SignKeys
	// vochainApp is a pointer to the Vochain BaseApplication allowing to call SendTx method
	vochainApp *vochain.BaseApplication
	// handlers maps each event to a callback using the event sourceType
	handlers map[SourceType]eventCallback
}

// start starts the event worker.
// The worker will constantly pull for new events
// and will handle them one by one
// Events can be handled sync or async and it is up
// to the callback to define the behaviour.
// i.e if we want OnProcessResults event to be processed
// async the implementation of the handler must be wrapped with
// an anonymous goroutine.
func (ew *eventWorker) start() {
	for {
		// Receive a event.
		event, err := ew.nextEvent()
		if err != nil {
			log.Warnf("cannot get next event: %s", err)
			continue
		}
		if event == nil {
			time.Sleep(time.Millisecond * types.EventWorkerSleepTime)
			continue
			// TODO: mvdan improve this
		}
		log.Debugf("processing event: %+v", event)
		// handle event
		callback, ok := ew.handlers[event.SourceType]
		if !ok {
			log.Warnf("callback not supported, skipping...")
			continue
		}
		if err := callback(ew.vochainApp, ew.signer, event.Data); err != nil {
			log.Warnf("event processing failed: %s", err)
			continue
		}
		log.Infof("event %+v processed successfully", event)
	}
}

// nextEvent gets an event from the events persistent storage
func (ew *eventWorker) nextEvent() (*Event, error) {
	event := &Event{}
	ev := <-ew.storage.ReadChan()
	switch SourceType(ev[0]) {
	case ScrutinizerOnprocessresult:
		evData := &models.ProcessResult{}
		if err := proto.Unmarshal(ev[1:], evData); err != nil {
			return nil, err
		}
		event.Data = evData
	}
	event.SourceType = SourceType(ev[0])
	return event, nil
}

// AddHandler adds a new event handler to the worker
func (ew *eventWorker) AddHandler(sourceType SourceType, callback eventCallback) {
	ew.handlers[sourceType] = callback
}
