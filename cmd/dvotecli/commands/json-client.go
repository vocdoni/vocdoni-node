package commands

import (
	"bufio"
	"encoding/json"
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
	"go.vocdoni.io/dvote/types"
	"nhooyr.io/websocket"
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
	if privKey != "" {
		if err := signer.AddHexKey(privKey); err != nil {
			log.Fatal(err)
		}
	} else {
		signer.Generate()
	}
	log.Infof("connecting to %s", host)
	cl, err := client.New(host)
	if err != nil {
		log.Fatal(err)
	}
	defer cl.Conn.Close(websocket.StatusNormalClosure, "")
	var req types.MetaRequest
	reader := bufio.NewReader(os.Stdin)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
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
	return nil
}

func processLine(input []byte) types.MetaRequest {
	var req types.MetaRequest
	err := json.Unmarshal(input, &req)
	if err != nil {
		log.Fatal(err)
	}
	return req
}

func printNice(resp *types.MetaResponse) {
	v := reflect.ValueOf(*resp)
	typeOfS := v.Type()
	output := "\n"
	var val reflect.Value
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Type().Name() == "bool" || v.Field(i).Type().Name() == "int64" || !v.Field(i).IsZero() {
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
