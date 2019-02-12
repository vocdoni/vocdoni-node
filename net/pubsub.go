package net

import (
	"os"
	"fmt"
	"encoding/json"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/vocdoni/go-dvote/batch"
	"github.com/vocdoni/go-dvote/types"
)

type PubSubHandle struct {
	topic string
	subscription *shell.PubSubSubscription
}

func PsSubscribe(topic string) *shell.PubSubSubscription {
	sh := shell.NewShell("localhost:5001")
	sub, err := sh.PubSubSubscribe(topic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
	return sub
}

func PsPublish(topic, data string) error {
	sh := shell.NewShell("localhost:5001")
	err := sh.PubSubPublish(topic, data)
	if err != nil {
		return err
	}
	return nil
}

func (p *PubSubHandle) Init(topic string) error {
	p.topic = topic
	p.subscription = PsSubscribe(p.topic)
	return nil
}

func (p *PubSubHandle) Listen() error {
	var msg *shell.Message
	var err error
	for {
		msg, err = p.subscription.Next()
		if err != nil {
			fmt.Fprintf(os.Stderr, "recieve error: %s", err)
			return err
		}

		payload := msg.Data

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

func (p *PubSubHandle) Send(data string) error {
	return PsPublish(p.topic, data)
}
