package jsonrpcapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
)

// SignedJRPC is a httprouter handler to easily build a JSON rpc endpoint.
//
// The basic JSON structure for the RPC is predefined and follows the format:
//
//	{
//	  "request": {
//	    "id":"randomID",
//	    "method":"methodName",
//	    "timestamp":"currentTS",
//	    <customFields...>
//	    },
//	  "signature":"0x...",
//	  "id":"sameID"
//	}
//
// The handler manages automatically signatures, ids and timestamps.
// The developer using SignedJRPC only needs to take care of the information within the request field (method and customFields).
// The handler is prepared to accept several methods within the same URL path, such as http://localhost/jsonapi, so there
// is no need to declare multiple URL paths (but its also allowed).
type SignedJRPC struct {
	signer       *ethereum.SignKeys
	reqType      func() MessageAPI
	resType      func() MessageAPI
	methods      map[string]*registeredMethod
	adminAddr    *ethcommon.Address
	allowPrivate bool
}

// SignedJRPCdata is the data type used by SignedJRPC
type SignedJRPCdata struct {
	ID                 string
	Method             string
	Address            ethcommon.Address
	SignaturePublicKey []byte
	Private            bool
	Message            MessageAPI
}

type registeredMethod struct {
	public        bool
	skipSignature bool
}

// NewSignedJRPC returns an initialized instance of the SignedJRPC type.
// A signer is required in order to create signatures and authenticate clients.
// A function returning an empty type implementing the custom MessageAPI should be provided
// for request message and for response messages (might be the same).
func NewSignedJRPC(signer *ethereum.SignKeys, reqType func() MessageAPI, resType func() MessageAPI, allowPrivate bool) *SignedJRPC {
	return &SignedJRPC{
		methods:      make(map[string]*registeredMethod, 128),
		signer:       signer,
		reqType:      reqType,
		resType:      resType,
		allowPrivate: allowPrivate,
	}
}

// AllowPrivate if set to true, makes private requests available for authorized addresses.
// If no authorized addresses configured, it will allow any address to private methods
func (s *SignedJRPC) AllowPrivate(allow bool) {
	s.allowPrivate = allow
}

// AuthorizeRequest implements the httprouter interface
func (s *SignedJRPC) AuthorizeRequest(data interface{},
	accessType httprouter.AuthAccessType) (bool, error) {
	request, ok := data.(*SignedJRPCdata)
	if !ok {
		panic("type is not SignedJRPCdata")
	}
	// If admin method, compare with the admin address
	switch accessType {
	case httprouter.AccessTypeAdmin:
		if s.adminAddr == nil {
			return false, s.returnError("admin address not defined", request.ID)
		}
		if *s.adminAddr != request.Address {
			return false, s.returnError("admin address do not match", request.ID)
		}
		return true, nil
	case httprouter.AccessTypeQuota:
		panic("quota auth access not implemented for jsonRPC")
	case httprouter.AccessTypePrivate:
		// If private method, check authentication.
		// If no authorized addresses configured, allows any address.
		if s.allowPrivate && request.Private {
			if len(s.signer.Authorized) == 0 {
				return true, nil
			}
			if !s.signer.Authorized[request.Address] {
				return false, s.returnError("not authorized", request.ID)
			}
		}
		return true, nil
	default:
		return true, nil
	}
}

// ProcessData implements the httprouter interface
func (s *SignedJRPC) ProcessData(req *http.Request) (interface{}, error) {
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("HTTP connection closed: (%v)", err)
	}
	if len(reqBody) > 0 {
		log.Debugf("request: %s", reqBody)
	}
	procReq, err := s.getRequest(reqBody)
	if err != nil {
		return nil, s.returnError(err.Error(), procReq.ID)
	}
	return procReq, nil
}

// SendError builds a response message error and sends it using the provided http context.
func (s *SignedJRPC) SendError(requestID string, errorMsg string, ctxt *httprouter.HTTPContext) error {
	if ctxt == nil {
		return fmt.Errorf("http context is nil")
	}
	msg := s.resType()
	msg.SetError(errorMsg)
	data, err := BuildReply(s.signer, msg, requestID)
	if err != nil {
		return err
	}
	return ctxt.Send(data, 200)
}

func (s *SignedJRPC) returnError(errorMsg, requestID string) error {
	msg := s.resType()
	msg.SetError(errorMsg)
	data, err := BuildReply(s.signer, msg, requestID)
	if err != nil {
		return err
	}
	return fmt.Errorf("%s", data)
}

// AddAuthorizedAddress adds a new address to the list of authorized clients to perform private requests
func (s *SignedJRPC) AddAuthorizedAddress(addr ethcommon.Address) {
	s.signer.AddAuthKey(addr)
}

// SetAdminAddress sets the administrator address for admin private methods
func (s *SignedJRPC) SetAdminAddress(addr ethcommon.Address) {
	s.adminAddr = &addr
}

// RegisterMethod adds a new method to the SignedJRPC endpoint.
// If private the method won't try to authenticate the user.
// If skipSignature the method will not add or check any signature.
func (s *SignedJRPC) RegisterMethod(method string, private, skipSignature bool) error {
	if _, ok := s.methods[method]; ok {
		return fmt.Errorf("duplicate method %s", method)
	}
	s.methods[method] = &registeredMethod{public: !private, skipSignature: skipSignature}
	return nil
}

func (s *SignedJRPC) getRequest(payload []byte) (*SignedJRPCdata, error) {
	// First unmarshal the outer layer, to obtain the request ID, the signed
	// request, and the signature.
	request := new(SignedJRPCdata)
	log.Debugf("got request: %s", payload)
	if len(payload) < 22 { // 22 = min num characters json_tags+method+request
		return request, fmt.Errorf("empty payload")
	}
	reqOuter := &RequestMessage{}
	if err := json.Unmarshal(payload, &reqOuter); err != nil {
		return request, err
	}
	if reqOuter.ID == "" {
		return request, fmt.Errorf("missing request ID")
	}
	request.ID = reqOuter.ID

	// Second unmarshal the request message to obtain the method
	request.Message = s.reqType()
	if err := json.Unmarshal(reqOuter.MessageAPI, request.Message); err != nil {
		return request, err
	}
	request.Method = request.Message.GetMethod()
	if request.Method == "" {
		return request, fmt.Errorf("method is empty")
	}

	// The method must be registered
	method, ok := s.methods[request.Method]
	if !ok {
		return request, fmt.Errorf("method not valid: (%s)", request.Method)
	}
	request.Private = !method.public

	// If skipSignature true for the method, we are done
	if method.skipSignature {
		return request, nil
	}

	// Extract signature and address
	if len(reqOuter.Signature) != ethereum.SignatureLength {
		return request, fmt.Errorf("no signature provided or invalid length")
	}

	var err error
	if request.SignaturePublicKey, err = ethereum.PubKeyFromSignature(
		ethereum.BuildVocdoniMessage(reqOuter.MessageAPI),
		reqOuter.Signature); err != nil {
		return request, err
	}

	if len(request.SignaturePublicKey) != ethereum.PubKeyLengthBytes {
		return request, fmt.Errorf("could not extract public key from signature")
	}
	if request.Address, err = ethereum.AddrFromPublicKey(request.SignaturePublicKey); err != nil {
		return request, err
	}
	log.Debugf("recovered signer address: %s", request.Address.Hex())

	return request, nil
}
