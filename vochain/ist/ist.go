package ist

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"

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
	st              *state.State
	isSynchronizing uint32
}

// NewISTC creates a new ISTC.
// The ISTC is the controller of the internal state transitions.
// It schedule actions to be executed at a specific height that will modify the state.
func NewISTC(s *state.State) *Controller {
	return &Controller{
		st: s,
	}
}

const (
	// ActionComputeResults computes and schedules the commit of results, for the given election and height.
	ActionComputeResults = iota
	// ActionCommitResults schedules the commit of the results to the state.
	ActionCommitResults
	// ActionEndProcess sets a process as ended. It schedules ActionComputeResults.
	ActionEndProcess
)

// ActionsToString translates the action identifers to its corresponding human friendly string.
var ActionsToString = map[int]string{
	ActionComputeResults: "compute-results",
	ActionCommitResults:  "commit-results",
	ActionEndProcess:     "end-process",
}

// Actions is the model used to store the list of IST actions for
// a specific height into state.
type Actions struct {
	Actions map[string]Action
}

// Action is the model used to store the IST actions into state.
type Action struct {
	Action     int
	ElectionID []byte
}

// encode performs the encoding of the IST action using Gob.
func (m *Actions) encode() []byte {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(m); err != nil {
		// should never happend
		panic(err)
	}
	return buf.Bytes()
}

// decode performs the decoding of the IST action using Gob.
func (m *Actions) decode(data []byte) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(m)
}

// SetSyncing sets the syncing flag.
func (c *Controller) SetSyncing(isSynchronizing bool) {
	if isSynchronizing {
		atomic.StoreUint32(&c.isSynchronizing, 1)
	} else {
		atomic.StoreUint32(&c.isSynchronizing, 0)
	}
}

// IsSyncing returns true if the syncing flag is set.
func (c *Controller) IsSyncing() bool {
	return atomic.LoadUint32(&c.isSynchronizing) == 1
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
	actions.Actions[string(id)] = action

	// store the IST actions
	log.Debugw("schedule IST action", "height", height, "id", fmt.Sprintf("%x", id), "action", ActionsToString[action.Action])
	c.st.Tx.Lock()
	defer c.st.Tx.Unlock()
	return c.st.Tx.NoState().Set(dbIndex(height), actions.encode())
}

// Actions returns the IST actions scheduled for the given height.
// If no actions are scheduled, it returns an empty IstActions.
func (c *Controller) Actions(height uint32) (*Actions, error) {
	// get the IST actions
	actions := Actions{}
	c.st.Tx.RLock()
	actionsBytes, err := c.st.Tx.NoState().Get(dbIndex(height))
	c.st.Tx.RUnlock()
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			// no IST actions scheduled for this height, we return empty
			actions.Actions = make(map[string]Action)
			return &actions, nil
		}
		return nil, fmt.Errorf("cannot get IST actions on height %d: %w", height, err)
	}
	// decode the IST actions
	if err := actions.decode(actionsBytes); err != nil {
		return nil, fmt.Errorf("could not decode actions: %w", err)
	}
	return &actions, nil
}

// dbIndex returns the IST action index in the state database.
func dbIndex(height uint32) []byte {
	hBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(hBytes, height)
	return append([]byte(dbPrefix), hBytes...)
}

// Reschedule reschedules an IST action to the given new height.
func (c *Controller) Reschedule(id []byte, oldHeight, newHeight uint32) error {
	if id == nil {
		return fmt.Errorf("cannot reschedule IST action: nil id")
	}
	if oldHeight <= newHeight {
		return fmt.Errorf("cannot reschedule IST action: old height (%d) <= new height (%d)", oldHeight, newHeight)
	}
	actions, err := c.Actions(oldHeight)
	if err != nil {
		return err
	}
	// find the IST action
	var actID string
	for actID = range actions.Actions {
		if string(id) == actID {
			break
		}
	}
	if actID == "" {
		return fmt.Errorf("cannot reschedule IST action: action not found")
	}
	// store the IST action
	if err := c.Schedule(newHeight, id, actions.Actions[actID]); err != nil {
		return err
	}
	// delete the IST action
	delete(actions.Actions, actID)

	// update the IST actions
	c.st.Tx.Lock()
	defer c.st.Tx.Unlock()
	if err := c.st.Tx.NoState().Set(dbIndex(oldHeight), actions.encode()); err != nil {
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
	for id, action := range actions.Actions {
		switch action.Action {
		case ActionComputeResults:
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
			var r *results.Results
			// if we are synchronizing, we compute the results here, otherwise we get them from the store
			if isSynchronizing {
				r, err = results.ComputeResults(action.ElectionID, c.st)
				if err != nil {
					return fmt.Errorf("cannot compute results on commit: %w", err)
				}
			}
			if err := c.commitResults(action.ElectionID, r); err != nil {
				return fmt.Errorf("cannot commit results for election %x: %w",
					action.ElectionID, err)
			}
		case ActionEndProcess:
			if err := c.endElection(action.ElectionID); err != nil {
				return fmt.Errorf("cannot end election %x: %w",
					action.ElectionID, err)
			}
		default:
			return fmt.Errorf("unknown IST action %d", action.Action)
		}
		log.Debugw("executed IST action", "height", height, "id",
			hex.EncodeToString([]byte(id)), "action", ActionsToString[action.Action])
	}
	// delete the IST actions for the given height
	c.st.Tx.Lock()
	defer c.st.Tx.Unlock()
	if err := c.st.Tx.NoState().Delete(dbIndex(height)); err != nil {
		return fmt.Errorf("cannot delete IST actions: %w", err)
	}
	return nil
}

func (c *Controller) endElection(electionID []byte) error {
	process, err := c.st.Process(electionID, false)
	if err != nil {
		return fmt.Errorf("cannot get process: %w", err)
	}
	// if the process is canceled, ended or in results, we don't need to do
	// anything else, smooth return.
	if process.Status == models.ProcessStatus_CANCELED ||
		process.Status == models.ProcessStatus_RESULTS ||
		process.Status == models.ProcessStatus_ENDED {
		return nil
	}
	// set the election to ended
	if err := c.st.SetProcessStatus(electionID, models.ProcessStatus_ENDED, true); err != nil {
		return fmt.Errorf("cannot end election: %w", err)
	}
	// schedule the IST action to compute the results
	return c.Schedule(c.st.CurrentHeight()+1, electionID, Action{
		Action:     ActionComputeResults,
		ElectionID: electionID,
	})
}
