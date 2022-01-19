package jsonrpcapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
)

func TestJSONRPCAPI(t *testing.T) {
	r := httprouter.HTTProuter{}
	rng := testutil.NewRandom(123)
	port := 23000 + rng.RandomIntn(1024)
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	err := r.Init("127.0.0.1", port)
	qt.Check(t, err, qt.IsNil)

	// Create a standard API handler
	signer := ethereum.NewSignKeys()
	signer.Generate()
	rpcAPI := NewSignedJRPC(signer, NewAPI, NewAPI, true)
	rpcAPI.AddAuthorizedAddress(signer.Address())

	// Add it under namespace "rpc"
	r.AddNamespace("rpc", rpcAPI)

	rpcAPI.RegisterMethod("add", false, false)
	qt.Check(t, err, qt.IsNil)
	rpcAPI.RegisterMethod("del", true, false)
	qt.Check(t, err, qt.IsNil)

	// Add a private handler to serve requests on rpc namespace
	r.AddPrivateHandler("rpc", "/names", "POST", func(msg httprouter.Message) {
		req := msg.Data.(*SignedJRPCdata)
		apiMsg := req.Message.(*API)
		resp := API{}
		switch req.Method {
		case "add":
			if apiMsg.Name == "" || apiMsg.Age == 0 {
				rpcAPI.SendError(req.ID, "name or age missing", msg.Context)
				break
			}
			resp.Ok = true
			data, err := BuildReply(signer, &resp, req.ID)
			qt.Check(t, err, qt.IsNil)
			err = msg.Context.Send(data, 200)
			qt.Check(t, err, qt.IsNil)
		case "del":
			if apiMsg.Name == "" {
				rpcAPI.SendError(req.ID, "name missing", msg.Context)
				break
			}
			resp.Ok = true
			data, err := BuildReply(signer, &resp, req.ID)
			qt.Check(t, err, qt.IsNil)
			msg.Context.Send(data, 200)
			qt.Check(t, err, qt.IsNil)
		default:
			t.Fatal("method invalid")
		}
	})

	// Crete a client signer
	clientSigner := ethereum.SignKeys{}
	clientSigner.Generate()

	// Test add method
	resp := doRequest(t, url+"/names", "POST", &clientSigner, &API{Method: "add", ID: "abc", Age: 21, Name: "John"})
	qt.Check(t, string(resp), qt.Contains, `"ok":true`)

	// Test an error
	resp = doRequest(t, url+"/names", "POST", &clientSigner, &API{Method: "add", ID: "abc", Name: "John"})
	qt.Check(t, string(resp), qt.Contains, `age missing`)

	// Test a delete (authorized), should fail
	resp = doRequest(t, url+"/names", "POST", &clientSigner, &API{Method: "del", ID: "bcd", Name: "John"})
	qt.Check(t, string(resp), qt.Contains, "not authorized")

	// Add the client address key and test again (should work)
	rpcAPI.AddAuthorizedAddress(clientSigner.Address())
	resp = doRequest(t, url+"/names", "POST", &clientSigner, &API{Method: "del", ID: "feg", Name: "John"})
	qt.Check(t, string(resp), qt.Contains, `"ok":true`)
}

func NewAPI() MessageAPI {
	return &API{}
}

type API struct {
	Method    string `json:"method,omitempty"`
	Name      string `json:"name,omitempty"`
	Age       int    `json:"age,omitempty"`
	ID        string `json:"id"`
	Ok        bool   `json:"ok,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp int32  `json:"timestamp"`
}

func (a *API) GetMethod() string {
	return a.Method
}

func (a *API) SetID(id string) {
	a.ID = id
}

func (a *API) SetError(errorMsg string) {
	a.Error = errorMsg
}

func (a *API) SetTimestamp(ts int32) {
	a.Timestamp = ts
}

func doRequest(t *testing.T, url, method string, signer *ethereum.SignKeys, msg *API) []byte {
	data, err := json.Marshal(msg)
	qt.Check(t, err, qt.IsNil)
	signature := []byte{}
	if signer != nil {
		signature, err = signer.SignEthereum(data)
		qt.Check(t, err, qt.IsNil)
	}
	msgReq := RequestMessage{
		ID:         msg.ID,
		Signature:  signature,
		MessageAPI: data,
	}
	body, err := json.Marshal(msgReq)
	qt.Check(t, err, qt.IsNil)

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	qt.Check(t, err, qt.IsNil)
	resp, err := http.DefaultClient.Do(req)
	qt.Check(t, err, qt.IsNil)
	respBody, err := io.ReadAll(resp.Body)
	qt.Check(t, err, qt.IsNil)
	return respBody
}
