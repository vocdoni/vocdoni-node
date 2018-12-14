package net

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"io"
	"github.com/vocdoni/dvote-relay/batch"
	"github.com/vocdoni/dvote-relay/types"
)

func parse(rw http.ResponseWriter, request *http.Request) {
	decoder := json.NewDecoder(request.Body)

	var s Submission
	err := decoder.Decode(&s)

	if err != nil {
		panic(err)
	}

	//check PoW
	//check key
	//decrypt
	//check franchise
	err = batch.add(p)
	if err != nil {
		return err
	}

	j, err := json.Marshal(s)
	io.WriteString(rw, string(j))
}

func listen(port string) {
	http.HandleFunc("/submit", parse)
	//add waitgroup
	go func() {
		fmt.Println("serving on " + port)
		err := http.ListenAndServe(":" + port, nil)
		if err != nil {
			panic("ListenAndServe: " + err.Error())
		}
	}()
}
