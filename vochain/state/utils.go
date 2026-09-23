package state

import (
	"bytes"
	"fmt"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

// GenerateNullifier generates the nullifier of a vote (hash(address+processId))
// This function assumes address and processID are correct.
func GenerateNullifier(address ethcommon.Address, processID []byte) []byte {
	nullifier := bytes.Buffer{}
	nullifier.Write(address.Bytes())
	nullifier.Write(processID)
	return ethereum.HashRaw(nullifier.Bytes())
}

// GetFriendlyResults returns the results of a process in a human friendly format.
func GetFriendlyResults(results []*models.QuestionResult) [][]*types.BigInt {
	r := [][]*types.BigInt{}
	for i := range results {
		r = append(r, []*types.BigInt{})
		for j := range results[i].Question {
			r[i] = append(r[i], new(types.BigInt).SetBytes(results[i].Question[j]))
		}
	}
	return r
}

// CheckDuplicateDelegates checks if the given delegates are not duplicated
// if addr is not nill will check if the addr is present in the delegates
func CheckDuplicateDelegates(delegates [][]byte, addr *ethcommon.Address) error {
	delegatesToSetMap := make(map[ethcommon.Address]bool)
	for _, delegate := range delegates {
		delegateAddress := ethcommon.BytesToAddress(delegate)
		if addr != nil {
			if delegateAddress == *addr {
				return fmt.Errorf("delegate cannot be the same as the sender")
			}
		}
		if _, ok := delegatesToSetMap[delegateAddress]; !ok {
			delegatesToSetMap[delegateAddress] = true
			continue
		}
		return fmt.Errorf("duplicate delegate address %s", delegateAddress)
	}
	return nil
}

func printPrettierDelegates(delegates [][]byte) []string {
	prettierDelegates := make([]string, 0, len(delegates))
	for _, delegate := range delegates {
		prettierDelegates = append(prettierDelegates, ethcommon.BytesToAddress(delegate).String())
	}
	return prettierDelegates
}

// toPrefixKey helper function returns the encoded key of the provided one
// prefixing it with the prefix provided.
func toPrefixKey(prefix, key []byte) []byte {
	return append(prefix, key...)
}

// fromPrefixKey helper function returns the original key of the provided
// encoded one stripping the prefix provided.
func fromPrefixKey(prefix, key []byte) []byte {
	return key[len(prefix):]
}
