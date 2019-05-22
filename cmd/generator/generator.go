package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/url"
	"os"
	"time"

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

func parseMsg(payload []byte) (map[string]interface{}, error) {
	var msgJSON interface{}
	err := json.Unmarshal(payload, &msgJSON)
	if err != nil {
		return nil, err
	}
	msgMap, ok := msgJSON.(map[string]interface{})
	if !ok {
		return nil, errors.New("Could not parse request JSON")
	}
	return msgMap, nil
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
	u := url.URL{Scheme: "ws", Host: *target, Path: "/dvote"}

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

	//dummyRequestPing := `{"id": "req-0000001", "request": {"method": "ping"}}`
	dummyRequestAddFile := `{"id": "req0000002", "request": {"method": "addFile", "name": "My first file", "type": "ipfs", "content": "SGVsbG8gVm9jZG9uaSE=", "timestamp": 1556110671}, "signature": "539"}`
	dummyRequestPinFileFmt := `{"id": "req0000003","request": {"method": "pinFile","uri": "%s","timestamp": 1556110671},"signature": "0x00"}`
	dummyRequestListPins := `{"id": "req0000004","request": {"method": "pinList","timestamp": 1556110671},"signature": "0x00"}`
	dummyRequestUnpinFileFmt := `{"id": "req0000005","request": {"method": "unpinFile","uri": "%s", "timestamp": 1556110671},"signature": "0x00"}`

	i := 0
	for {
		select {
		case <-timer.C:
			//var jsonStr = makeEnvelope(makeBallot())
			//log.Printf("Conn %d sending message %s", i%*connections, jsonStr)
			//log.Printf("%v", conns)
			conn := conns[i%*connections]
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second*5)); err != nil {
				fmt.Printf("Failed to receive pong: %v", err)
			}
			//conn.WriteMessage(websocket.TextMessage, []byte(jsonStr))  //test with raw votes
			//conn.WriteMessage(websocket.TextMessage, []byte(dummyRequestPing))
			log.Printf("Conn %d sending message %s", i%*connections, dummyRequestAddFile)
			conn.WriteMessage(websocket.TextMessage, []byte(dummyRequestAddFile))
			// request here
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Cannot read message")
				break
			}
			fmt.Println("Response message type: ", msgType)
			fmt.Println("Response message content: ", string(msg))
			msgMap, err := parseMsg(msg)
			if err != nil {
				log.Printf("couldn't parse message %v", string(msg))
				log.Printf("error was: %v", err)
				break
			}
			responseMap, ok := msgMap["response"].(map[string]interface{})
			if !ok {
				log.Printf("couldnt parse response field")
				break
			}
			uri, ok := responseMap["uri"].(string)
			if !ok {
				log.Printf("couldnt parse uri")
				break
			}

			log.Printf("Conn %d sending message %s", i%*connections, fmt.Sprintf(dummyRequestPinFileFmt, uri))
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(dummyRequestPinFileFmt, uri)))
			msgType, msg, err = conn.ReadMessage()
			if err != nil {
				log.Printf("Cannot read message")
				break
			}
			fmt.Println("Response message content: ", string(msg))

			log.Printf("Conn %d sending message %s", i%*connections, dummyRequestListPins)
			conn.WriteMessage(websocket.TextMessage, []byte(dummyRequestListPins))
			msgType, msg, err = conn.ReadMessage()
			if err != nil {
				log.Printf("Cannot read message")
				break
			}
			fmt.Println("Response message content: ", string(msg))

			log.Printf("Conn %d sending message %s", i%*connections, fmt.Sprintf(dummyRequestUnpinFileFmt, uri))
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(dummyRequestUnpinFileFmt, uri)))
			msgType, msg, err = conn.ReadMessage()
			if err != nil {
				log.Printf("Cannot read message")
				break
			}
			fmt.Println("Response message content: ", string(msg))

			i++
			log.Printf("Sent!")
		default:
			continue
		}
	}
}
