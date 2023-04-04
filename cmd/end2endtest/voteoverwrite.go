package main

import (
	"net/url"
	"os"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
)

func init() {
	ops = append(ops,
		operation{
			fn:          voteOverwriteTest,
			name:        "voteoverwrite",
			description: "Checks that the MaxVoteOverwrite feature is correctly implemented",
			example: os.Args[0] + " --operation=voteoverwrite # test against public dev API\n" +
				os.Args[0] + " --host http://127.0.0.1:9090/v2 --faucet=http://127.0.0.1:9090/v2/faucet/dev/" +
				" --operation=voteoverwrite # test against local testsuite",
		},
	)
}

func voteOverwriteTest(c config) {
	// Connect to the API host
	hostURL, err := url.Parse(c.host)
	if err != nil {
		log.Fatal(err)
	}
	log.Debugf("connecting to %s", hostURL.String())

	token := uuid.New()
	api, err := apiclient.NewHTTPclient(hostURL, &token)
	if err != nil {
		log.Fatal(err)
	}

	log.Warn("TBD")
	log.Warn(api)
}
