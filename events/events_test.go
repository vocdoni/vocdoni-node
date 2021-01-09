package events_test

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/nsqio/go-diskqueue"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/events"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// WARNING: This tests do not provide any vochain tx guarantee

// TestCollectorOnProcessResults tests the collector with the
// OnProcessResults event originated at the scrutinizer
func TestCollectorOnProcessResults(t *testing.T) {
	l := func(lvl diskqueue.LogLevel, f string, args ...interface{}) {}
	// create dispatcher
	signer := new(ethereum.SignKeys)
	if err := signer.Generate(); err != nil {
		t.Fatalf("cannot generate ethereum signer: %s", err)
	}
	dispatcherMock := &DispatcherMock{}
	dispatcherMock.Dispatcher = *events.NewDispatcher(
		signer,
		nil,
		t.TempDir(),
		5000000,
		1,
		5000000,
		100,
		10000,
		l,
	)
	// start dispatcher
	dispatcherMock.StartDispatcherMock()

	// create OnProcessResults event data
	eventData := &models.ProcessResult{
		Votes:     make([]*models.QuestionResult, 1),
		ProcessId: Hex2byte(nil, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		EntityId:  Hex2byte(nil, "2b7222146a805bba0dbb61869c4b3a03209dffba"),
	}
	eventData.Votes[0] = &models.QuestionResult{
		Question: [][]byte{{1}, {1}, {1}},
	}
	dispatcherMock.Collect(eventData, events.ScrutinizerOnprocessresult)
	time.Sleep(time.Millisecond * 500)
}

func Hex2byte(tb testing.TB, s string) []byte {
	b, err := hex.DecodeString(util.TrimHex(s))
	if err != nil {
		if tb == nil {
			panic(err)
		}
		tb.Fatal(err)
	}
	return b
}

// EventWorker is responsible for performing a unit of work given an event
type EventWorkerMock struct {
	Storage diskqueue.Interface
	// Signer
	Signer *ethereum.SignKeys
	// VochainApp is a pointer to the Vochain BaseApplication allowing to call SendTx method
	VochainApp *vochain.BaseApplication
}

// Start starts the event worker.
// The worker will constantly pull for new events
// and will handle them one by one
// Events can be handled sync or async and it is up
// to the callback to define the behaviour.
// i.e if we want OnProcessResults event to be processed
// async the implementation of the handler must be wrapped with
// an anonymous goroutine.
func (ew *EventWorkerMock) Start() {
	for {
		// Receive a event.
		event, err := ew.NextEvent()
		if err != nil {
			log.Warnf("cannot get next event: %s", err)
			continue
		}
		if event == nil {
			time.Sleep(time.Millisecond * 200)
			continue
			// TODO: mvdan improve this
		}
		log.Debugf("processing event: %+v", event)
		switch event.SourceType {
		case events.ScrutinizerOnprocessresult:
			if err := ew.onComputeResults(event.Data.(*models.ProcessResult)); err != nil {
				log.Warnf("event processing failed: %s", err)
				continue
			}
			fmt.Println("onProcessResults received")
		default:
			log.Warnf("cannot process event, invalid source type: %x", event.SourceType)
			continue
		}
		log.Infof("event %+v processed successfully", event)
	}
}

// OnComputeResults is called once a process result is computed by the scrutinizer.
func (ew *EventWorkerMock) onComputeResults(results *models.ProcessResult) error {
	return nil
}

// NextEvent gets an event from the events persistent storage
func (ew *EventWorkerMock) NextEvent() (*events.Event, error) {
	event := &events.Event{}
	ev := <-ew.Storage.ReadChan()
	switch events.SourceType(ev[0]) {
	case events.ScrutinizerOnprocessresult:
		evData := &models.ProcessResult{}
		if err := proto.Unmarshal(ev[1:], evData); err != nil {
			return nil, err
		}
		event.Data = evData
	}
	event.SourceType = events.SourceType(ev[0])
	return event, nil
}

type DispatcherMock struct {
	events.Dispatcher
}

func (d *DispatcherMock) StartDispatcherMock() {
	// create all of our event workers.
	for i := 0; i < len(types.EventSources); i++ {
		log.Debugf("Starting event worker", i+1)
		eventWorker := &EventWorkerMock{
			Storage: d.Storage[i],
			Signer:  d.Signer,
		}
		go eventWorker.Start()
	}
}
