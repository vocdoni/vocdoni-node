// Package ist provides the Internal State Transition (IST) mechanism for Vochain.
// The IST is responsible for modifying the Vochain's state based on actions scheduled
// to be executed at specific heights. This ensures accurate, tamper-proof results
// through consensus among nodes.
//
// The Internal State Transition Controller (ISTC) is the main component of this package.
// It schedules and executes IST actions at specified heights. Actions include computing
// results, committing results, and ending processes.
//
// The package contains the following types:
//
// - Controller: the IST controller.
// - Actions: a model to store the list of IST actions for a specific height.
// - Action: a model to store the IST actions.
//
// IST actions are encoded and decoded using the Gob encoding package.
// The encoding and decoding functions are provided by the Actions type.
//
// The Controller struct provides methods for scheduling, rescheduling, and committing
// IST actions. When committing an action, it performs the necessary operations depending
// on the type of action, such as computing results, committing results, or ending processes.
// It also provides methods to store and retrieve data from the no-state, which is a separate
// key-value store that holds non-consensus-critical data.
//
// Constants used in the package are:
//
// - BlocksToWaitForResultsFactor: a factor used to compute the height at which the results will be committed to the state.
// - ExtraWaitSecondsForResults: the number of extra seconds to wait for results.
//
// Technical decisions taken in the code include:
//
// - Implementing a separate no-state key-value store for non-consensus-critical data storage.
// - Computing results in a separate goroutine to avoid blocking state transitions.
// - Including a mechanism for rescheduling actions in case of missing keys or other issues.
package ist

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	dbPrefix = "ist/"
)

// Controller is the internal state transition controller.
type Controller struct {
	state *state.State
}

// NewISTC creates a new ISTC.
// The ISTC is the controller of the internal state transitions.
// It schedule actions to be executed at a specific height that will modify the state.
func NewISTC(s *state.State) *Controller {
	return &Controller{
		state: s,
	}
}

// ActionID is the type used to identify the IST actions.
type ActionID int32

const (
	// ActionComputeResults computes and schedules the commit of results, for the given election and height.
	ActionComputeResults ActionID = iota
	// ActionCommitResults schedules the commit of the results to the state.
	ActionCommitResults
	// ActionEndProcess sets a process as ended. It schedules ActionComputeResults.
	ActionEndProcess
)

// ActionsToString translates the action identifers to its corresponding human friendly string.
var ActionsToString = map[ActionID]string{
	ActionComputeResults: "compute-results",
	ActionCommitResults:  "commit-results",
	ActionEndProcess:     "end-process",
}

// Actions is the model used to store the list of IST actions for
// a specific height into state.
type Actions map[string]Action

// Action is the model used to store the IST actions into state.
type Action struct {
	ID         ActionID
	ElectionID []byte
	Attempts   uint32
}

// encode performs the encoding of the IST action using Gob.
func (m *Actions) encode() []byte {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(m); err != nil {
		// should never happen
		panic(err)
	}
	return buf.Bytes()
}

// decode performs the decoding of the IST action using Gob.
func (m *Actions) decode(data []byte) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(m)
}

// Schedule schedules an IST action to be executed at the given height.
func (c *Controller) Schedule(height uint32, id []byte, action Action) error {
	if id == nil {
		return fmt.Errorf("cannot schedule IST action: nil id")
	}
	actions, err := c.Actions(height)
	if err != nil {
		return err
	}
	// set the IST action
	actions[string(id)] = action

	// store the IST actions
	log.Debugw("schedule IST action", "height", height, "id", fmt.Sprintf("%x", id), "action", ActionsToString[action.ID])
	return c.storeToNoState(dbIndex(height), actions.encode())
}

// Actions returns the IST actions scheduled for the given height.
// If no actions are scheduled, it returns an empty IstActions.
func (c *Controller) Actions(height uint32) (Actions, error) {
	// get the IST actions
	actions := Actions{}
	actionsBytes, err := c.retrieveFromNoState(dbIndex(height))
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			// no IST actions scheduled for this height, we return empty
			actions = make(map[string]Action)
			return actions, nil
		}
		return nil, fmt.Errorf("cannot get IST actions on height %d: %w", height, err)
	}
	// decode the IST actions
	if err := actions.decode(actionsBytes); err != nil {
		return nil, fmt.Errorf("could not decode actions: %w", err)
	}
	return actions, nil
}

// Reschedule reschedules an IST action to the given new height.
func (c *Controller) Reschedule(id []byte, oldHeight, newHeight uint32) error {
	if len(id) == 0 {
		return fmt.Errorf("cannot reschedule IST action: nil id")
	}
	if oldHeight >= newHeight {
		return fmt.Errorf("cannot reschedule IST action: old height (%d) >= new height (%d)", oldHeight, newHeight)
	}
	actions, err := c.Actions(oldHeight)
	if err != nil {
		return err
	}
	// find the IST action
	_, ok := actions[string(id)]
	if !ok {
		return fmt.Errorf("cannot reschedule IST action: action not found")
	}
	// store the IST action
	a := actions[string(id)]
	a.Attempts++
	if err := c.Schedule(newHeight, id, a); err != nil {
		return err
	}
	// delete the IST action
	delete(actions, string(id))

	// update the IST actions for the old height
	if err := c.storeToNoState(dbIndex(oldHeight), actions.encode()); err != nil {
		return fmt.Errorf("cannot reschedule IST action: %w", err)
	}
	return nil
}

// Commit executes the IST actions scheduled for the given height.
func (c *Controller) Commit(height uint32, isSynchronizing bool) error {
	actions, err := c.Actions(height)
	if err != nil {
		return fmt.Errorf("cannot commit IST actions: %w", err)
	}
	if len(actions) == 0 {
		return nil
	}
	for id, action := range actions {
		switch action.ID {
		case ActionComputeResults:
			log.Debugw("compute results", "height", height, "id", fmt.Sprintf("%x", id), "attempt", action.Attempts)
			// check if we have all the revealed keys, else reschedule
			if !c.checkRevealedKeys(action.ElectionID) {
				// we are missing some keys, we cannot compute the results yet
				newHeight := height + (1 + action.Attempts*2)
				log.Infow("missing keys, rescheduling IST action", "newHeight", newHeight, "id",
					fmt.Sprintf("%x", id), "action", ActionsToString[action.ID], "attempt", action.Attempts)
				if err := c.Reschedule([]byte(id), height, newHeight); err != nil {
					return fmt.Errorf("cannot reschedule IST action: %w", err)
				}
				continue
			}
			if !isSynchronizing {
				// compute results in a goroutine to avoid blocking the state transition
				go func() {
					if err := c.computeAndStoreResults(action.ElectionID); err != nil {
						log.Errorw(err, fmt.Sprintf("cannot compute results for election %x", action.ElectionID))
					}
				}()
			}
			// schedule the commit results action
			if err := c.scheduleCommitResults(action.ElectionID); err != nil {
				return fmt.Errorf("cannot schedule commit results for election %x: %w",
					action.ElectionID, err)
			}
		case ActionCommitResults:
			log.Debugw("commit results", "height", height, "id", fmt.Sprintf("%x", id), "action", ActionsToString[action.ID])
			var r *results.Results
			// if we are synchronizing, we compute the results here, otherwise we get them from the store
			if isSynchronizing {
				if !c.checkRevealedKeys(action.ElectionID) {
					// we are missing some keys, we cannot compute the results yet
					newHeight := height + (1 + action.Attempts*2)
					log.Infow("missing keys, rescheduling IST action", "newHeight", newHeight, "id",
						fmt.Sprintf("%x", id), "action", ActionsToString[action.ID], "attempt", action.Attempts)
					if err := c.Reschedule([]byte(id), height, newHeight); err != nil {
						return fmt.Errorf("cannot reschedule IST action: %w", err)
					}
					continue
				}
				r, err = results.ComputeResults(action.ElectionID, c.state)
				if err != nil {
					return fmt.Errorf("cannot compute results on commit: %w", err)
				}
			}
			if err := c.commitResults(action.ElectionID, r); err != nil {
				return fmt.Errorf("cannot commit results for election %x: %w",
					action.ElectionID, err)
			}
		case ActionEndProcess:
			log.Debugw("end process", "height", height, "id", fmt.Sprintf("%x", id), "action", ActionsToString[action.ID])
			if err := c.endElection(action.ElectionID); err != nil {
				return fmt.Errorf("cannot end election %x: %w",
					action.ElectionID, err)
			}
		default:
			return fmt.Errorf("unknown IST action %d", action.ID)
		}
	}
	// delete the IST actions for the given height
	if err := c.deleteFromNoState(dbIndex(height)); err != nil {
		log.Warnf("cannot delete IST actions: %v", err)
	}
	return nil
}

func (c *Controller) checkRevealedKeys(electionID []byte) bool {
	process, err := c.state.Process(electionID, false)
	if err != nil {
		log.Warnw("cannot get process", "electionID", hex.EncodeToString(electionID), "err", err.Error())
		return false
	}
	if process.KeyIndex == nil {
		return true
	}
	return *process.KeyIndex == 0
}

func (c *Controller) endElection(electionID []byte) error {
	process, err := c.state.Process(electionID, false)
	if err != nil {
		return fmt.Errorf("cannot get process: %w", err)
	}
	// if the process is canceled, ended or in results, we don't need to do
	// anything else, smooth return.
	switch process.Status {
	case models.ProcessStatus_CANCELED,
		models.ProcessStatus_ENDED,
		models.ProcessStatus_RESULTS:
		return nil
	}

	// set the election to ended
	if err := c.state.SetProcessStatus(electionID, models.ProcessStatus_ENDED, true); err != nil {
		return fmt.Errorf("cannot end election: %w", err)
	}
	// schedule the IST action to compute the results
	return c.Schedule(c.state.CurrentHeight()+1, electionID, Action{
		ID:         ActionComputeResults,
		ElectionID: electionID,
	})
}
