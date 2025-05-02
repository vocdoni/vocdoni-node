package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"strings"

	"go.vocdoni.io/dvote/util"
)

func main() {
	strInputList := flag.String("inputs", "", "inputs parts")
	strExpected := flag.String("raw", "", "expected raw")
	flag.Parse()

	if *strInputList == "" {
		log.Fatal("inputs is required")
	}
	inputList := strings.Split(*strInputList, ",")
	if len(inputList) != 2 {
		log.Fatal("inputs is required, and must be a comma separated list of two big ints")
	}
	if *strExpected == "" {
		log.Fatal("raw expected is required")
	}
	rawExpected, err := hex.DecodeString(*strExpected)
	if err != nil {
		log.Fatalf("error decoding vote package: %s", err)
	}
	expectedHash := sha256.Sum256(rawExpected)

	rebuildHash := util.SplittedArboStrToBytes(inputList[0], inputList[1], false, false)
	fmt.Println("")
	fmt.Println("Without swap endianess and lazy")
	fmt.Println("")
	fmt.Println("- Provided raw", hex.EncodeToString(rawExpected), len(rawExpected))
	fmt.Println("")
	fmt.Println("- Rebuild hash", hex.EncodeToString(rebuildHash), len(rebuildHash))
	fmt.Println("")
	fmt.Println("- Calculated hash", hex.EncodeToString(expectedHash[:]), len(expectedHash[:]))
	fmt.Println("")
	fmt.Println("- Match?", bytes.Equal(rebuildHash, expectedHash[:]))
	fmt.Println("")

	rebuildHash = util.SplittedArboStrToBytes(inputList[0], inputList[1], true, false)
	fmt.Println("With swap endianess and lazy")
	fmt.Println("")
	fmt.Println("- Provided raw", hex.EncodeToString(rawExpected), len(rawExpected))
	fmt.Println("")
	fmt.Println("- Rebuild hash", hex.EncodeToString(rebuildHash), len(rebuildHash))
	fmt.Println("")
	fmt.Println("- Calculated hash", hex.EncodeToString(expectedHash[:]), len(expectedHash[:]))
	fmt.Println("")
	fmt.Println("- Match?", bytes.Equal(rebuildHash, expectedHash[:]))

	rebuildHash = util.SplittedArboStrToBytes(inputList[0], inputList[1], false, true)
	fmt.Println("")
	fmt.Println("Without swap endianess and strict")
	fmt.Println("")
	fmt.Println("- Provided raw", hex.EncodeToString(rawExpected), len(rawExpected))
	fmt.Println("")
	fmt.Println("- Rebuild hash", hex.EncodeToString(rebuildHash), len(rebuildHash))
	fmt.Println("")
	fmt.Println("- Calculated hash", hex.EncodeToString(expectedHash[:]), len(expectedHash[:]))
	fmt.Println("")
	fmt.Println("- Match?", bytes.Equal(rebuildHash, expectedHash[:]))
	fmt.Println("")

	rebuildHash = util.SplittedArboStrToBytes(inputList[0], inputList[1], true, true)
	fmt.Println("With swap endianess and strict")
	fmt.Println("")
	fmt.Println("- Provided raw", hex.EncodeToString(rawExpected), len(rawExpected))
	fmt.Println("")
	fmt.Println("- Rebuild hash", hex.EncodeToString(rebuildHash), len(rebuildHash))
	fmt.Println("")
	fmt.Println("- Calculated hash", hex.EncodeToString(expectedHash[:]), len(expectedHash[:]))
	fmt.Println("")
	fmt.Println("- Match?", bytes.Equal(rebuildHash, expectedHash[:]))
}
