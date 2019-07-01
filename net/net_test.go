package net

import (
	"testing"
	"encoding/json"
	"net/http"
	"time"
	"bytes"
	"io/ioutil"

	"github.com/vocdoni/go-dvote/log"
)

//func generateSubmission() submission {

//}

func TestListen(t *testing.T) {
	t.Log("Testing listener")

	testSubmission := submission {
		"package",
		[]byte("012345678"),
		[]byte("012345678"),
		[]byte("012345678"),
		time.Now(),
	}

	listen("8080")

	url := "http://localhost:8080/submit"
	log.Infof("URL:>", url)

	j, err := json.Marshal(testSubmission)
	if err != nil {
		t.Errorf("Bad test JSON: %s", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("Error in client: %s", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	log.Infof("Response: Status: %v, Headers: %v, Body: %v", resp.Status, resp.Header, body)
}
