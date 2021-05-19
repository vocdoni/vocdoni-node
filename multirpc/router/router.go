// Package router provides the routing and entry point for the go-dvote API
package router

import (
	"encoding/json"
	"fmt"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/multirpc/transports"
)

type RouterRequest struct {
	transports.MessageContext
	Message            transports.MessageAPI
	Method             string
	Id                 string
	Authenticated      bool
	Address            ethcommon.Address
	SignaturePublicKey []byte
	Private            bool
	Signer             *ethereum.SignKeys
}

type RequestMessage struct {
	MessageAPI json.RawMessage `json:"request"`

	ID        string   `json:"id"`
	Signature HexBytes `json:"signature"`
}

type ResponseMessage struct {
	MessageAPI json.RawMessage `json:"response"`

	ID        string   `json:"id"`
	Signature HexBytes `json:"signature"`
}

type registeredMethod struct {
	public        bool
	skipSignature bool
	handler       func(RouterRequest)
}

// Router holds a router object
type Router struct {
	Transports  map[string]transports.Transport
	messageType func() transports.MessageAPI
	methods     map[string]registeredMethod
	inbound     <-chan transports.Message
	signer      *ethereum.SignKeys
}

// NewRouter creates a router multiplexer instance
func NewRouter(inbound <-chan transports.Message, transports map[string]transports.Transport,
	signer *ethereum.SignKeys, messageTypeFunc func() transports.MessageAPI) *Router {
	r := new(Router)
	r.methods = make(map[string]registeredMethod)
	r.inbound = inbound
	r.Transports = transports
	r.signer = signer
	r.messageType = messageTypeFunc
	return r
}

// AddHandler adds a new function handler for serving a specific method identified by name.
func (r *Router) AddHandler(method, namespace string, handler func(RouterRequest), private, skipSignature bool) error {
	log.Debugf("adding new handler %s for namespace %s", method, namespace)
	if private {
		return r.registerPrivate(namespace, method, handler)
	}
	return r.registerPublic(namespace, method, handler, skipSignature)
}

// Route routes requests through the Router object
func (r *Router) Route() {
	if len(r.methods) == 0 {
		log.Warnf("router methods are not properly initialized")
		return
	}
	for {
		msg := <-r.inbound
		request, err := r.getRequest(msg.Namespace, msg.Data, msg.Context)
		if err != nil {
			go r.SendError(request, err.Error())
			continue
		}

		method := r.methods[msg.Namespace+request.Method]
		if !method.skipSignature && !request.Authenticated {
			go r.SendError(request, "invalid authentication")
			continue
		}
		log.Infof("calling api method %s/%s", msg.Namespace, request.Method)
		go method.handler(request)
	}
}

func (r *Router) getRequest(namespace string, payload []byte, context transports.MessageContext) (request RouterRequest, err error) {
	// In the case of errors, we need the context to reply too.
	request.MessageContext = context

	// First unmarshal the outer layer, to obtain the request ID, the signed
	// request, and the signature.
	log.Debugf("got request: %s", payload)
	reqOuter := &RequestMessage{}
	if len(payload) < 22 { // 22 = min num characters json_tags+method+request
		return request, fmt.Errorf("empty payload")
	}
	if err := json.Unmarshal(payload, &reqOuter); err != nil {
		return request, err
	}
	request.Id = reqOuter.ID
	request.Message = r.messageType()
	if err := json.Unmarshal(reqOuter.MessageAPI, request.Message); err != nil {
		return request, err
	}

	request.Method = request.Message.GetMethod()

	if request.Method == "" {
		return request, fmt.Errorf("method is empty")
	}

	method, ok := r.methods[namespace+request.Method]
	if !ok {
		return request, fmt.Errorf("method not valid: (%s)", request.Method)
	}

	if !method.skipSignature {
		if len(reqOuter.Signature) != ethereum.SignatureLength {
			return request, fmt.Errorf("no signature provided or invalid length")
		}
		var sigBytes []byte
		if err = reqOuter.Signature.UnmarshalJSON(sigBytes); err != nil {
			return request, fmt.Errorf("cannot unmarshal signature: %w {%s}", err, reqOuter.Signature)
		}

		if request.SignaturePublicKey, err = ethereum.PubKeyFromSignature(reqOuter.MessageAPI, reqOuter.Signature); err != nil {
			return request, err
		}

		if len(request.SignaturePublicKey) != ethereum.PubKeyLengthBytes {
			return request, fmt.Errorf("could not extract public key from signature")
		}
		if request.Address, err = ethereum.AddrFromPublicKey(request.SignaturePublicKey); err != nil {
			return request, err
		}
		log.Debugf("recovered signer address: %s", request.Address.Hex())
		request.Private = !method.public

		// If private method, check authentication
		if method.public {
			request.Authenticated = true
		} else {
			if r.signer.Authorized[request.Address] {
				request.Authenticated = true
			}
		}
	}

	// Add the signer for signing the reply
	request.Signer = r.signer
	return request, err
}

func (r *Router) registerPrivate(namespace, method string, handler func(RouterRequest)) error {
	if _, ok := r.methods[namespace+method]; ok {
		return fmt.Errorf("duplicate method %s for namespace %s", method, namespace)
	}
	r.methods[namespace+method] = registeredMethod{handler: handler}
	return nil
}

func (r *Router) registerPublic(namespace, method string, handler func(RouterRequest), skipSignature bool) error {
	if _, ok := r.methods[namespace+method]; ok {
		return fmt.Errorf("duplicate method %s for namespace %s", method, namespace)
	}
	r.methods[namespace+method] = registeredMethod{public: true, handler: handler, skipSignature: skipSignature}
	return nil
}

// SendError formats and sends an error message
func (r *Router) SendError(request RouterRequest, errMsg string) {
	if request.MessageContext == nil {
		log.Errorf("cannot reply with error: MessageContext==nil: %s", errMsg)
		return
	}
	var err error
	log.Warn(errMsg)

	// Add any last fields to the inner response, and marshal it with sorted
	// fields for signing.
	response := &ResponseMessage{}
	message := r.messageType()
	message.SetError(errMsg)

	// Only sign and add basic information if the request ID sent by the client
	// is valid.
	if len(request.Id) > 0 {
		response.ID = request.Id
		message.SetID(request.Id)
		message.SetTimestamp(int32(time.Now().Unix()))

		response.MessageAPI, err = crypto.SortedMarshalJSON(message)
		if err != nil {
			log.Error(err)
			return
		}

		// Sign the marshaled inner response.
		messagePayload, err := response.MessageAPI.MarshalJSON()
		if err != nil {
			log.Error(err)
			return
		}
		response.Signature, err = r.signer.Sign(messagePayload)
		if err != nil {
			log.Warnf("could not sign error message: %v", err)
			// continue without the signature
		}
	}

	data, err := json.Marshal(response)
	if err != nil {
		log.Warnf("error marshaling response body: %s", err)
	}
	msg := transports.Message{
		TimeStamp: int32(time.Now().Unix()),
		Context:   request.MessageContext,
		Data:      data,
	}
	if err := request.Send(msg); err != nil {
		log.Warn(err)
	}
}

// AddAuthKey adds a new pubkey address that will have access to private methods
func (r *Router) AddAuthKey(addr ethcommon.Address) {
	r.signer.AddAuthKey(addr)
}

// DelAuthKey deletes a pubkey address from the authorized list
func (r *Router) DelAuthKey(addr ethcommon.Address) {
	delete(r.signer.Authorized, addr)
}

// BuildReply builds a response message (set ID, Timestamp and Signature)
func BuildReply(response transports.MessageAPI, request RouterRequest) transports.Message {
	var err error
	respRequest := &ResponseMessage{ID: request.Id}

	response.SetID(request.Id)
	response.SetTimestamp(int32(time.Now().Unix()))
	respRequest.MessageAPI, err = crypto.SortedMarshalJSON(response)
	if err != nil {
		// This should never happen. If it does, return a very simple
		// plaintext error, and log the error.
		log.Error(err)
		return transports.Message{
			TimeStamp: int32(time.Now().Unix()),
			Context:   request.MessageContext,
			Data:      []byte(err.Error()),
		}
	}

	// Sign the marshaled inner response.
	respRequest.Signature, err = request.Signer.Sign(respRequest.MessageAPI)
	if err != nil {
		log.Error(err)
		// continue without the signature
	}

	// We don't need to use crypto.SortedMarshalJSON here, since we don't sign these bytes.
	respData, err := json.Marshal(respRequest)
	if err != nil {
		// This should never happen. If it does, return a very simple
		// plaintext error, and log the error.
		log.Error(err)
		return transports.Message{
			TimeStamp: int32(time.Now().Unix()),
			Context:   request.MessageContext,
			Data:      []byte(err.Error()),
		}
	}
	log.Debugf("response: %s", respData)
	return transports.Message{
		TimeStamp: int32(time.Now().Unix()),
		Context:   request.MessageContext,
		Data:      respData,
	}
}
