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
// {
//   "request": {
//     "id":"randomID",
//     "method":"methodName",
//     "timestamp":"currentTS",
//     <customFields...>
//     },
//   "signature":"0x...",
//   "id":"sameID"}
// }
//
// The handler manages automatically signatures, ids and timestamps.
// The developer using SignedJRPC only needs to take care of the information within the request field (method and customFields).
// The handler is prepared to accept several methods within the same URL path, such as http://localhost/jsonapi, so there
// is no need to declare multiple URL paths (but its also allowed).
type SignedJRPC struct {
	signer    *ethereum.SignKeys
	reqType   func() MessageAPI
	methods   map[string]*registeredMethod
	adminAddr *ethcommon.Address
}

// SignedJRPCdata is the data type used by SignedJRPC
type SignedJRPCdata struct {
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
// A function returning an empty type implementing the custom MessageAPI should also be provided.
func NewSignedJRPC(signer *ethereum.SignKeys, reqType func() MessageAPI) *SignedJRPC {
	return &SignedJRPC{
		methods: make(map[string]*registeredMethod, 128),
		signer:  signer,
		reqType: reqType,
	}
}

// AuthorizeRequest implements the httprouter interface
func (s *SignedJRPC) AuthorizeRequest(data interface{}, isAdmin bool) bool {
	request, ok := data.(*SignedJRPCdata)
	if !ok {
		log.Warnf("type is not SignedJRPCdata")
		return false
	}
	// If admin method, compare with the admin address
	if isAdmin {
		if s.adminAddr == nil {
			log.Warnf("admin address not defined")
			return false
		}
		return *s.adminAddr == request.Address
	}
	// If private method, check authentication
	if request.Private {
		return s.signer.Authorized[request.Address]
	}
	// If the method is registered as public, return true
	return true
}

// ProcessData implements the httprouter interface
func (s *SignedJRPC) ProcessData(req *http.Request) (interface{}, error) {
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("HTTP connection closed: (%v)", err)
	}
	return s.getRequest(reqBody)
}

// SendError builds a response message error and sends it using the provided http context.
func (s *SignedJRPC) SendError(requestID string, errorMsg string, ctxt *httprouter.HTTPContext) error {
	if ctxt == nil {
		return fmt.Errorf("http context is nil")
	}
	msg := s.reqType()
	msg.SetError(errorMsg)
	data, err := BuildReply(s.signer, msg, requestID)
	if err != nil {
		return err
	}
	return ctxt.Send(data)
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
	log.Debugf("got request: %s", payload)
	if len(payload) < 22 { // 22 = min num characters json_tags+method+request
		return nil, fmt.Errorf("empty payload")
	}
	reqOuter := &RequestMessage{}
	if err := json.Unmarshal(payload, &reqOuter); err != nil {
		return nil, err
	}

	request := new(SignedJRPCdata)
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

	// Extract signature and address
	if !method.skipSignature {
		if len(reqOuter.Signature) != ethereum.SignatureLength {
			return request, fmt.Errorf("no signature provided or invalid length")
		}
		var sigBytes []byte
		if err := reqOuter.Signature.UnmarshalJSON(sigBytes); err != nil {
			return request, fmt.Errorf("cannot unmarshal signature: %w {%s}", err, reqOuter.Signature)
		}
		var err error
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
	}
	return request, nil
}
