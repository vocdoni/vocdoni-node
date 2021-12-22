package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"go.vocdoni.io/dvote/crypto"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/multirpc/example/subpub/message"
	"go.vocdoni.io/dvote/multirpc/router"
	"go.vocdoni.io/dvote/multirpc/transports"
	"go.vocdoni.io/dvote/multirpc/transports/mhttp"
	"go.vocdoni.io/dvote/multirpc/transports/subpubtransport"

	"go.vocdoni.io/dvote/log"
)

var sharedKey = "sharedSecret123"

func processLine(input []byte) *message.MyAPI {
	var req message.MyAPI
	err := json.Unmarshal(input, &req)
	if err != nil {
		panic(err)
	}
	return &req
}

func main() {
	logLevel := flag.String("logLevel", "error", "log level <debug, info, warn, error>")
	privKey := flag.String("key", "", "private key for signature (leave blank for auto-generated key)")
	flag.Parse()
	log.Init(*logLevel, "stdout")
	rand.Seed(time.Now().UnixNano())

	// Set or generate client signing key
	signer := ethereum.NewSignKeys()
	if *privKey != "" {
		if err := signer.AddHexKey(*privKey); err != nil {
			panic(err)
		}
	} else {
		signer.Generate()
	}
	_, priv := signer.HexString()
	conn := transports.Connection{
		Port:         7799,
		Key:          priv,
		Topic:        fmt.Sprintf("%x", ethereum.HashRaw([]byte(sharedKey))),
		TransportKey: sharedKey,
	}
	sp := subpubtransport.SubPubHandle{}

	if err := sp.Init(&conn); err != nil {
		log.Fatal(err)
	}
	msg := make(chan transports.Message)
	sp.Listen(msg)

	pxy, err := proxy("127.0.0.1", 8080)
	if err != nil {
		log.Fatal(err)
	}

	pxy.AddHandler("/p2p", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			log.Errorf("failed to read request body: %v", err)
			return
		}
		req := processLine(body)
		if err := sp.Send(transports.Message{Data: buildRequest(req, signer), Namespace: "/main"}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json")

		log.Infof("waiting for reply...")
		st := time.Now()
		for {
			select {
			case data := <-msg:
				log.Infof("peer:%s data:%s", data.Context.(*subpubtransport.SubPubContext).PeerID, data.Data)
				w.Write(data.Data)
				w.Write([]byte("\n"))
				return
			default:
				if time.Since(st) > time.Second*15 {
					log.Warnf("request timeout")
					w.Write([]byte("{\"error\": \"timeout\"}"))
					w.Write([]byte("\n"))
					return
				}
			}
		}

	})

	select {}
}

func buildRequest(req *message.MyAPI, signer *ethereum.SignKeys) []byte {
	// Prepare and send request
	req.Timestamp = (int32(time.Now().Unix()))
	req.ID = fmt.Sprintf("%d", rand.Intn(1000))
	reqInner, err := crypto.SortedMarshalJSON(req)
	if err != nil {
		log.Fatalf("%s: %v", req.Method, err)
	}
	var signature []byte
	if signer != nil {
		signature, err = signer.Sign(reqInner)
		if err != nil {
			log.Fatalf("%s: %v", req.Method, err)
		}
	}

	reqOuter := router.RequestMessage{
		ID:         req.ID,
		Signature:  signature,
		MessageAPI: reqInner,
	}
	reqBody, err := json.Marshal(reqOuter)
	if err != nil {
		log.Fatalf("%s: %v", req.Method, err)
	}
	return reqBody

}

func proxy(host string, port int32) (*mhttp.Proxy, error) {
	pxy := mhttp.NewProxy()
	pxy.Conn.Address = host
	pxy.Conn.Port = port
	log.Infof("creating proxy service, listening on %s:%d", host, port)
	return pxy, pxy.Init()
}
