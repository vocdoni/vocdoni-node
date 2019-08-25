package main

import (
	"encoding/json"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"os/user"
	"time"

	"github.com/gorilla/websocket"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
)


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

func newConfig() (config.ClusterTestCfg, error) {
	var globalCfg config.ClusterTestCfg
	//setup flags
	usr, err := user.Current()
	if err != nil {
		return globalCfg, err
	}
	defaultDirPath := usr.HomeDir + "/.dvote/clusterTest"
	path := flag.String("cfgpath", defaultDirPath+"/config.yaml", "cfgpath. Specify filepath for clusterTest config")

	flag.String("logLevel", "warn", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")
	flag.Int("pkgSize", 1000, "size in bytes of files to upload")
	flag.StringSlice("targets", []string{"127.0.0.1:9090"}, "target IP and port")
	flag.Int("interval", 1000, "interval between requests in ms")

	flag.Usage = func() {
		io.WriteString(os.Stderr, `Websockets client generator
		Example usage: ./clusterTest -targets=172.17.0.1 -conn=10 -interval=100
		`)
		flag.PrintDefaults()
	}

	flag.Parse()

	viper.SetDefault("logLevel", "warn")
	viper.SetDefault("targets", []string{"127.0.0.1:9090"})
	viper.SetDefault("interval", 1000)
	viper.SetDefault("pkgSize", 1000)

	viper.SetConfigType("yaml")
	if *path == defaultDirPath+"/config.yaml" { //if path left default, write new cfg file if empty or if file doesn't exist.
		if err = viper.SafeWriteConfigAs(*path); err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(defaultDirPath, os.ModePerm)
				if err != nil {
					return globalCfg, err
				}
				err = viper.WriteConfigAs(*path)
				if err != nil {
					return globalCfg, err
				}
			}
		}
	}

	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("targets", flag.Lookup("targets"))
	viper.BindPFlag("interval", flag.Lookup("interval"))
	viper.BindPFlag("pkgSize", flag.Lookup("pkgSize"))

	viper.SetConfigFile(*path)
	err = viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

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

	u := make([]url.URL, len(globalCfg.Targets))
	for i := 0; i < len(globalCfg.Targets); i++ {
		u[i] = url.URL{Scheme: "ws", Host: globalCfg.Targets[i], Path: "/dvote"}
	}

	var conns []*websocket.Conn
	for i := 0; i < len(globalCfg.Targets); i++ {
		c, _, err := websocket.DefaultDialer.Dial(u[i].String(), nil)
		if err != nil {
			log.Error("Failed to connect: ", i, err)
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

	dummyRequestAddFile := `{"id": "req0000002", "request": {"method": "addFile", "name": "My first file", "type": "ipfs", "content": "%s", "timestamp": 1556110671}, "signature": "539"}`
	dummyRequestPinFileFmt := `{"id": "req0000003","request": {"method": "pinFile","uri": "%s","timestamp": 1556110671},"signature": "0x00"}`
	dummyRequestListPins := `{"id": "req0000004","request": {"method": "pinList","timestamp": 1556110671},"signature": "0x00"}`
	//dummyRequestUnpinFileFmt := `{"id": "req0000005","request": {"method": "unpinFile","uri": "%s", "timestamp": 1556110671},"signature": "0x00"}`

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		select {
		case <-timer.C:
			c := r.Intn(len(conns))
				conn := conns[c]
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second*5)); err != nil {
					log.Errorf("Failed to receive pong: %v", err)
				}
			for i := 0; i < 10; i++ {
				msg := make([]byte, globalCfg.PkgSize)
				_, _ = r.Read(msg)
				b64 := base64.StdEncoding.EncodeToString(msg)
				request := fmt.Sprintf(dummyRequestAddFile, b64)
				log.Infof("Conn %d sending message number %d, message %s", c, i, request)
				conn.WriteMessage(websocket.TextMessage, []byte(request))
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
			log.Infof("Sending message: connection: %v, message: %v, uri: %v", c, dummyRequestPinFileFmt, uri)
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(dummyRequestPinFileFmt, uri)))
			msgType, msg, err = conn.ReadMessage()
			if err != nil {
				log.Errorf("Cannot read message")
				break
			}
			log.Infof("Message info: Response message type: %v, Response message content: %v", msgType, string(msg))
			}

			log.Infof("Sending message: connection: %v, message: %v", c, dummyRequestListPins)
			conn.WriteMessage(websocket.TextMessage, []byte(dummyRequestListPins))
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				log.Errorf("Cannot read message")
				break
			}
			log.Infof("Message info: Response message type: %v, Response message content: %v", msgType, string(msg))

			// log.Infof("Sending message: connection: %v, message: %v, uri: %v", c, dummyRequestUnpinFileFmt, uri)
			// conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(dummyRequestUnpinFileFmt, uri)))
			// msgType, msg, err = conn.ReadMessage()
			// if err != nil {
			// 	log.Errorf("Cannot read message")
			// 	break
			// }
			// log.Infof("Response message content: ", string(msg))
		default:
			continue
		}
	}
}
