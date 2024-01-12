package ist

import (
	"fmt"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
)

// removeAction removes an action from the pending actions list.
func (c *Controller) removeAction(id []byte) error {
	return c.state.NoState(true).Delete(addPrefix(id))
}

// addAction adds an action to the pending actions list.
func (c *Controller) addAction(act *Action) error {
	if act.ID == nil {
		return fmt.Errorf("action ID is nil")
	}
	return c.state.NoState(true).Set(addPrefix(act.ID), act.encode())
}

func (c *Controller) retrieveAction(id []byte) (*Action, error) {
	act := &Action{}
	value, err := c.state.NoState(true).Get(addPrefix(id))
	if err != nil {
		if err == db.ErrKeyNotFound {
			return nil, ErrActionNotFound
		}
		return nil, err
	}
	if err := act.decode(value); err != nil {
		return nil, err
	}
	return act, nil
}

// findPendingActions returns all pending actions that are ready to be executed.
func (c *Controller) findPendingActions(height, timestamp uint32) ([]*Action, error) {
	pendingActions := []*Action{}
	if err := c.state.NoState(true).Iterate([]byte(dbPrefix), func(key, value []byte) bool {
		act := &Action{}
		if err := act.decode(value); err != nil {
			log.Errorw(err, fmt.Sprintf("error decoding action with key %x", key))
			return true
		}
		if (act.Height != 0 && height >= act.Height) || (act.TimeStamp != 0) && timestamp >= act.TimeStamp {
			pendingActions = append(pendingActions, act)
		}
		return true
	}); err != nil {
		return nil, err
	}

	return pendingActions, nil
}

// addPrefix adds the database prefix to the given ID.
func addPrefix(id []byte) []byte {
	return append([]byte(dbPrefix), id...)
}
