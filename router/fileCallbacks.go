package router

import (

	"encoding/json"
	"encoding/base64"
	"net/http"
	"io/ioutil"

	"log"
	"strings"
	"time"
	"errors"
	"fmt"

signature "github.com/vocdoni/go-dvote/crypto/signature_ecdsa"
"github.com/vocdoni/go-dvote/data"
"github.com/vocdoni/go-dvote/types"
"github.com/vocdoni/go-dvote/net"
)

type requestMethod func(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys)

func fetchFileMethod(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	var fileRequest types.FetchFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		log.Printf("Couldn't decode into FetchFileRequest type from request %v", msg.Data)
		return
	}
	log.Printf("Called method fetchFile, uri %s", fileRequest.Request.URI)
	go fetchFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)
}

func fetchFile(uri, requestId string, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Printf("Calling FetchFile %s", uri)
	parsedURIs := parseUrisContent(uri)
	transportTypes := parseTransportFromUri(parsedURIs)
	var resp *http.Response
	var content []byte
	var err error
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
		fmt.Printf(fmt.Sprintf("Error fetching uri %s", uri))
		transport.Send(buildReply(msg, buildFailReply(requestId, "Error fetching uri")))
	} else {
		b64content := base64.StdEncoding.EncodeToString(content)
		log.Printf("File fetched, b64 size %d", len(b64content))
		var response types.FetchResponse
		response.ID = requestId
		response.Response.Content = b64content
		response.Response.Request = requestId
		response.Response.Timestamp = time.Now().UnixNano()
		response.Signature = signMsg(response.Response, signer)
		rawResponse, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response body: %s", err)
		}
		transport.Send(buildReply(msg, rawResponse))
	}
}


func addFileMethod(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	var fileRequest types.AddFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		log.Printf("Couldn't decode into AddFileRequest type from request %s", msg.Data)
		return
	}
	authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
	if err != nil {
		log.Printf("Wrong authorization: %s", err)
		return
	}
	if authorized {
		content := fileRequest.Request.Content
		b64content, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			log.Printf("Couldn't decode content")
			return
		}
		reqType := fileRequest.Request.Type

		go addFile(reqType, fileRequest.ID, b64content, msg, storage, transport, signer)

	} else {
		transport.Send(buildReply(msg, buildFailReply(fileRequest.ID, "Unauthorized")))
	}
}

func addFile(reqType, requestId string, b64content []byte, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Println("Calling addFile")
	switch reqType {
	case "swarm":
		// TODO
		break
	case "ipfs":
		cid, err := storage.Publish(b64content)
		if err != nil {
			log.Printf("Cannot add file")
		}
		log.Printf("Added file %s, b64 size of %d", cid, len(b64content))
		ipfsRouteBaseURL := "ipfs://"
		var response types.AddResponse
		response.ID = requestId
		response.Response.Request = requestId
		response.Response.Timestamp = time.Now().UnixNano()
		response.Response.URI = ipfsRouteBaseURL + cid
		response.Signature = signMsg(response.Response, signer)
		rawResponse, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response body: %s", err)
		}
		transport.Send(buildReply(msg, rawResponse))
	}

}

func pinListMethod(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	var fileRequest types.PinListRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		log.Printf("Couldn't decode into PinListRequest type from request %s", msg.Data)
		return
	}
	authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
	if err != nil {
		log.Printf("Error checking authorization: %s", err)
		return
	}
	if authorized {
		go pinList(fileRequest.ID, msg, storage, transport, signer)
	} else {
		transport.Send(buildReply(msg, buildFailReply(fileRequest.ID, "Unauthorized")))
	}
}

func pinList(requestId string, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Println("Calling PinList")
	pins, err := storage.ListPins()
	if err != nil {
		log.Printf("Internal error fetching pins")
	}
	pinsJsonArray, err := json.Marshal(pins)
	if err != nil {
		log.Printf("Internal error parsing pins")
	} else {
		var response types.ListPinsResponse
		response.ID = requestId
		response.Response.Files = pinsJsonArray
		response.Response.Request = requestId
		response.Response.Timestamp = time.Now().UnixNano()
		response.Signature = signMsg(response.Response, signer)
		rawResponse, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response body: %s", err)
		}
		transport.Send(buildReply(msg, rawResponse))
	}
}

func pinFileMethod(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	var fileRequest types.PinFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		log.Printf("Couldn't decode into PinFileRequest type from request %s", msg.Data)
		return
	}
	authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
	if err != nil {
		log.Printf("Error checking authorization: %s", err)
		return
	}
	if authorized {
		go pinFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)
	} else {
		transport.Send(buildReply(msg, buildFailReply(fileRequest.ID, "Unauthorized")))
	}
}

func pinFile(uri, requestId string, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Printf("Calling PinFile %s", uri)
	err := storage.Pin(uri)
	if err != nil {
		log.Printf(fmt.Sprintf("Error pinning file %s", uri))
		transport.Send(buildReply(msg, buildFailReply(requestId, "Error pinning file")))
	} else {
		var response types.BoolResponse
		response.ID = requestId
		response.Response.OK = true
		response.Response.Request = requestId
		response.Response.Timestamp = time.Now().UnixNano()
		response.Signature = signMsg(response.Response, signer)
		rawResponse, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response body: %s", err)
		}
		transport.Send(buildReply(msg, rawResponse))
	}
}

func unpinFileMethod(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	var fileRequest types.UnpinFileRequest
	if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
		log.Printf("Couldn't decode into UnpinFileRequest type from request %s", msg.Data)
		return
	}
	authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
	if err != nil {
		log.Printf("Error checking authorization: %s", err)
		return
	}
	if authorized {

		go unPinFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)
	} else {
		transport.Send(buildReply(msg, buildFailReply(fileRequest.ID, "Unauthorized")))
	}
}

func unPinFile(uri, requestId string, msg types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys) {
	log.Printf("Calling UnPinFile %s", uri)
	err := storage.Unpin(uri)
	if err != nil {
		log.Printf(fmt.Sprintf("Error unpinning file %s", uri))
		transport.Send(buildReply(msg, buildFailReply(requestId, "Error unpinning file")))
	} else {
		var response types.BoolResponse
		response.ID = requestId
		response.Response.OK = true
		response.Response.Request = requestId
		response.Response.Timestamp = time.Now().UnixNano()
		response.Signature = signMsg(response.Response, signer)
		rawResponse, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response body: %s", err)
		}
		transport.Send(buildReply(msg, rawResponse))
	}
}