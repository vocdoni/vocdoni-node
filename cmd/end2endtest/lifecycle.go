package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

func init() {
	ops["lifecyclelection"] = operation{
		testFunc: func() VochainTest {
			return &E2ELifecycleElection{}
		},
		description: "Publishes two elections one to validate that the election cannot be interrupted and the other validate that the election can be interrupted to valid status",
		example:     os.Args[0] + " --operation=lifecyclelection --votes=1000",
	}
}

var _ VochainTest = (*E2ELifecycleElection)(nil)

type E2ELifecycleElection struct {
	elections []e2eElection
}

func (t *E2ELifecycleElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.elections = append(t.elections, e2eElection{api: api, config: c})
	t.elections = append(t.elections, e2eElection{api: api, config: c})

	electionDescriptions := []struct {
		d             *vapi.ElectionDescription
		interruptible bool
	}{
		{d: newTestElectionDescription(2), interruptible: false},
		{d: newTestElectionDescription(2), interruptible: true},
	}

	for i, ed := range electionDescriptions {
		ed.d.ElectionType.Autostart = true
		ed.d.ElectionType.Interruptible = ed.interruptible
		ed.d.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}

		if err := t.elections[i].setupElection(ed.d, t.elections[0].config.nvotes); err != nil {
			log.Errorf("failed to setup election %d: %v", i, err)
		}
	}
	logElection(t.elections[0].election)
	logElection(t.elections[1].election)
	return nil
}

func (*E2ELifecycleElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2ELifecycleElection) Run() error {
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()

		api := t.elections[0].api
		electionID := t.elections[0].election.ElectionID

		isExpectedError := func(err error, status string) bool {
			switch status {
			case "READY":
				return strings.Contains(err.Error(), "already in READY state")
			case "PROCESS_UNKNOWN":
				return strings.Contains(err.Error(), "process status PROCESS_UNKNOWN unknown")
			case "RESULTS":
				return strings.Contains(err.Error(), "not authorized")
			default:
				return strings.Contains(err.Error(), "is not interruptible")
			}
		}

		for _, status := range models.ProcessStatus_name {
			log.Infow("testing election status transition",
				"election", electionID.String(),
				"to", status,
			)
			if _, err := api.SetElectionStatus(electionID, status); err != nil {
				if !isExpectedError(err, status) {
					errChan <- fmt.Errorf("unexpected error for status %s: %v", status, err)
					return
				}
				log.Debugf("expected error message: %v", err.Error())
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		api := t.elections[1].api
		electionID := t.elections[1].election.ElectionID

		// Define the desired election status transition
		transitions := []struct {
			fromStatus string
			toStatus   string
			errMsg     string
		}{
			{"PAUSED", "PAUSED", "already in PAUSED state"},
			{"PAUSED", "READY", ""},
			{"READY", "RESULTS", "not authorized to set process status to RESULTS"},
			{"READY", "CANCELED", ""},
			{"CANCELED", "PAUSED", "cannot set process status from CANCELED to paused"},
			{"CANCELED", "ENDED", "only be ended from ready or paused status"},
			{"CANCELED", "READY", "cannot set process status from CANCELED to ready"},
			{"CANCELED", "CANCELED", "already in CANCELED state"},
		}

		// Test each status transition
		for _, t := range transitions {
			log.Infow("testing election status transition",
				"election", electionID.String(),
				"from", t.fromStatus,
				"to", t.toStatus,
				"expected error", t.errMsg != "",
			)
			hash, err := api.SetElectionStatus(electionID, t.toStatus)
			if err != nil {
				if t.errMsg != "" && !strings.Contains(err.Error(), t.errMsg) {
					errChan <- fmt.Errorf("unexpected error from election status %s -> %s: %v", t.fromStatus, t.toStatus, err)
					return
				}
				log.Debugf("expected error message: %v", err.Error())
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
			defer cancel()
			if _, err := api.WaitUntilTxIsMined(ctx, hash); err != nil {
				errChan <- fmt.Errorf("gave up waiting for tx %s to be mined: %s", hash, err.Error())
				return
			}
		}
	}()

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}
