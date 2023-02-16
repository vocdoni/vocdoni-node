package apirest

import (
	"encoding/json"
	"errors"
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

	namespace              = "bearerStd"
	bearerPrefix           = "Bearer "
	HTTPstatusCodeOK       = http.StatusOK
	HTTPstatusNoContent    = http.StatusNoContent
	HTTPstatusCodeErr      = http.StatusBadRequest
	HTTPstatusInternalErr  = http.StatusInternalServerError
	HTTPstatusCodeNotFound = 404
)

// API is a namespace handler for the httpRouter with Bearer authorization
type API struct {
	router         *httprouter.HTTProuter
	basePath       string
	authTokens     sync.Map
	adminToken     string
	adminTokenLock sync.RWMutex
	verboseAuthLog bool
}

// APIdata is the data type used by the API.
// On handler functions Message.Data can be cast safely to this type.
type APIdata struct {
	Data      []byte
	AuthToken string
}

// APIhandler is the handler function used by the bearer std API httprouter implementation
type APIhandler = func(*APIdata, *httprouter.HTTPContext) error

// ErrorMsg is the error returned by bearer std API
type ErrorMsg struct {
	Error string `json:"error"`
}

// NewAPI returns a API initialized type
func NewAPI(router *httprouter.HTTProuter, baseRoute string) (*API, error) {
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
	bsa := API{router: router, basePath: baseRoute}
	router.AddNamespace(namespace, &bsa)
	return &bsa, nil
}

// AuthorizeRequest is a function for the RouterNamespace interface.
// On private handlers checks if the supplied bearer token have still request credits
func (a *API) AuthorizeRequest(data interface{},
	accessType httprouter.AuthAccessType) (bool, error) {
	msg, ok := data.(*APIdata)
	if !ok {
		panic("type is not bearerStandardApi")
	}
	switch accessType {
	case httprouter.AccessTypeAdmin:
		a.adminTokenLock.RLock()
		defer a.adminTokenLock.RUnlock()
		if msg.AuthToken != a.adminToken {
			return false, fmt.Errorf("admin token not valid")
		}
		return true, nil
	case httprouter.AccessTypePrivate:
		_, ok = a.authTokens.Load(msg.AuthToken)
		if !ok {
			return false, fmt.Errorf("auth token not valid")
		}
		return true, nil
	case httprouter.AccessTypeQuota:
		remainingReqs, ok := a.authTokens.Load(msg.AuthToken)
		if !ok || remainingReqs.(int64) < 1 {
			return false, fmt.Errorf("no more requests available")
		}
		a.authTokens.Store(msg.AuthToken, remainingReqs.(int64)-1)
		return true, nil
	default:
		return true, nil
	}
}

// ProcessData is a function for the RouterNamespace interface.
// The body of the http requests and the bearer auth token are readed.
func (a *API) ProcessData(req *http.Request) (interface{}, error) {
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("HTTP connection closed: (%v)", err)
	}
	if len(reqBody) > 0 {
		log.Debugf("request: %s", reqBody)
	}
	msg := &APIdata{
		Data:      reqBody,
		AuthToken: strings.TrimPrefix(req.Header.Get("Authorization"), bearerPrefix),
	}
	if a.verboseAuthLog && msg.AuthToken != "" {
		fmt.Printf("[BearerAPI/%d/%s] %s {%s}\n", time.Now().Unix(), msg.AuthToken, req.URL.RequestURI(), reqBody)
	}
	return msg, nil
}

// RegisterMethod adds a new method under the URL pattern.
// The pattern URL can contain variable names by using braces, such as /send/{name}/hello
// The pattern can also contain wildcard at the end of the path, such as /send/{name}/hello/*
// The accessType can be of type private, public or admin.
func (a *API) RegisterMethod(pattern, HTTPmethod string,
	accessType string, handler APIhandler) error {
	if pattern[0] != '/' {
		panic("pattern must start with /")
	}
	routerHandler := func(msg httprouter.Message) {
		bsaMsg := msg.Data.(*APIdata)
		if err := handler(bsaMsg, msg.Context); err != nil {
			// catch some specific errors to return the HTTP status code
			if errors.Is(err, httprouter.ErrNotFound) {
				if err := msg.Context.Send(nil, HTTPstatusCodeNotFound); err != nil {
					log.Warn(err)
				}
				return
			}
			if errors.Is(err, httprouter.ErrInternal) {
				if err := msg.Context.Send(nil, HTTPstatusInternalErr); err != nil {
					log.Warn(err)
				}
				return
			}
			// return 400 with an error message
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

	path := path.Join(a.basePath, pattern)
	switch accessType {
	case "public":
		a.router.AddPublicHandler(namespace, path, HTTPmethod, routerHandler)
	case "quota":
		a.router.AddQuotaHandler(namespace, path, HTTPmethod, routerHandler)
	case "private":
		a.router.AddPrivateHandler(namespace, path, HTTPmethod, routerHandler)
	case "admin":
		a.router.AddAdminHandler(namespace, path, HTTPmethod, routerHandler)
	default:
		return fmt.Errorf("method access type not implemented: %s", accessType)
	}
	return nil
}

// SetAdminToken sets the bearer admin token capable to execute admin handlers
func (a *API) SetAdminToken(bearerToken string) {
	a.adminTokenLock.Lock()
	defer a.adminTokenLock.Unlock()
	a.adminToken = bearerToken
}

// AddAuthToken adds a new bearer token capable to perform up to n requests
func (a *API) AddAuthToken(bearerToken string, requests int64) {
	a.authTokens.Store(bearerToken, requests)
}

// DelAuthToken removes a bearer token (will be not longer valid)
func (a *API) DelAuthToken(bearerToken string) {
	a.authTokens.Delete(bearerToken)
}

// GetAuthTokens returns the number of pending requests credits for a bearer token
func (a *API) GetAuthTokens(bearerToken string) int64 {
	ts, ok := a.authTokens.Load(bearerToken)
	if !ok {
		return 0
	}
	return ts.(int64)
}

// EnableVerboseAuthLog prints on stdout the details of every request performed with auth token.
// It can be used for keeping track of private/admin actions on a service.
func (a *API) EnableVerboseAuthLog() {
	a.verboseAuthLog = true
}
