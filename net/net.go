package net

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"strconv"
	"io"
)

type submission struct {
	Type string
	Nonce []byte
	Key []byte
	Package []byte
	Expiry time.Time
}

func parseSubmission(rw http.ResponseWriter, request *http.Request) {
	decoder := json.NewDecoder(request.Body)

	var s submission
	err := decoder.Decode(&s)

	if err != nil {
		panic(err)
	}

	//check PoW
	//check discriminator
	//decrypt
	//check franchise
	//add to leveldb
	j, err := json.Marshal(s)
	//io.WriteString(rw, string(j))
}

func listen(port int) {
	http.HandleFunc("/submit", parseSubmission)
	portstr := strconv.Itoa(port)
	go func() {
		fmt.Println("serving on " + portstr)
		err := http.ListenAndServe(":" + portstr, nil)
		if err != nil {
			panic("ListenAndServe: " + err.Error())
		}
	}()
}
