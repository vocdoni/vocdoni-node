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

type requestMethod func(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys)

func fetchFileMethod(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	var fileRequest types.FetchFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		log.Warnf("couldn't decode into FetchFileRequest type from request %v", msg.Data)
		return
	}
	log.Infof("called method fetchFile, uri %s", fileRequest.Request.URI)
	go fetchFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)
}

func fetchFile(uri, requestId string, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
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
			content, err = storage.Retrieve(hash)
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
		sendError(transport, signer, msg, requestId, errMsg)
	} else {
		b64content := base64.StdEncoding.EncodeToString(content)
		log.Debugf("file fetched, b64 size %d", len(b64content))
		var response types.FetchResponse
		response.ID = requestId
		response.Response.Content = b64content
		response.Response.Request = requestId
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			errMsg = fmt.Sprintf("error marshaling response body: %s", err)
			sendError(transport, signer, msg, requestId, errMsg)
		} else {
			transport.Send(buildReply(msg, rawResponse))
		}
	}
}

func sendError(transport net.Transport, signer signature.SignKeys, msg types.Message, requestID, errMsg string) {
	log.Warn(errMsg)
	var err error
	var response types.ErrorResponse
	response.ID = requestID
	response.Error.Request = requestID
	response.Error.Timestamp = int32(time.Now().Unix())
	response.Error.Message = errMsg
	response.Signature, err = signer.SignJSON(response.Error)
	if err != nil {
		log.Warn(err.Error())
	}
	rawResponse, err := json.Marshal(response)
	if err != nil {
		log.Warnf("error marshaling response body: %s", err)
	}
	transport.Send(buildReply(msg, rawResponse))
}

func addFileMethod(msg types.Message, rawRequest []byte, storage data.Storage,
	transport net.Transport, signer signature.SignKeys) {
	var fileRequest types.AddFileRequest
	var errMsg string
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		errMsg = fmt.Sprintf("could not unmarshal AddFileRequest (%s)", err.Error())
		sendError(transport, signer, msg, fileRequest.ID, errMsg)
		return
	}
	authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
	if err != nil {
		errMsg = fmt.Sprintf("wrong authorization: %s", err.Error())
		sendError(transport, signer, msg, fileRequest.ID, errMsg)
		return
	}
	if authorized {
		content := fileRequest.Request.Content
		b64content, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			errMsg = "could not decode base64 content"
			sendError(transport, signer, msg, fileRequest.ID, errMsg)
			return
		}
		reqType := fileRequest.Request.Type

		go addFile(reqType, fileRequest.ID, b64content, msg, storage, transport, signer)

	} else {
		errMsg = "unauthorized"
		sendError(transport, signer, msg, fileRequest.ID, errMsg)
	}
}

func addFile(reqType, requestId string, b64content []byte, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Infof("calling addFile")
	switch reqType {
	case "swarm":
		// TODO
		break
	case "ipfs":
		cid, err := storage.Publish(b64content)
		if err != nil {
			sendError(transport, signer, msg, requestId,
				fmt.Sprintf("cannot add file (%s)", err.Error()))
			return
		}
		log.Debugf("added file %s, b64 size of %d", cid, len(b64content))
		// ipfsRouteBaseURL := "ipfs://"
		var response types.AddResponse
		response.ID = requestId
		response.Response.Request = requestId
		response.Response.Timestamp = int32(time.Now().Unix())
		// response.Response.URI = ipfsRouteBaseURL + cid
		response.Response.URI = cid
		response.Signature, err = signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			sendError(transport, signer, msg, requestId,
				fmt.Sprintf("could not unmarshal response (%s)", err.Error()))
		} else {
			transport.Send(buildReply(msg, rawResponse))
		}
	}

}

func pinListMethod(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	var fileRequest types.PinListRequest
	var errMsg string
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		errMsg = fmt.Sprintf("couldn't decode into PinListRequest type from request (%s)", err.Error())
		sendError(transport, signer, msg, fileRequest.ID, errMsg)
		return
	}
	authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
	if err != nil {
		errMsg = fmt.Sprintf("error checking authorization (%s)", err.Error())
		sendError(transport, signer, msg, fileRequest.ID, errMsg)
		return
	}
	if authorized {
		go pinList(fileRequest.ID, msg, storage, transport, signer)
	} else {
		sendError(transport, signer, msg, fileRequest.ID, "Unauthorized")
	}
}

func pinList(requestId string, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	var errMsg string
	log.Info("calling PinList")
	pins, err := storage.ListPins()
	if err != nil {
		errMsg = fmt.Sprintf("internal error fetching pins (%s)", err.Error())
		sendError(transport, signer, msg, requestId, errMsg)
		return
	}
	pinsJsonArray, err := json.Marshal(pins)
	if err != nil {
		errMsg = fmt.Sprintf("internal error parsing pins (%s)", err.Error())
		sendError(transport, signer, msg, requestId, errMsg)
		return
	} else {
		var response types.ListPinsResponse
		response.ID = requestId
		response.Response.Files = pinsJsonArray
		response.Response.Request = requestId
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			errMsg = fmt.Sprintf("internal error marshalig response body (%s)", err.Error())
			sendError(transport, signer, msg, requestId, errMsg)
			return
		}
		transport.Send(buildReply(msg, rawResponse))
	}
}

func pinFileMethod(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	var fileRequest types.PinFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		sendError(transport, signer, msg, fileRequest.ID, fmt.Sprintf("could not decode PinFileRequest (%s)", err.Error()))
		return
	}
	authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
	if err != nil {
		sendError(transport, signer, msg, fileRequest.ID, "error checking authorization")
		return
	}
	if authorized {
		go pinFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)
	} else {
		sendError(transport, signer, msg, fileRequest.ID, "unauthorized")
	}
}

func pinFile(uri, requestId string, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Infof("calling PinFile %s", uri)
	err := storage.Pin(uri)
	if err != nil {
		sendError(transport, signer, msg, requestId, fmt.Sprintf("error pinning file (%s)", err.Error()))
	} else {
		var response types.BoolResponse
		response.ID = requestId
		response.Response.OK = true
		response.Response.Request = requestId
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			sendError(transport, signer, msg, requestId, fmt.Sprintf("error marshaling (%s)", err.Error()))
		} else {
			transport.Send(buildReply(msg, rawResponse))
		}
	}
}

func unpinFileMethod(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	var fileRequest types.UnpinFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		sendError(transport, signer, msg, fileRequest.ID,
			fmt.Sprintf("could not decode UnpinFileRequest (%s)", err.Error()))
		return
	}
	authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
	if err != nil {
		sendError(transport, signer, msg, fileRequest.ID, "unauthorized")
		return
	}
	if authorized {
		go unPinFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)
	} else {
		sendError(transport, signer, msg, fileRequest.ID, "unauthorized")
	}
}

func unPinFile(uri, requestId string, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Infof("calling UnPinFile %s", uri)
	err := storage.Unpin(uri)
	if err != nil {
		sendError(transport, signer, msg, requestId, fmt.Sprintf("could not unpin file (%s)", err.Error()))
	} else {
		var response types.BoolResponse
		response.ID = requestId
		response.Response.OK = true
		response.Response.Request = requestId
		response.Response.Timestamp = int32(time.Now().Unix())
		response.Signature, err = signer.SignJSON(response.Response)
		if err != nil {
			log.Warn(err.Error())
		}
		rawResponse, err := json.Marshal(response)
		if err != nil {
			sendError(transport, signer, msg, requestId, fmt.Sprintf("could not unmarshal response (%s)", err.Error()))
		} else {
			transport.Send(buildReply(msg, rawResponse))
		}
	}
}
