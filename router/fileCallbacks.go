package router

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"errors"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

func fetchFile(request routerRequest, router *Router) {
	uri := request.URI
	log.Debugf("calling FetchFile %s", uri)
	parsedURIs := strings.Split(uri, ",")
	transportTypes := parseTransportFromURI(parsedURIs)
	var resp *http.Response
	var content []byte
	var err error
	var errMsg string

	found := false
	for idx, t := range transportTypes {
		if found {
			break
		}
		switch t {
		case "http:", "https:":
			resp, err = http.Get(parsedURIs[idx])
			if err == nil {
				defer resp.Body.Close()
				content, err = ioutil.ReadAll(resp.Body)
				if len(content) > 0 {
					found = true
				}
			}
		case "ipfs:":
			splt := strings.Split(parsedURIs[idx], "/")
			hash := splt[len(splt)-1]
			content, err = router.storage.Retrieve(hash)
			if len(content) > 0 {
				found = true
			}
		case "bzz:", "bzz-feed":
			err = errors.New("Bzz and Bzz-feed not implemented yet")
		}
	}

	if err != nil {
		errMsg = fmt.Sprintf("error fetching uri %s", uri)
		sendError(router.transport, router.signer, request.context, request.id, errMsg)
		return
	}
	b64content := base64.StdEncoding.EncodeToString(content)
	log.Debugf("file fetched, b64 size %d", len(b64content))
	var response types.ResponseMessage
	response.ID = request.id
	response.Content = b64content
	response.Request = request.id
	response.Timestamp = int32(time.Now().Unix())
	response.Signature, err = router.signer.SignJSON(response.MetaResponse)
	if err != nil {
		log.Warn(err)
	}
	rawResponse, err := json.Marshal(response)
	if err != nil {
		errMsg = fmt.Sprintf("error marshaling response body: %s", err)
		sendError(router.transport, router.signer, request.context, request.id, errMsg)
	} else {
		log.Debugf("sending response %s", rawResponse)
		router.transport.Send(buildReply(request.context, rawResponse))
	}
}

func addFile(request routerRequest, router *Router) {
	log.Debugf("calling addFile")
	reqType := request.Type
	b64content, err := base64.StdEncoding.DecodeString(request.Content)
	if err != nil {
		errMsg := "could not decode base64 content"
		sendError(router.transport, router.signer, request.context, request.id, errMsg)
		return
	}
	switch reqType {
	case "swarm":
		// TODO
	case "ipfs":
		cid, err := router.storage.Publish(b64content)
		if err != nil {
			sendError(router.transport, router.signer, request.context, request.id,
				fmt.Sprintf("cannot add file (%s)", err))
			return
		}
		log.Debugf("added file %s, b64 size of %d", cid, len(b64content))
		var response types.ResponseMessage
		response.ID = request.id
		response.Request = request.id
		response.Timestamp = int32(time.Now().Unix())
		response.URI = router.storage.GetURIprefix() + cid
		response.Signature, err = router.signer.SignJSON(response.MetaResponse)
		if err != nil {
			log.Warn(err)
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			sendError(router.transport, router.signer, request.context, request.id,
				fmt.Sprintf("could not unmarshal response (%s)", err))
		} else {
			log.Debugf("sending response %s", rawResponse)
			router.transport.Send(buildReply(request.context, rawResponse))
		}
	}
}

func pinList(request routerRequest, router *Router) {
	var errMsg string
	log.Debug("calling PinList")
	pins, err := router.storage.ListPins()
	if err != nil {
		errMsg = fmt.Sprintf("internal error fetching pins (%s)", err)
		sendError(router.transport, router.signer, request.context, request.id, errMsg)
		return
	}
	pinsJSONArray, err := json.Marshal(pins)
	if err != nil {
		errMsg = fmt.Sprintf("internal error parsing pins (%s)", err)
		sendError(router.transport, router.signer, request.context, request.id, errMsg)
		return
	}
	var response types.ResponseMessage
	response.ID = request.id
	response.Files = pinsJSONArray
	response.Request = request.id
	response.Timestamp = int32(time.Now().Unix())
	response.Signature, err = router.signer.SignJSON(response.MetaResponse)
	if err != nil {
		log.Warn(err)
	}
	rawResponse, err := json.Marshal(response)
	if err != nil {
		errMsg = fmt.Sprintf("internal error marshalig response body (%s)", err)
		sendError(router.transport, router.signer, request.context, request.id, errMsg)
		return
	}
	router.transport.Send(buildReply(request.context, rawResponse))
}

func pinFile(request routerRequest, router *Router) {
	uri := request.URI
	log.Debugf("calling PinFile %s", uri)
	err := router.storage.Pin(uri)
	if err != nil {
		sendError(router.transport, router.signer, request.context, request.id, fmt.Sprintf("error pinning file (%s)", err))
		return
	}
	var response types.ResponseMessage
	response.ID = request.id
	response.Ok = types.True
	response.Request = request.id
	response.Timestamp = int32(time.Now().Unix())
	response.Signature, err = router.signer.SignJSON(response.MetaResponse)
	if err != nil {
		log.Warn(err)
	}
	rawResponse, err := json.Marshal(response)
	if err != nil {
		sendError(router.transport, router.signer, request.context, request.id, fmt.Sprintf("error marshaling (%s)", err))
	} else {
		log.Debugf("sending response %s", rawResponse)
		router.transport.Send(buildReply(request.context, rawResponse))
	}
}

func unpinFile(request routerRequest, router *Router) {
	uri := request.URI
	log.Debugf("calling UnPinFile %s", uri)
	err := router.storage.Unpin(uri)
	if err != nil {
		sendError(router.transport, router.signer, request.context, request.id, fmt.Sprintf("could not unpin file (%s)", err))
		return
	}
	var response types.ResponseMessage
	response.ID = request.id
	response.Ok = types.True
	response.Request = request.id
	response.Timestamp = int32(time.Now().Unix())
	response.Signature, err = router.signer.SignJSON(response.MetaResponse)
	if err != nil {
		log.Warn(err)
	}
	rawResponse, err := json.Marshal(response)
	if err != nil {
		sendError(router.transport, router.signer, request.context, request.id, fmt.Sprintf("could not unmarshal response (%s)", err))
	} else {
		log.Debugf("sending response %s", rawResponse)
		router.transport.Send(buildReply(request.context, rawResponse))
	}
}
