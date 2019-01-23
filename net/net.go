package net

import (
	"encoding/json"
	"fmt"

	"github.com/vocdoni/dvote-relay/batch"
	"github.com/vocdoni/dvote-relay/data"
	"github.com/vocdoni/dvote-relay/types"
)

func Sub(topic string) error {
	subscription := data.PsSubscribe(topic)
	fmt.Println("Subscribed > " + topic)
	var msg data.Record
	var err error
	for {
		msg.Shell, err = subscription.Next()
		if err != nil {
			return err
		}

		payload := msg.Shell.Data

		var e types.Envelope
		var b types.Ballot

		err = json.Unmarshal(payload, &e)
		if err != nil {
			return err
		}

		err = json.Unmarshal(e.Ballot, &b)
		if err != nil {
			return err
		}

		err = batch.Add(b)
		if err != nil {
			return err
		}

		fmt.Println("Got > " + string(payload))
	}
}
