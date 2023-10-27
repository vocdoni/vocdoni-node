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

	namespace         = "bearerStd"
	bearerPrefix      = "Bearer "
	maxRequestBodyLog = 1024 // maximum request body size to log
)

// HTTPstatus* equal http.Status*, simple sugar to avoid importing http everywhere
const (
	HTTPstatusOK                 = http.StatusOK
	HTTPstatusNoContent          = http.StatusNoContent
	HTTPstatusBadRequest         = http.StatusBadRequest
	HTTPstatusInternalErr        = http.StatusInternalServerError
	HTTPstatusNotFound           = http.StatusNotFound
	HTTPstatusServiceUnavailable = http.StatusServiceUnavailable
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

// APIerror is used by handler functions to wrap errors, assigning a unique error code
// and also specifying which HTTP Status should be used.
type APIerror struct {
	Err        error
	Code       int
	HTTPstatus int
}

// MarshalJSON returns a JSON containing Err.Error() and Code. Field HTTPstatus is ignored.
//
// Example output: {"error":"account not found","code":4003}
func (e APIerror) MarshalJSON() ([]byte, error) {
	// This anon struct is needed to actually include the error string,
	// since it wouldn't be marshaled otherwise. (json.Marshal doesn't call Err.Error())
	return json.Marshal(
		struct {
			Err  string `json:"error"`
			Code int    `json:"code"`
		}{
			Err:  e.Err.Error(),
			Code: e.Code,
		})
}

// Error returns the Message contained inside the APIerror
func (e APIerror) Error() string {
	return e.Err.Error()
}

// Send serializes a JSON msg using APIerror.Message and APIerror.Code
// and passes that to ctx.Send()
func (e APIerror) Send(ctx *httprouter.HTTPContext) error {
	msg, err := json.Marshal(e)
	if err != nil {
		log.Warn(err)
		return ctx.Send([]byte("marshal failed"), HTTPstatusInternalErr)
	}
	return ctx.Send(msg, e.HTTPstatus)
}

// Withf returns a copy of APIerror with the Sprintf formatted string appended at the end of e.Err
func (e APIerror) Withf(format string, args ...any) APIerror {
	return APIerror{
		Err:        fmt.Errorf("%w: %v", e.Err, fmt.Sprintf(format, args...)),
		Code:       e.Code,
		HTTPstatus: e.HTTPstatus,
	}
}

// With returns a copy of APIerror with the string appended at the end of e.Err
func (e APIerror) With(s string) APIerror {
	return APIerror{
		Err:        fmt.Errorf("%w: %v", e.Err, s),
		Code:       e.Code,
		HTTPstatus: e.HTTPstatus,
	}
}

// WithErr returns a copy of APIerror with err.Error() appended at the end of e.Err
func (e APIerror) WithErr(err error) APIerror {
	return APIerror{
		Err:        fmt.Errorf("%w: %v", e.Err, err.Error()),
		Code:       e.Code,
		HTTPstatus: e.HTTPstatus,
	}
}

// NewAPI returns an API initialized type
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
func (a *API) AuthorizeRequest(data any, accessType httprouter.AuthAccessType) (bool, error) {
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

// ProcessData processes the HTTP request and returns structured data.
// The body of the http requests and the bearer auth token are readed.
func (a *API) ProcessData(req *http.Request) (any, error) {
	// Read and handle the request body
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading request body: %v", err)
	}
	// Log the request body if it exists
	if len(reqBody) > 0 {
		displayReq := string(reqBody)
		if len(displayReq) > maxRequestBodyLog {
			displayReq = displayReq[:maxRequestBodyLog] + "..." // truncate for display
		}
		// This assumes you have a configured logging method. Replace with your logger instance.
		log.Debugf("request: %s", displayReq)
	}
	// Get and validate the Authorization header
	token := ""
	authHeader := req.Header.Get("Authorization")
	if authHeader != "" {
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			return nil, errors.New("authorization header is not a Bearer token")
		}
		// Extract the token
		token = strings.TrimPrefix(authHeader, bearerPrefix)
	}
	// Prepare the structured data
	msg := &APIdata{
		Data:      reqBody,
		AuthToken: token,
	}
	// If verbose logging is enabled, log the verbose authentication information
	if a.verboseAuthLog && msg.AuthToken != "" {
		fmt.Printf("[BearerAPI/%d/%s] %s {%s}\n", time.Now().Unix(), msg.AuthToken, req.URL.RequestURI(), reqBody)
	}
	return msg, nil
}

// RegisterMethod adds a new method under the URL pattern.
// The pattern URL can contain variable names by using braces, such as /send/{name}/hello
// The pattern can also contain wildcard at the end of the path, such as /send/{name}/hello/*
// The accessType can be of type private, public or admin.
func (a *API) RegisterMethod(pattern, HTTPmethod string, accessType string, handler APIhandler) error {
	if pattern[0] != '/' {
		panic("pattern must start with /")
	}
	routerHandler := func(msg httprouter.Message) {
		bsaMsg := msg.Data.(*APIdata)
		if err := handler(bsaMsg, msg.Context); err != nil {
			// err should be an APIError, use those properties
			if apierror, ok := err.(APIerror); ok {
				err := apierror.Send(msg.Context)
				if err != nil {
					log.Warnf("couldn't send apierror: %v", err)
				}
				return
			}
			// else, it's a plain error (this shouldn't happen)
			// send it in plaintext with HTTP Status 500 Internal Server Error
			if err := msg.Context.Send([]byte(err.Error()), HTTPstatusInternalErr); err != nil {
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
