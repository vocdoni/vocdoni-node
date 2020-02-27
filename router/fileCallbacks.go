package router

import (
	"context"
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

func (r *Router) fetchFile(request routerRequest) {
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
			content, err = r.storage.Retrieve(context.TODO(), hash)
			if len(content) > 0 {
				found = true
			}
		case "bzz:", "bzz-feed":
			err = errors.New("bzz and bzz-feed not implemented yet")
		}
	}

	if err != nil {
		errMsg := fmt.Sprintf("error fetching uri %s", request.URI)
		r.sendError(request, errMsg)
		return
	}
	b64content := base64.StdEncoding.EncodeToString(content)
	log.Debugf("file fetched, b64 size %d", len(b64content))
	var response types.ResponseMessage
	response.Content = b64content
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) addFile(request routerRequest) {
	log.Debugf("calling addFile")
	reqType := request.Type
	b64content, err := base64.StdEncoding.DecodeString(request.Content)
	if err != nil {
		errMsg := "could not decode base64 content"
		r.sendError(request, errMsg)
		return
	}
	switch reqType {
	case "swarm":
		// TODO
	case "ipfs":
		cid, err := r.storage.Publish(context.TODO(), b64content)
		if err != nil {
			r.sendError(request,
				fmt.Sprintf("cannot add file (%s)", err))
			return
		}
		log.Debugf("added file %s, b64 size of %d", cid, len(b64content))
		var response types.ResponseMessage
		response.URI = r.storage.URIprefix() + cid
		r.transport.Send(r.buildReply(request, response))
	}
}

func (r *Router) pinList(request routerRequest) {
	log.Debug("calling PinList")
	pins, err := r.storage.ListPins(context.TODO())
	if err != nil {
		errMsg := fmt.Sprintf("internal error fetching pins (%s)", err)
		r.sendError(request, errMsg)
		return
	}
	pinsJSONArray, err := json.Marshal(pins)
	if err != nil {
		errMsg := fmt.Sprintf("internal error parsing pins (%s)", err)
		r.sendError(request, errMsg)
		return
	}
	var response types.ResponseMessage
	response.Files = pinsJSONArray
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) pinFile(request routerRequest) {
	log.Debugf("calling PinFile %s", request.URI)
	err := r.storage.Pin(context.TODO(), request.URI)
	if err != nil {
		r.sendError(request, fmt.Sprintf("error pinning file (%s)", err))
		return
	}
	var response types.ResponseMessage
	r.transport.Send(r.buildReply(request, response))
}

func (r *Router) unpinFile(request routerRequest) {
	log.Debugf("calling UnPinFile %s", request.URI)
	err := r.storage.Unpin(context.TODO(), request.URI)
	if err != nil {
		r.sendError(request, fmt.Sprintf("could not unpin file (%s)", err))
		return
	}
	var response types.ResponseMessage
	r.transport.Send(r.buildReply(request, response))
}
