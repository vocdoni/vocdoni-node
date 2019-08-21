package router

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
	"fmt"

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
	log.Infof("request is: %v", censusRequest)
	go censusLocal(msg, censusRequest, router)
}

func censusLocal(msg types.Message, crm types.CensusRequestMessage, router *Router) {
	var err error
	var response types.CensusResponseMessage
	cresponse := router.census.Handler(&crm.Request, true)
	response.Response = *cresponse
	//response.Request = router.census.Handler(&crm.Request, true)
	response.ID = crm.ID
	
	response.Response.TimeStamp = int32(time.Now().Unix())
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
