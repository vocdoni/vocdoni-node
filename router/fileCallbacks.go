package router

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"errors"
	"strings"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

func fetchFile(request routerRequest, router *Router) {
	log.Debugf("calling FetchFile %s", request.URI)
	parsedURIs := strings.Split(request.URI, ",")
	transportTypes := parseTransportFromURI(parsedURIs)
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
			err = errors.New("bzz and bzz-feed not implemented yet")
		}
	}

	if err != nil {
		errMsg := fmt.Sprintf("error fetching uri %s", request.URI)
		router.sendError(request, errMsg)
		return
	}
	b64content := base64.StdEncoding.EncodeToString(content)
	log.Debugf("file fetched, b64 size %d", len(b64content))
	var response types.ResponseMessage
	response.Content = b64content
	router.transport.Send(router.buildReply(request, response))
}

func addFile(request routerRequest, router *Router) {
	log.Debugf("calling addFile")
	reqType := request.Type
	b64content, err := base64.StdEncoding.DecodeString(request.Content)
	if err != nil {
		errMsg := "could not decode base64 content"
		router.sendError(request, errMsg)
		return
	}
	switch reqType {
	case "swarm":
		// TODO
	case "ipfs":
		cid, err := router.storage.Publish(b64content)
		if err != nil {
			router.sendError(request,
				fmt.Sprintf("cannot add file (%s)", err))
			return
		}
		log.Debugf("added file %s, b64 size of %d", cid, len(b64content))
		var response types.ResponseMessage
		response.URI = router.storage.URIprefix() + cid
		router.transport.Send(router.buildReply(request, response))
	}
}

func pinList(request routerRequest, router *Router) {
	log.Debug("calling PinList")
	pins, err := router.storage.ListPins()
	if err != nil {
		errMsg := fmt.Sprintf("internal error fetching pins (%s)", err)
		router.sendError(request, errMsg)
		return
	}
	pinsJSONArray, err := json.Marshal(pins)
	if err != nil {
		errMsg := fmt.Sprintf("internal error parsing pins (%s)", err)
		router.sendError(request, errMsg)
		return
	}
	var response types.ResponseMessage
	response.Files = pinsJSONArray
	router.transport.Send(router.buildReply(request, response))
}

func pinFile(request routerRequest, router *Router) {
	log.Debugf("calling PinFile %s", request.URI)
	err := router.storage.Pin(request.URI)
	if err != nil {
		router.sendError(request, fmt.Sprintf("error pinning file (%s)", err))
		return
	}
	var response types.ResponseMessage
	router.transport.Send(router.buildReply(request, response))
}

func unpinFile(request routerRequest, router *Router) {
	log.Debugf("calling UnPinFile %s", request.URI)
	err := router.storage.Unpin(request.URI)
	if err != nil {
		router.sendError(request, fmt.Sprintf("could not unpin file (%s)", err))
		return
	}
	var response types.ResponseMessage
	router.transport.Send(router.buildReply(request, response))
}
