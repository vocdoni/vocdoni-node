package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"time"

	"github.com/spf13/viper"
	flag "github.com/spf13/pflag"
	"github.com/gorilla/websocket"

	//	"net/http"
	//	"bytes"
	//	"io/ioutil"
	"github.com/vocdoni/go-dvote/types"
	"github.com/vocdoni/go-dvote/log"
	"github.com/vocdoni/go-dvote/config"
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
		log.Error(err)
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
		log.Error(err)
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

func newConfig() (config.GenCfg, error) {
	//setup flags
	flag.String("loglevel", "info", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")
	flag.String("target", "127.0.0.1:9090", "target IP and port")
	flag.Int("conn", 1, "number of separate connections")
	flag.Int("interval", 1000, "interval between requests in ms")

	flag.Usage = func() {
		io.WriteString(os.Stderr, `Websockets client generator
		Example usage: ./generator -target=172.17.0.1 -conn=10 -interval=100
		`)
		flag.PrintDefaults()
	}

	flag.Parse()
	viper := viper.New()
	var globalCfg config.GenCfg
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/go-dvote/cmd/generator/") // path to look for the config file in
	viper.AddConfigPath(".")                      // optionally look for config in the working directory
	err := viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	viper.BindPFlags(flag.CommandLine)
	
	err = viper.Unmarshal(&globalCfg)
	return globalCfg, err
}

func main() {
	//setup config
	globalCfg, err := newConfig()
	//setup logger
	log.InitLoggerAtLevel(globalCfg.LogLevel)
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	timer := time.NewTicker(time.Millisecond * time.Duration(globalCfg.Interval))
	rand.Seed(time.Now().UnixNano())
	//topic := "vocdoni_pubsub_testing"
	//fmt.Println("PubSub Topic:>", topic)
	u := url.URL{Scheme: "ws", Host: globalCfg.Target, Path: "/dvote"}

	var conns []*websocket.Conn
	for i := 0; i < globalCfg.Connections; i++ {
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Errorf("Failed to connect", i, err)
			break
		}
		conns = append(conns, c)
		defer func() {
			c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
			time.Sleep(time.Second)
			c.Close()
		}()
	}

	log.Infof("Finished initializing %d connections", len(conns))

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
			conn := conns[i%globalCfg.Connections]
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second*5)); err != nil {
				log.Errorf("Failed to receive pong: %v", err)
			}
			//conn.WriteMessage(websocket.TextMessage, []byte(jsonStr))  //test with raw votes
			//conn.WriteMessage(websocket.TextMessage, []byte(dummyRequestPing))
			log.Infof("Conn %d sending message %s", i%globalCfg.Connections, dummyRequestAddFile)
			conn.WriteMessage(websocket.TextMessage, []byte(dummyRequestAddFile))
			// request here
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				log.Errorf("Cannot read message")
				break
			}
			log.Infof("Message info: Response message type: %v, Response message content: %v", msgType, string(msg))
			msgMap, err := parseMsg(msg)
			if err != nil {
				log.Errorf("Couldn't parse message: %v, error: %v", err)
				break
			}
			responseMap, ok := msgMap["response"].(map[string]interface{})
			if !ok {
				log.Errorf("couldnt parse response field")
				break
			}
			uri, ok := responseMap["uri"].(string)
			if !ok {
				log.Errorf("couldnt parse uri")
				break
			}

			log.Infof("Sending message: connection: %v, message: %v, uri: %v", i%globalCfg.Connections, dummyRequestPinFileFmt, uri)
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(dummyRequestPinFileFmt, uri)))
			msgType, msg, err = conn.ReadMessage()
			if err != nil {
				log.Errorf("Cannot read message")
				break
			}
			log.Infof("Response message content: ", string(msg))

			log.Infof("Sending message: connection: %v, message: %v", i%globalCfg.Connections, dummyRequestListPins)
			conn.WriteMessage(websocket.TextMessage, []byte(dummyRequestListPins))
			msgType, msg, err = conn.ReadMessage()
			if err != nil {
				log.Errorf("Cannot read message")
				break
			}
			log.Infof("Response message content: ", string(msg))

			log.Infof("Sending message: connection: %v, message: %v, uri: %v", i%globalCfg.Connections, dummyRequestUnpinFileFmt, uri)
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(dummyRequestUnpinFileFmt, uri)))
			msgType, msg, err = conn.ReadMessage()
			if err != nil {
				log.Errorf("Cannot read message")
				break
			}
			log.Infof("Response message content: ", string(msg))

			i++
			log.Infof("Sent!")
		default:
			continue
		}
	}
}
