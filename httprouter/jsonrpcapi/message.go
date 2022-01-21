package jsonrpcapi

import (
	"encoding/json"
	"time"

	"go.vocdoni.io/dvote/crypto"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

// MessageAPI inteface defines the methods that the SignedJRPC custom message type must contain.
type MessageAPI interface {
	SetID(string)
	SetTimestamp(int32)
	SetError(string)
	GetMethod() string
}

// RequestMessage is the envelope for the SignedJRPC message request
type RequestMessage struct {
	MessageAPI json.RawMessage `json:"request"`

	ID        string         `json:"id"`
	Signature types.HexBytes `json:"signature"`
}

// ResponseMessage is the envelope for the SignedJRPC message response
type ResponseMessage struct {
	MessageAPI json.RawMessage `json:"response"`

	ID        string         `json:"id"`
	Signature types.HexBytes `json:"signature"`
}

// BuildReply builds a response message (ID, Timestamp and Signature) and marshals it using JSON.
// The output message is ready to send as reply to the client using reqMsg.Context.Send(data)
func BuildReply(signer *ethereum.SignKeys, msg MessageAPI, requestID string) ([]byte, error) {
	var err error
	respRequest := &ResponseMessage{ID: requestID}
	msg.SetID(requestID)
	msg.SetTimestamp(int32(time.Now().Unix()))
	respRequest.MessageAPI, err = crypto.SortedMarshalJSON(msg)
	if err != nil {
		return nil, err
	}

	respRequest.Signature, err = signer.SignVocdoniMsg(respRequest.MessageAPI)
	if err != nil {
		log.Error(err)
		// continue without the signature
	}

	// We don't need to use crypto.SortedMarshalJSON here, since we don't sign these bytes.
	respData, err := json.Marshal(respRequest)
	if err != nil {
		return nil, err
	}
	log.Debugf("response: %s", respData)
	return respData, nil
}
