package net

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/vocdoni/go-dvote/batch"
	"github.com/vocdoni/go-dvote/types"
)

type HTTPHandle struct {
	port string
	path string
}

func (h *HTTPHandle) Init(c string) error {
	//split c to port and path
	cs := strings.Split(c, "/")
	h.port = cs[0]
	h.path = cs[1]
	return nil

}

//this should become submitVote handler
//move initial logic to core router
func parse(rw http.ResponseWriter, request *http.Request) {
	decoder := json.NewDecoder(request.Body)

	var e types.Envelope
	var b types.Ballot

	err := decoder.Decode(&e)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(e.Ballot, &b)
	if err != nil {
		panic(err)
	}

	//check PoW
	//check key
	//decrypt
	//check franchise
	//construct packet

	//this should should be randomized, or actually taken from input
	//b.PID = "1"
	//b.Nullifier = []byte{1,2,3}
	//b.Vote = []byte{4,5,6}
	//b.Franchise = []byte{7,8,9}

	err = batch.Add(b)
	if err != nil {
		panic(err)
	}

	j, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	io.WriteString(rw, string(j))
}

func (h *HTTPHandle) Listen() error {
	http.HandleFunc(h.path, parse)
	//add waitgroup
	func() {
		fmt.Println("serving on " + h.port + "/" + h.path)
		err := http.ListenAndServe(":"+h.port, nil)
		if err != nil {
			return
		}
	}()
	return nil
}
