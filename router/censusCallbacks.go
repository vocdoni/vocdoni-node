package router

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	signature "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/types"
)

func censusProxyMethod(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Info("Census request sent to method")
	var censusRequest types.CensusRequestMessage
	if err := json.Unmarshal(msg.Data, &censusRequest); err != nil {
		log.Warnf("couldn't decode into CensusRequest type from request %v", msg.Data)
		return
	}
	log.Infof("Message data is: %s", msg.Data)
	log.Infof("Request is: %v", censusRequest)
	log.Infof("Proxying census request to %s", censusRequest.Request.CensusURI)
	go censusProxy(msg, censusRequest, transport)
}

// Strips CensusURI and forwards to census service, send reply back to client
func censusProxy(msg types.Message, censusRequestMessage types.CensusRequestMessage, transport net.Transport) {
	censusURI, err := url.Parse(censusRequestMessage.Request.CensusURI)
	if err != nil {
		log.Error("Provided census URI was malformed")
		//Send error to client?
	}
	if censusURI.Scheme != "https" && censusURI.Scheme != "http" {
		log.Errorf("Provided census URI did not use correct scheme (https or http), got %s", censusURI.Scheme)
		//Send error to client?
	}

	censusRequestMessage.Request.CensusURI = ""
	requestBody, err := json.Marshal(censusRequestMessage)
	if err != nil {
		log.Error("Could not unmarshall census request")
		//send error
	}
	log.Infof("Sending request to census service: %s", requestBody)
	resp, err := http.Post(censusURI.String(), "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		log.Error("error handling census service response body")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("error reading census response body to bytes")
	}
	log.Infof("got response from census service: %s", body)
	//success, send resp to client
	transport.Send(buildReply(msg, body))
}
