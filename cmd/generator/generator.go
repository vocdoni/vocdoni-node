package main

import (
	"os"
	"time"
	"flag"
	"io"
	"log"
	"fmt"
	"math/rand"
	"encoding/json"
	"encoding/base64"
	"net/url"
	"github.com/gorilla/websocket"
//	"net/http"
//	"bytes"
//	"io/ioutil"
	"github.com/vocdoni/go-dvote/types"
//	"github.com/vocdoni/go-dvote/net"
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

	env.Type = "zk-snarks-envelope"

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
	var target = flag.String("target", "127.0.0.1:9090", "target IP and port")
	var connections = flag.Int("conn", 1, "number of separate connections")
	var interval = flag.Int("interval", 1000, "interval between requests in ms")

	flag.Usage = func() {
		io.WriteString(os.Stderr, `Websockets client generator
		Example usage: ./generator -target=172.17.0.1 -conn=10 -interval=100
		`)
		flag.PrintDefaults()
	}
	flag.Parse()

	timer := time.NewTicker(time.Millisecond * time.Duration(*interval))
	rand.Seed(time.Now().UnixNano())
	//topic := "vocdoni_pubsub_testing"
	//fmt.Println("PubSub Topic:>", topic)
	u := url.URL{Scheme: "ws", Host: *target, Path: "/vocdoni"}

	var conns []*websocket.Conn
	for i := 0; i < *connections; i++ {
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Printf("Failed to connect", i, err)
			break
		}
		conns = append(conns, c)
		defer func() {
			c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
			time.Sleep(time.Second)
			c.Close()
		}()
	}

	log.Printf("Finished initializing %d connections", len(conns))

	i := 0
	for {
		select {
		case <- timer.C:
			var jsonStr = makeEnvelope(makeBallot())
			log.Printf("Conn %d sending message %s", i % *connections, jsonStr)
			log.Printf("%v", conns)
			conn := conns[i % *connections]
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second*5)); err != nil {
				fmt.Printf("Failed to receive pong: %v", err)
			}
			conn.WriteMessage(websocket.TextMessage, []byte(jsonStr))
			i++
			log.Printf("Sent!")
		default:
			continue
		}
	}
}
