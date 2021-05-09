package message

import "go.vocdoni.io/dvote/multirpc/transports"

// MyAPI is an example JSON API type compatible with transports.MessageAPI
type MyAPI struct {
	ID        string   `json:"request"`
	Method    string   `json:"method,omitempty"`
	PubKeys   []string `json:"pubKeys,omitempty"`
	Timestamp int32    `json:"timestamp"`
	Error     string   `json:"error,omitempty"`
	Reply     string   `json:"reply,omitempty"`
}

// GetID returns the ID for de request
func (ma *MyAPI) GetID() string {
	return ma.ID
}

// SetID sets a request ID
func (ma *MyAPI) SetID(id string) {
	ma.ID = id
}

// SetTimestamp sets the timestamp
func (ma *MyAPI) SetTimestamp(ts int32) {
	ma.Timestamp = ts
}

// SetError sets an error message
func (ma *MyAPI) SetError(e string) {
	ma.Error = e
}

// GetMethod returns the method
func (ma *MyAPI) GetMethod() string {
	return ma.Method
}

// newAPI is a required function for returning the implemented interface transports.MessageAPI
// This function is used by the router in order to fetch the specific type defined here.
func NewAPI() transports.MessageAPI {
	return &MyAPI{}
}
