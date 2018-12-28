package main

import (
	"os"
	"strconv"
	"time"
	"fmt"
	"math/rand"
	"encoding/json"
	"encoding/base64"
	"net/http"
	"bytes"
	"io/ioutil"
	"github.com/vocdoni/dvote-relay/types"
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
	url := "http://localhost:8090/submit"
	fmt.Println("URL:>", url)

	for {
		select {
		case <- timer.C:
			fmt.Println(makeEnvelope(makeBallot()))
			var jsonStr = []byte(makeEnvelope(makeBallot()))
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
			req.Header.Set("X-Custom-Header", "myvalue")
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()
			fmt.Println("response Status:", resp.Status)
			fmt.Println("response Headers:", resp.Header)
			body, _ := ioutil.ReadAll(resp.Body)
			fmt.Println("response Body:", string(body))
		default:
			continue
		}
	}
}
