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

	var s types.Submission
	var p types.Packet
	err := decoder.Decode(&s)

	if err != nil {
		panic(err)
	}

	//check PoW
	//check key
	//decrypt
	//check franchise
	//construct packet

	p.PID = 1
	p.Nullifier = []byte{1,2,3}
	p.Vote = []byte{4,5,6}
	p.Franchise = []byte{7,8,9}
	err = batch.Add(p)
	if err != nil {
		panic(err)
	}

	j, err := json.Marshal(s)
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
