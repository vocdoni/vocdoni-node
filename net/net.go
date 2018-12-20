package net

import (
	"encoding/json"
	"fmt"
	"net/http"
	"io"
	"github.com/vocdoni/dvote-relay/batch"
	"github.com/vocdoni/dvote-relay/types"
)


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

func Listen(port string) {
	http.HandleFunc("/submit", parse)
	//add waitgroup
	func() {
		fmt.Println("serving on " + port)
		err := http.ListenAndServe(":" + port, nil)
		if err != nil {
			panic("ListenAndServe: " + err.Error())
		}
	}()
}
