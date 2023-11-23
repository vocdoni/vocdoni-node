package rpcapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/log"
)

const (
	storageTimeout = time.Second * 240
	maxJSONsize    = 1024 * 1024     // 1 MiB
	maxFetchFile   = 1024 * 1024 * 2 // 2 MiB
)

func (r *RPCAPI) fetchFile(request *api.APIrequest) (*api.APIresponse, error) {
	log.Debugf("calling FetchFile %s", request.URI)
	var content []byte
	var err error

	switch strings.Split(request.URI, "://")[0] {
	case "ipfs":
		ctx, cancel := context.WithTimeout(context.Background(), storageTimeout)
		content, err = r.storage.Retrieve(ctx, request.URI, maxFetchFile)
		cancel()
	case "bzz", "bzz-feed":
		err = fmt.Errorf("bzz and bzz-feed not implemented yet")
	default:
		err = fmt.Errorf("transport type not supported")
	}

	if err != nil {
		return nil, fmt.Errorf("error fetching file: %w", err)
	}
	log.Debugf("fetched file of size %d", len(content))
	var response api.APIresponse
	response.Content = content
	return &response, nil
}

func (r *RPCAPI) addFile(request *api.APIrequest) (*api.APIresponse, error) {
	log.Debugf("calling addFile")
	ctx, cancel := context.WithTimeout(context.Background(), storageTimeout)
	defer cancel()
	var response api.APIresponse
	switch request.Type {
	case "swarm":
		// TODO
	case "ipfs":
		cid, err := r.storage.Publish(ctx, request.Content)
		if err != nil {
			return nil, fmt.Errorf("cannot add file (%s)", err)
		}
		log.Debugf("added file %s of size %d", cid, len(request.Content))
		response.URI = r.storage.URIprefix() + cid
		return &response, nil
	}
	return nil, fmt.Errorf("not supported")
}

func (r *RPCAPI) addJSONfile(request *api.APIrequest) (*api.APIresponse, error) {
	if len(request.Content) > maxJSONsize {
		return nil, fmt.Errorf("JSON file too big: %d bytes", len(request.Content))
	}
	if !isJSON(request.Content) {
		return nil, fmt.Errorf("not a JSON file")
	}
	return r.addFile(request)
}

func (r *RPCAPI) pinList(request *api.APIrequest) (*api.APIresponse, error) {
	log.Debug("calling PinList")
	ctx, cancel := context.WithTimeout(context.Background(), storageTimeout)
	defer cancel()
	pins, err := r.storage.ListPins(ctx)
	if err != nil {
		return nil, fmt.Errorf("internal error fetching pins: %w", err)
	}
	pinsJSONArray, err := json.Marshal(pins)
	if err != nil {
		return nil, fmt.Errorf("internal error parsing pins: %w", err)
	}
	var response api.APIresponse
	response.Files = pinsJSONArray
	return &response, nil
}

func (r *RPCAPI) pinFile(request *api.APIrequest) (*api.APIresponse, error) {
	log.Debugf("calling PinFile %s", request.URI)
	ctx, cancel := context.WithTimeout(context.Background(), storageTimeout)
	defer cancel()
	err := r.storage.Pin(ctx, request.URI)
	if err != nil {
		return nil, fmt.Errorf("error pinning file: %w", err)
	}
	var response api.APIresponse
	response.URI = request.URI
	return &response, nil
}

func (r *RPCAPI) unpinFile(request *api.APIrequest) (*api.APIresponse, error) {
	log.Debugf("calling UnPinFile %s", request.URI)
	ctx, cancel := context.WithTimeout(context.Background(), storageTimeout)
	defer cancel()
	err := r.storage.Unpin(ctx, request.URI)
	if err != nil {
		return nil, fmt.Errorf("could not unpin file: %w", err)
	}
	var response api.APIresponse
	response.URI = request.URI
	return &response, nil
}

func isJSON(c []byte) bool {
	var js interface{}
	return json.Unmarshal(c, &js) == nil
}
