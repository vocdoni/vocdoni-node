package ist

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
// - Action: a model to store the IST action.
// - ActionTypeID: the type used to identify the IST actions.
//
// IST actions are encoded and decoded using the Gob encoding package.
// The encoding and decoding functions are provided by the Actions type.
//
// The Controller struct provides methods for scheduling, rescheduling, and committing
// IST actions. When committing an action, it performs the necessary operations depending
// on the type of action, such as computing results, committing results or ending processes.
// As temporary storage, IST uses the nostate space of the Vochain state. Which is a separate
// key-value store that holds non-consensus-critical data.

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
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

var (
	// ErrActionAlreadyExists is returned when trying to create an action that already exists.
	ErrActionAlreadyExists = fmt.Errorf("action already exists")
	// ErrActionNotFound is returned when trying to retrieve an action that does not exist.
	ErrActionNotFound = fmt.Errorf("action not found")
)

// NewISTC creates a new ISTC.
// The ISTC is the controller of the internal state transitions.
// It schedule actions to be executed at a specific height that will modify the state.
func NewISTC(s *state.State) *Controller {
	return &Controller{
		state: s,
	}
}

// ActionID is the type used to identify the IST actions.
type ActionTypeID int32

const (
	// ActionCommitResults schedules the commit of the results to the state.
	ActionCommitResults ActionTypeID = iota
	// ActionEndProcess sets a process as ended. It schedules ActionComputeResults.
	ActionEndProcess
	// ActionUpdateValidatorScore updates the validator score (votes and proposer) in the state.
	ActionUpdateValidatorScore
)

// ActionsToString translates the action identifiers to its corresponding human friendly string.
var ActionsToString = map[ActionTypeID]string{
	ActionCommitResults:        "commit-results",
	ActionEndProcess:           "end-process",
	ActionUpdateValidatorScore: "update-validator-score",
}

// Action is the model used to store the IST actions into state.
type Action struct {
	ID        []byte
	TypeID    ActionTypeID
	Height    uint32
	TimeStamp uint32

	ElectionID        []byte
	Attempts          uint32
	ValidatorVotes    [][]byte
	ValidatorProposer []byte
}

// encode performs the encoding of the IST action using Gob.
func (m *Action) encode() []byte {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(m); err != nil {
		// should never happen
		panic(err)
	}
	return buf.Bytes()
}

// decode performs the decoding of the IST action using Gob.
func (m *Action) decode(data []byte) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(m)
}

// Schedule schedules an IST action to be executed at the given height.
func (c *Controller) Schedule(action Action) error {
	if action.ID == nil {
		return fmt.Errorf("cannot schedule IST action: nil id")
	}
	_, err := c.retrieveAction(action.ID)
	if err != ErrActionNotFound {
		return ErrActionAlreadyExists
	}
	// store the IST action
	return c.addAction(&action)
}

// Reschedule reschedules an IST action to the given new height or timestamp.
// If isHeight is true, the new value is treated as a block height. Else it is treated as a timestamp.
// It also increases the attempts of the action if attemptsDelta is greater than 0.
func (c *Controller) Reschedule(id []byte, new uint32, isHeight bool, attemptsDelta uint32) error {
	if len(id) == 0 {
		return fmt.Errorf("cannot reschedule IST action: nil id")
	}
	action, err := c.retrieveAction(id)
	if err != nil {
		return err
	}
	// increase attempts
	action.Attempts += attemptsDelta
	// set the new height or timestamp
	var old uint32
	if isHeight {
		if new <= c.state.CurrentHeight() {
			return fmt.Errorf("new height (%d) <= current height (%d)", new, c.state.CurrentHeight())
		}
		old = action.Height
		action.Height = new
		action.TimeStamp = 0
	} else {
		ts, err := c.state.Timestamp(false)
		if err != nil {
			return fmt.Errorf("cannot get current timestamp: %w", err)
		}
		if new <= ts {
			return fmt.Errorf("new timestamp (%d) <= current timestamp (%d)", new, ts)
		}
		old = action.TimeStamp
		action.TimeStamp = new
		action.Height = 0
	}

	log.Debugw("reschedule action", "id", fmt.Sprintf("%x", action.ID), "old", old, "new", new, "attempts", action.Attempts)
	// store the IST action
	return c.addAction(action)
}

// Commit executes the IST actions scheduled for the given height.
func (c *Controller) Commit(height, timestamp uint32) error {
	// get the IST actions for the given height
	actions, err := c.findPendingActions(height, timestamp)
	if err != nil {
		return fmt.Errorf("cannot get pending actions: %w", err)
	}
	// if there are no actions, return
	if len(actions) == 0 {
		return nil
	}

	for _, action := range actions {
		switch action.TypeID {
		case ActionCommitResults:
			log.Debugw("commit results", "height", height, "id", fmt.Sprintf("%x", action.ID), "action", ActionsToString[action.TypeID])
			// if the election is set up with tempSIKs as true, purge the election
			// related SIKs
			process, err := c.state.Process(action.ElectionID, false)
			if err != nil {
				return fmt.Errorf("cannot get process: %w", err)
			}
			if process.GetTempSIKs() {
				log.Infow("purge temporal siks", "pid", hex.EncodeToString(process.ProcessId))
				if err := c.state.PurgeSIKsByElection(process.ProcessId); err != nil {
					log.Errorw(err, "cannot purge temp SIKs after election ends")
				}
			}
			var r *results.Results
			if !c.checkRevealedKeys(action.ElectionID) {
				// we are missing some keys, we cannot compute the results yet
				newHeight := height + (1 + action.Attempts*2)
				log.Infow("missing encryption keys, rescheduling IST action", "newHeight", newHeight, "id",
					fmt.Sprintf("%x", action.ID), "action", ActionsToString[action.TypeID], "attempt", action.Attempts)
				if err := c.Reschedule(action.ID, newHeight, true, 1); err != nil {
					return fmt.Errorf("cannot reschedule IST action: %w", err)
				}
				continue
			}
			r, err = results.ComputeResults(action.ElectionID, c.state)
			if err != nil {
				return fmt.Errorf("cannot compute results on commit: %w", err)
			}
			if err := c.commitResults(action.ElectionID, r); err != nil {
				return fmt.Errorf("cannot commit results for election %x: %w",
					action.ElectionID, err)
			}
			// delete the IST action
			if err := c.removeAction(action.ID); err != nil {
				return fmt.Errorf("cannot delete IST actions: %w", err)
			}
		case ActionEndProcess:
			log.Debugw("end process", "height", height, "id", fmt.Sprintf("%x", action.ID), "action", ActionsToString[action.TypeID])
			if err := c.endElection(action.ElectionID); err != nil {
				return fmt.Errorf("cannot end election %x: %w",
					action.ElectionID, err)
			}
			// delete the IST action
			if err := c.removeAction(action.ID); err != nil {
				return fmt.Errorf("cannot delete IST actions: %w", err)
			}
		case ActionUpdateValidatorScore:
			if err := c.updateValidatorScore(action.ValidatorVotes, action.ValidatorProposer); err != nil {
				return fmt.Errorf("cannot update validator score: %w", err)
			}
			// delete the IST action
			if err := c.removeAction(action.ID); err != nil {
				return fmt.Errorf("cannot delete IST actions: %w", err)
			}
		default:
			return fmt.Errorf("unknown IST action %d", action.ID)
		}
	}
	return nil
}

// Remove removes an scheduled IST action.
func (c *Controller) Remove(actionID []byte) error {
	return c.removeAction(actionID)
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
		return fmt.Errorf("endElection: cannot get election: %w", err)
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
		return fmt.Errorf("endElection: cannot set election to ENDED status: %w", err)
	}
	// schedule the IST action to compute the results
	return c.Schedule(Action{
		ID:         util.RandomBytes(32),
		TypeID:     ActionCommitResults,
		ElectionID: electionID,
		Height:     c.state.CurrentHeight() + 1,
	})
}
