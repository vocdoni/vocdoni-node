package bearerstdapi

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

const bearerPrefix = "Bearer "

// BearerStandardAPI is a namespace handler for the httpRouter with Bearer authorization
type BearerStandardAPI struct {
	authTokens     sync.Map
	adminToken     string
	adminTokenLock sync.RWMutex
}

// BearerStandardAPIdata is the data type used by the BearerStandardAPI.
// On handler functions Message.Data can be cast safely to this type.
type BearerStandardAPIdata struct {
	Data      []byte
	AuthToken string
}

// NewBearerStandardAPI returns a BearerStandardAPI initialized type
func NewBearerStandardAPI() *BearerStandardAPI {
	return &BearerStandardAPI{}
}

// AuthorizeRequest is a function for the RouterNamespace interface.
// On private handlers checks if the supplied bearer token have still request credits
func (b *BearerStandardAPI) AuthorizeRequest(data interface{}, isAdmin bool) (bool, error) {

	msg, ok := data.(*BearerStandardAPIdata)
	if !ok {
		panic("type is not bearerStandardApi")
	}
	if isAdmin {
		b.adminTokenLock.RLock()
		defer b.adminTokenLock.RUnlock()
		if msg.AuthToken != b.adminToken {
			return false, fmt.Errorf("admin token not valid")
		}
		return true, nil
	}
	remainingReqs, ok := b.authTokens.Load(msg.AuthToken)
	if !ok || remainingReqs.(int64) < 1 {
		return false, fmt.Errorf("no more requests available")
	}
	b.authTokens.Store(msg.AuthToken, remainingReqs.(int64)-1)
	return true, nil
}

// ProcessData is a function for the RouterNamespace interface.
// The body of the http requests and the bearer auth token are readed.
func (b *BearerStandardAPI) ProcessData(req *http.Request) (interface{}, error) {
	respBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("HTTP connection closed: (%v)", err)
	}
	return &BearerStandardAPIdata{
		Data:      respBody,
		AuthToken: strings.TrimPrefix(req.Header.Get("Authorization"), bearerPrefix),
	}, nil
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
