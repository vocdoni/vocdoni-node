package main

import (
	"os"
	"strconv"
	"time"
	"fmt"
	"math/rand"
	"encoding/json"
	"encoding/base64"
//	"net/http"
//	"bytes"
//	"io/ioutil"
	"github.com/vocdoni/dvote-relay/types"
	"github.com/vocdoni/dvote-relay/data"
)

func makeBallot() string {
	var bal types.Ballot

	bal.Type = "ballot0"

	pidBytes := make([]byte, 32)
	rand.Read(pidBytes)
	bal.PID = base64.StdEncoding.EncodeToString(pidBytes)

	nullifier := make([]byte, 32)
	rand.Read(nullifier)
	bal.Nullifier = nullifier

	vote := make([]byte, 32)
	rand.Read(vote)
	bal.Vote = vote

	franchise := make([]byte, 32)
	rand.Read(franchise)
	bal.Franchise = franchise


	b, err := json.Marshal(bal)
	if err != nil {
		fmt.Println(err)
		return "error"
	}
	//todo: add encryption, pow
	return string(b)
}

func makeEnvelope(ballot string) string {
	var env types.Envelope

	env.Type = "envelope0"

	env.Nonce = rand.Uint64()

	kp := make([]byte, 4)
	rand.Read(kp)
	env.KeyProof = kp

	env.Ballot = []byte(ballot)

	env.Timestamp = time.Now()

	e, err := json.Marshal(env)
	if err != nil {
		fmt.Println(err)
		return "error"
	}
	//todo: add encryption, pow
	return string(e)

}

func main() {
	interval := os.Args[1]
	i, _ := strconv.Atoi(interval)
	timer := time.NewTicker(time.Millisecond * time.Duration(i))
	rand.Seed(time.Now().UnixNano())
	topic := "vocdoni_pubsub_testing"
	fmt.Println("PubSub Topic:>", topic)


	for {
		select {
		case <- timer.C:
			var jsonStr = makeEnvelope(makeBallot())
			fmt.Println(jsonStr)
			data.PsPublish(topic, jsonStr)
		default:
			continue
		}
	}
}
