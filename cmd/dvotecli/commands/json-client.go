package commands

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	api "go.vocdoni.io/dvote/rpctypes"
)

var jsonClientCmd = &cobra.Command{
	Use:   "json-client",
	Short: "JSON command line client",
	RunE:  jsonInput,
}

func init() {
	rootCmd.AddCommand(jsonClientCmd)
	genesisGenCmd.Flags().String("loglevel", "error", "log level <debug, info, warn, error>")
}

func jsonInput(cmd *cobra.Command, args []string) error {
	logLevel, _ := cmd.Flags().GetString("loglevel")
	log.Init(logLevel, "stdout")
	rand.Seed(time.Now().UnixNano())

	signer := ethereum.NewSignKeys()
	if opt.privKey != "" {
		if err := signer.AddHexKey(opt.privKey); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := signer.Generate(); err != nil {
			log.Fatal(err)
		}
	}
	log.Infof("connecting to %s", opt.host)
	cl, err := client.New(opt.host)
	if err != nil {
		log.Fatal(err)
	}
	defer cl.CheckClose(&err)
	var req api.APIrequest
	reader := bufio.NewReader(os.Stdin)
	for {
		line, _, err := reader.ReadLine()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if len(line) < 7 || strings.HasPrefix(string(line), "#") {
			continue
		}
		req = processLine(line)

		resp, err := cl.Request(req, signer)
		if err != nil {
			log.Fatal(err)
		}
		printNice(resp)
	}
	return err
}

func processLine(input []byte) api.APIrequest {
	var req api.APIrequest
	err := json.Unmarshal(input, &req)
	if err != nil {
		log.Fatal(err)
	}
	return req
}

func printNice(resp *api.APIresponse) {
	v := reflect.ValueOf(*resp)
	typeOfS := v.Type()
	output := "\n"
	var val reflect.Value
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Type().Name() == "bool" || v.Field(i).Type().Name() ==
			"int64" || !v.Field(i).IsZero() {
			if v.Field(i).Kind() == reflect.Ptr {
				val = v.Field(i).Elem()
			} else {
				val = v.Field(i)
			}
			output += fmt.Sprintf("%v: %v\n", typeOfS.Field(i).Name, val)
		}
	}
	fmt.Print(output + "\n")
}
