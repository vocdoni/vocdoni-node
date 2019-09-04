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

	signature "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/types"
)

// TO-DO msg is not longer needed since the request Data is into request.rawRequest

func fetchFileMethod(msg types.Message, request routerRequest, router *Router) {
	var fileRequest types.FetchFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		log.Warnf("couldn't decode into FetchFileRequest type from request %v", msg.Data)
		return
	}
	log.Infof("called method fetchFile, uri %s", fileRequest.Request.URI)
	go fetchFile(fileRequest.Request.URI, fileRequest.ID, msg, router)
}

func fetchFile(uri, requestID string, msg types.Message, router *Router) {
	log.Debugf("calling FetchFile %s", uri)
	parsedURIs := parseUrisContent(uri)
	transportTypes := parseTransportFromUri(parsedURIs)
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
		sendError(router.transport, router.signer, msg, requestID, errMsg)
	} else {
		b64content := base64.StdEncoding.EncodeToString(content)
		log.Debugf("file fetched, b64 size %d", len(b64content))
		var response types.FetchResponse
		response.ID = requestID
		response.Response.Content = b64content
		response.Response.Request = requestID
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = router.signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			errMsg = fmt.Sprintf("error marshaling response body: %s", err)
			sendError(router.transport, router.signer, msg, requestID, errMsg)
		} else {
			log.Debugf("sending response %s", rawResponse)
			router.transport.Send(buildReply(msg, rawResponse))
		}
	}
}

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

func addFile(reqType, requestID string, b64content []byte, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Infof("calling addFile")
	switch reqType {
	case "swarm":
		// TODO
		break
	case "ipfs":
		cid, err := storage.Publish(b64content)
		if err != nil {
			sendError(transport, signer, msg, requestID,
				fmt.Sprintf("cannot add file (%s)", err.Error()))
			return
		}
		log.Debugf("added file %s, b64 size of %d", cid, len(b64content))
		var response types.AddResponse
		response.ID = requestID
		response.Response.Request = requestID
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Response.URI = storage.GetURIprefix() + cid
		response.Signature, err = signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			sendError(transport, signer, msg, requestID,
				fmt.Sprintf("could not unmarshal response (%s)", err.Error()))
		} else {
			log.Debugf("sending response %s", rawResponse)
			transport.Send(buildReply(msg, rawResponse))
		}
	}
}

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

func pinList(requestID string, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	var errMsg string
	log.Info("calling PinList")
	pins, err := storage.ListPins()
	if err != nil {
		errMsg = fmt.Sprintf("internal error fetching pins (%s)", err.Error())
		sendError(transport, signer, msg, requestID, errMsg)
		return
	}
	pinsJSONArray, err := json.Marshal(pins)
	if err != nil {
		errMsg = fmt.Sprintf("internal error parsing pins (%s)", err.Error())
		sendError(transport, signer, msg, requestID, errMsg)
	} else {
		var response types.ListPinsResponse
		response.ID = requestID
		response.Response.Files = pinsJSONArray
		response.Response.Request = requestID
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			errMsg = fmt.Sprintf("internal error marshalig response body (%s)", err.Error())
			sendError(transport, signer, msg, requestID, errMsg)
			return
		}
		transport.Send(buildReply(msg, rawResponse))
	}
}

func pinFileMethod(msg types.Message, request routerRequest, router *Router) {
	var fileRequest types.PinFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		sendError(router.transport, router.signer, msg, fileRequest.ID, fmt.Sprintf("could not decode PinFileRequest (%s)", err.Error()))
		return
	}
	pinFile(fileRequest.Request.URI, fileRequest.ID, msg, router.storage, router.transport, router.signer)
}

func pinFile(uri, requestID string, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Infof("calling PinFile %s", uri)
	err := storage.Pin(uri)
	if err != nil {
		sendError(transport, signer, msg, requestID, fmt.Sprintf("error pinning file (%s)", err.Error()))
	} else {
		var response types.BoolResponse
		response.ID = requestID
		response.Response.OK = true
		response.Response.Request = requestID
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			sendError(transport, signer, msg, requestID, fmt.Sprintf("error marshaling (%s)", err.Error()))
		} else {
			log.Debugf("sending response %s", rawResponse)
			transport.Send(buildReply(msg, rawResponse))
		}
	}
}

func unpinFileMethod(msg types.Message, request routerRequest, router *Router) {
	var fileRequest types.UnpinFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		sendError(router.transport, router.signer, msg, fileRequest.ID,
			fmt.Sprintf("could not decode UnpinFileRequest (%s)", err.Error()))
	} else {
		unPinFile(fileRequest.Request.URI, fileRequest.ID, msg, router.storage, router.transport, router.signer)
	}
}

func unPinFile(uri, requestID string, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Infof("calling UnPinFile %s", uri)
	err := storage.Unpin(uri)
	if err != nil {
		sendError(transport, signer, msg, requestID, fmt.Sprintf("could not unpin file (%s)", err.Error()))
	} else {
		var response types.BoolResponse
		response.ID = requestID
		response.Response.OK = true
		response.Response.Request = requestID
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			sendError(transport, signer, msg, requestID, fmt.Sprintf("could not unmarshal response (%s)", err.Error()))
		} else {
			log.Debugf("sending response %s", rawResponse)
			transport.Send(buildReply(msg, rawResponse))
		}
	}
}
