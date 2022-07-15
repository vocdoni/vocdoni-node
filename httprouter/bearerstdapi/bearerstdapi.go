package bearerstdapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
)

const (
	// MethodAccessTypePrivate for private requests
	MethodAccessTypePrivate = "private"
	// MethodAccessTypeQuota for rate-limited quota requests
	MethodAccessTypeQuota = "quota"
	// MethodAccessTypePublic for public requests
	MethodAccessTypePublic = "public"
	// MethodAccessTypeAdmin for admin requests
	MethodAccessTypeAdmin = "admin"

	namespace         = "bearerStd"
	bearerPrefix      = "Bearer "
	HTTPstatusCodeOK  = 200
	HTTPstatusCodeErr = 400
)

// BearerStandardAPI is a namespace handler for the httpRouter with Bearer authorization
type BearerStandardAPI struct {
	router         *httprouter.HTTProuter
	basePath       string
	authTokens     sync.Map
	adminToken     string
	adminTokenLock sync.RWMutex
	verboseAuthLog bool
}

// BearerStandardAPIdata is the data type used by the BearerStandardAPI.
// On handler functions Message.Data can be cast safely to this type.
type BearerStandardAPIdata struct {
	Data      []byte
	AuthToken string
}

// BearerStdAPIhandler is the handler function used by the bearer std API httprouter implementation
type BearerStdAPIhandler = func(*BearerStandardAPIdata, *httprouter.HTTPContext) error

// ErrorMsg is the error returned by bearer std API
type ErrorMsg struct {
	Error string `json:"error"`
}

// NewBearerStandardAPI returns a BearerStandardAPI initialized type
func NewBearerStandardAPI(router *httprouter.HTTProuter, baseRoute string) (*BearerStandardAPI, error) {
	if router == nil {
		panic("httprouter is nil")
	}
	if len(baseRoute) == 0 || baseRoute[0] != '/' {
		return nil, fmt.Errorf("invalid base route (%s), it must start with /", baseRoute)
	}
	// Remove trailing slash
	if len(baseRoute) > 1 {
		baseRoute = strings.TrimSuffix(baseRoute, "/")
	}
	bsa := BearerStandardAPI{router: router, basePath: baseRoute}
	router.AddNamespace(namespace, &bsa)
	return &bsa, nil
}

// AuthorizeRequest is a function for the RouterNamespace interface.
// On private handlers checks if the supplied bearer token have still request credits
func (b *BearerStandardAPI) AuthorizeRequest(data interface{},
	accessType httprouter.AuthAccessType) (bool, error) {
	msg, ok := data.(*BearerStandardAPIdata)
	if !ok {
		panic("type is not bearerStandardApi")
	}
	switch accessType {
	case httprouter.AccessTypeAdmin:
		b.adminTokenLock.RLock()
		defer b.adminTokenLock.RUnlock()
		if msg.AuthToken != b.adminToken {
			return false, fmt.Errorf("admin token not valid")
		}
		return true, nil
	case httprouter.AccessTypePrivate:
		_, ok = b.authTokens.Load(msg.AuthToken)
		if !ok {
			return false, fmt.Errorf("auth token not valid")
		}
		return true, nil
	case httprouter.AccessTypeQuota:
		remainingReqs, ok := b.authTokens.Load(msg.AuthToken)
		if !ok || remainingReqs.(int64) < 1 {
			return false, fmt.Errorf("no more requests available")
		}
		b.authTokens.Store(msg.AuthToken, remainingReqs.(int64)-1)
		return true, nil
	default:
		return true, nil
	}
}

// ProcessData is a function for the RouterNamespace interface.
// The body of the http requests and the bearer auth token are readed.
func (b *BearerStandardAPI) ProcessData(req *http.Request) (interface{}, error) {
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("HTTP connection closed: (%v)", err)
	}
	if len(reqBody) > 0 {
		log.Debugf("request: %s", reqBody)
	}
	msg := &BearerStandardAPIdata{
		Data:      reqBody,
		AuthToken: strings.TrimPrefix(req.Header.Get("Authorization"), bearerPrefix),
	}
	if b.verboseAuthLog && msg.AuthToken != "" {
		fmt.Printf("[BearerAPI/%d/%s] %s {%s}\n", time.Now().Unix(), msg.AuthToken, req.URL.RequestURI(), reqBody)
	}
	return msg, nil
}

// RegisterMethod adds a new method under the URL pattern.
// The pattern URL can contain variable names by using braces, such as /send/{name}/hello
// The pattern can also contain wildcard at the end of the path, such as /send/{name}/hello/*
// The accessType can be of type private, public or admin.
func (b *BearerStandardAPI) RegisterMethod(pattern, HTTPmethod string,
	accessType string, handler BearerStdAPIhandler) error {
	if pattern[0] != '/' {
		panic("pattern must start with /")
	}
	routerHandler := func(msg httprouter.Message) {
		bsaMsg := msg.Data.(*BearerStandardAPIdata)
		if err := handler(bsaMsg, msg.Context); err != nil {
			data, err2 := json.Marshal(&ErrorMsg{Error: err.Error()})
			if err2 != nil {
				log.Warn(err2)
				return
			}
			if err := msg.Context.Send(data, HTTPstatusCodeErr); err != nil {
				log.Warn(err)
			}
		}
	}

	path := path.Join(b.basePath, pattern)
	switch accessType {
	case "public":
		b.router.AddPublicHandler(namespace, path, HTTPmethod, routerHandler)
	case "quota":
		b.router.AddQuotaHandler(namespace, path, HTTPmethod, routerHandler)
	case "private":
		b.router.AddPrivateHandler(namespace, path, HTTPmethod, routerHandler)
	case "admin":
		b.router.AddAdminHandler(namespace, path, HTTPmethod, routerHandler)
	default:
		return fmt.Errorf("method access type not implemented: %s", accessType)
	}
	log.Infof("registered %s %s method for path %s", HTTPmethod, accessType, path)
	return nil
}

// SetAdminToken sets the bearer admin token capable to execute admin handlers
func (b *BearerStandardAPI) SetAdminToken(bearerToken string) {
	b.adminTokenLock.Lock()
	defer b.adminTokenLock.Unlock()
	b.adminToken = bearerToken
}

// AddAuthToken adds a new bearer token capable to perform up to n requests
func (b *BearerStandardAPI) AddAuthToken(bearerToken string, requests int64) {
	b.authTokens.Store(bearerToken, requests)
}

// DelAuthToken removes a bearer token (will be not longer valid)
func (b *BearerStandardAPI) DelAuthToken(bearerToken string) {
	b.authTokens.Delete(bearerToken)
}

// GetAuthTokens returns the number of pending requests credits for a bearer token
func (b *BearerStandardAPI) GetAuthTokens(bearerToken string) int64 {
	ts, ok := b.authTokens.Load(bearerToken)
	if !ok {
		return 0
	}
	return ts.(int64)
}

// EnableVerboseAuthLog prints on stdout the details of every request performed with auth token.
// It can be used for keeping track of private/admin actions on a service.
func (b *BearerStandardAPI) EnableVerboseAuthLog() {
	b.verboseAuthLog = true
}
