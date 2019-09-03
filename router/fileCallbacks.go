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
	uri := request.structured.URI
	log.Debugf("calling FetchFile %s", uri)
	parsedURIs := parseUrisContent(uri)
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
			defer resp.Body.Close()
			content, err = ioutil.ReadAll(resp.Body)
			if content != nil {
				found = true
			}
			break
		case "ipfs:":
			splt := strings.Split(parsedURIs[idx], "/")
			hash := splt[len(splt)-1]
			content, err = router.storage.Retrieve(hash)
			if content != nil {
				found = true
			}
			break
		case "bzz:", "bzz-feed":
			err = errors.New("Bzz and Bzz-feed not implemented yet")
			break
		}
	}

	if err != nil {
		errMsg = fmt.Sprintf("error fetching uri %s", uri)
		sendError(router.transport, router.signer, request.context, request.id, errMsg)
	} else {
		b64content := base64.StdEncoding.EncodeToString(content)
		log.Debugf("file fetched, b64 size %d", len(b64content))
		var response types.FetchResponse
		response.ID = request.id
		response.Response.Content = b64content
		response.Response.Request = request.id
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = router.signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
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
}

/*

func addFileMethod(msg types.Message, request routerRequest, router *Router) {
	var fileRequest types.AddFileRequest
	var errMsg string
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		errMsg = fmt.Sprintf("could not unmarshal AddFileRequest (%s)", err.Error())
		sendError(router.transport, router.signer, msg, fileRequest.ID, errMsg)
		return
	}
	content := fileRequest.Request.Content
	b64content, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		errMsg = "could not decode base64 content"
		sendError(router.transport, router.signer, msg, fileRequest.ID, errMsg)
		return
	}
	reqType := fileRequest.Request.Type
	addFile(reqType, fileRequest.ID, b64content, msg, router.storage, router.transport, router.signer)
}
*/

func addFile(request routerRequest, router *Router) {
	log.Infof("calling addFile")
	reqType := request.structured.Type
	b64content, err := base64.StdEncoding.DecodeString(request.structured.Content)
	if err != nil {
		errMsg := "could not decode base64 content"
		sendError(router.transport, router.signer, request.context, request.id, errMsg)
		return
	}
	switch reqType {
	case "swarm":
		// TODO
		break
	case "ipfs":
		cid, err := router.storage.Publish(b64content)
		if err != nil {
			sendError(router.transport, router.signer, request.context, request.id,
				fmt.Sprintf("cannot add file (%s)", err.Error()))
			return
		}
		log.Debugf("added file %s, b64 size of %d", cid, len(b64content))
		var response types.AddResponse
		response.ID = request.id
		response.Response.Request = request.id
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Response.URI = router.storage.GetURIprefix() + cid
		response.Signature, err = router.signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			sendError(router.transport, router.signer, request.context, request.id,
				fmt.Sprintf("could not unmarshal response (%s)", err.Error()))
		} else {
			log.Debugf("sending response %s", rawResponse)
			router.transport.Send(buildReply(request.context, rawResponse))
		}
	}
}

/*

func pinListMethod(msg types.Message, request routerRequest, router *Router) {
	var fileRequest types.PinListRequest
	var errMsg string
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		errMsg = fmt.Sprintf("couldn't decode into PinListRequest type from request (%s)", err.Error())
		sendError(router.transport, router.signer, msg, fileRequest.ID, errMsg)
		return
	}
	pinList(fileRequest.ID, msg, router.storage, router.transport, router.signer)
}
*/

func pinList(request routerRequest, router *Router) {
	var errMsg string
	log.Info("calling PinList")
	pins, err := router.storage.ListPins()
	if err != nil {
		errMsg = fmt.Sprintf("internal error fetching pins (%s)", err.Error())
		sendError(router.transport, router.signer, request.context, request.id, errMsg)
		return
	}
	pinsJSONArray, err := json.Marshal(pins)
	if err != nil {
		errMsg = fmt.Sprintf("internal error parsing pins (%s)", err.Error())
		sendError(router.transport, router.signer, request.context, request.id, errMsg)
	} else {
		var response types.ListPinsResponse
		response.ID = request.id
		response.Response.Files = pinsJSONArray
		response.Response.Request = request.id
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = router.signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			errMsg = fmt.Sprintf("internal error marshalig response body (%s)", err.Error())
			sendError(router.transport, router.signer, request.context, request.id, errMsg)
			return
		}
		router.transport.Send(buildReply(request.context, rawResponse))
	}
}

/*

func pinFileMethod(msg types.Message, request routerRequest, router *Router) {
	var fileRequest types.PinFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		sendError(router.transport, router.signer, msg, fileRequest.ID, fmt.Sprintf("could not decode PinFileRequest (%s)", err.Error()))
		return
	}
	pinFile(fileRequest.Request.URI, fileRequest.ID, msg, router.storage, router.transport, router.signer)
}

*/

func pinFile(request routerRequest, router *Router) {
	uri := request.structured.URI
	log.Infof("calling PinFile %s", uri)
	err := router.storage.Pin(uri)
	if err != nil {
		sendError(router.transport, router.signer, request.context, request.id, fmt.Sprintf("error pinning file (%s)", err.Error()))
	} else {
		var response types.BoolResponse
		response.ID = request.id
		response.Response.OK = true
		response.Response.Request = request.id
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = router.signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			sendError(router.transport, router.signer, request.context, request.id, fmt.Sprintf("error marshaling (%s)", err.Error()))
		} else {
			log.Debugf("sending response %s", rawResponse)
			router.transport.Send(buildReply(request.context, rawResponse))
		}
	}
}

/*
func unpinFileMethod(msg types.Message, request routerRequest, router *Router) {
	var fileRequest types.UnpinFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		sendError(router.transport, router.signer, msg, fileRequest.ID,
			fmt.Sprintf("could not decode UnpinFileRequest (%s)", err.Error()))
	} else {
		unPinFile(fileRequest.Request.URI, fileRequest.ID, msg, router.storage, router.transport, router.signer)
	}
}
*/

func unpinFile(request routerRequest, router *Router) {
	uri := request.structured.URI
	log.Infof("calling UnPinFile %s", uri)
	err := router.storage.Unpin(uri)
	if err != nil {
		sendError(router.transport, router.signer, request.context, request.id, fmt.Sprintf("could not unpin file (%s)", err.Error()))
	} else {
		var response types.BoolResponse
		response.ID = request.id
		response.Response.OK = true
		response.Response.Request = request.id
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = router.signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			sendError(router.transport, router.signer, request.context, request.id, fmt.Sprintf("could not unmarshal response (%s)", err.Error()))
		} else {
			log.Debugf("sending response %s", rawResponse)
			router.transport.Send(buildReply(request.context, rawResponse))
		}
	}
}
