package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	signature "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

func censusLocalMethod(msg types.Message, rawRequest []byte, router *Router) {
	log.Info("got local census request")
	var censusRequest types.CensusRequestMessage
	if err := json.Unmarshal(msg.Data, &censusRequest); err != nil {
		log.Warnf("couldn't decode into CensusRequest type from request %v", msg.Data)
		return
	}
	authorized, err := router.signer.VerifySender(string(rawRequest), censusRequest.Signature)
	if err != nil {
		log.Debugf("client not authorized for private methods")
	}
	var addr [20]byte
	if len(censusRequest.Signature) > 0 {
		addr, err = signature.AddrFromSignature(string(rawRequest), censusRequest.Signature)
		if err != nil {
			log.Warnf("error extracting Ethereum address from signature: %s", err)
		}
	}
	go censusLocal(msg, &censusRequest, router, authorized, fmt.Sprintf("%x", addr))
}

func censusLocal(msg types.Message, crm *types.CensusRequestMessage, router *Router, auth bool, addr string) {
	var response types.CensusResponseMessage
	var err error
	log.Debugf("recovered address %s", addr)
	cresponse := router.census.Handler(&crm.Request, auth, addr)
	response.Response = *cresponse
	response.ID = crm.ID
	response.Signature, err = router.signer.SignJSON(response.Response)
	if err != nil {
		log.Warn(err.Error())
	}
	rawResponse, err := json.Marshal(response)
	if err != nil {
		sendError(router.transport, router.signer, msg, crm.ID, fmt.Sprintf("could not unmarshal response (%s)", err.Error()))
	} else {
		log.Infof("sending census resposne: %s", rawResponse)
		router.transport.Send(buildReply(msg, rawResponse))
	}
}

func censusProxyMethod(msg types.Message, rawRequest []byte, router *Router) {
	log.Info("Census request sent to method")
	var censusRequest types.CensusRequestMessage
	if err := json.Unmarshal(msg.Data, &censusRequest); err != nil {
		log.Warnf("couldn't decode into CensusRequest type from request %v", msg.Data)
		return
	}
	log.Infof("Message data is: %s", msg.Data)
	log.Infof("Request is: %v", censusRequest)
	log.Infof("Proxying census request to %s", censusRequest.Request.CensusURI)
	go censusProxy(msg, censusRequest, router)
}

// Strips CensusURI and forwards to census service, send reply back to client
func censusProxy(msg types.Message, censusRequestMessage types.CensusRequestMessage, router *Router) {
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
	router.transport.Send(buildReply(msg, body))
}
