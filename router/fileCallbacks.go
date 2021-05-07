package router

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

const (
	storageTimeout = time.Second * 20
	maxJSONsize    = 1024 * 10       // 10 KiB
	maxFetchFile   = 1024 * 1024 * 2 // 2 MiB
)

func (r *Router) fetchFile(request routerRequest) {
	log.Debugf("calling FetchFile %s", request.URI)
	parsedURIs := strings.Split(request.URI, ",")
	transportTypes := parseTransportFromURI(parsedURIs)
	var content []byte
	var err error

	found := false
	for idx, t := range transportTypes {
		switch t {
		case "ipfs:":
			found = true
			splt := strings.Split(parsedURIs[idx], "/")
			hash := splt[len(splt)-1]
			ctx, cancel := context.WithTimeout(context.Background(), storageTimeout)
			content, err = r.storage.Retrieve(ctx, hash, maxFetchFile)
			if err == nil && len(content) == 0 {
				err = fmt.Errorf("no content fetched")
			}
			cancel()
		case "bzz:", "bzz-feed":
			found = true
			err = fmt.Errorf("bzz and bzz-feed not implemented yet")
		}
		if found {
			break
		}
	}

	if err != nil {
		r.sendError(request, fmt.Sprintf("error fetching file: (%s)", err))
		return
	}
	if !found {
		r.sendError(request, "error fetching file: (not supported)")
		return
	}

	log.Debugf("file fetched, size %d", len(content))
	var response types.MetaResponse
	response.Content = content
	request.Send(r.buildReply(request, &response))
}

func (r *Router) addFile(request routerRequest) {
	log.Debugf("calling addFile")
	ctx, cancel := context.WithTimeout(context.Background(), storageTimeout)
	defer cancel()
	switch request.Type {
	case "swarm":
		// TODO
	case "ipfs":
		cid, err := r.storage.Publish(ctx, request.Content)
		if err != nil {
			r.sendError(request,
				fmt.Sprintf("cannot add file (%s)", err))
			return
		}
		log.Debugf("added file %s, size %d", cid, len(request.Content))
		var response types.MetaResponse
		response.URI = r.storage.URIprefix() + cid
		request.Send(r.buildReply(request, &response))
	}
}

func (r *Router) addJSONfile(request routerRequest) {
	if len(request.Content) > maxJSONsize {
		r.sendError(request,
			fmt.Sprintf("JSON file too big: %d bytes", len(request.Content)))
		return
	}
	if !isJSON(request.Content) {
		r.sendError(request, "not a JSON file")
		return
	}
	r.addFile(request)
}

func (r *Router) pinList(request routerRequest) {
	log.Debug("calling PinList")
	ctx, cancel := context.WithTimeout(context.Background(), storageTimeout)
	defer cancel()
	pins, err := r.storage.ListPins(ctx)
	if err != nil {
		r.sendError(request, fmt.Sprintf("internal error fetching pins (%s)", err))
		return
	}
	pinsJSONArray, err := json.Marshal(pins)
	if err != nil {
		r.sendError(request, fmt.Sprintf("internal error parsing pins (%s)", err))
		return
	}
	var response types.MetaResponse
	response.Files = pinsJSONArray
	request.Send(r.buildReply(request, &response))
}

func (r *Router) pinFile(request routerRequest) {
	log.Debugf("calling PinFile %s", request.URI)
	ctx, cancel := context.WithTimeout(context.Background(), storageTimeout)
	defer cancel()
	err := r.storage.Pin(ctx, request.URI)
	if err != nil {
		r.sendError(request, fmt.Sprintf("error pinning file (%s)", err))
		return
	}
	var response types.MetaResponse
	request.Send(r.buildReply(request, &response))
}

func (r *Router) unpinFile(request routerRequest) {
	log.Debugf("calling UnPinFile %s", request.URI)
	ctx, cancel := context.WithTimeout(context.Background(), storageTimeout)
	defer cancel()
	err := r.storage.Unpin(ctx, request.URI)
	if err != nil {
		r.sendError(request, fmt.Sprintf("could not unpin file (%s)", err))
		return
	}
	var response types.MetaResponse
	request.Send(r.buildReply(request, &response))
}

func isJSON(c []byte) bool {
	var js interface{}
	return json.Unmarshal(c, &js) == nil
}
