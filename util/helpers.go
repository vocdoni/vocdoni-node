package util

import (
	"regexp"
	"strconv"
	"strings"
)

var validHexRegex = regexp.MustCompile("^([0-9a-fA-F])+$")

// IsHex checks if the given string contains only valid hex symbols
func IsHex(str string) bool { return validHexRegex.MatchString(str) }

// IsHexWithLength checks if the given string contains only valid hex symbols and have the desired length
func IsHexWithLength(str string, length int) bool {
	return len(str) == length && validHexRegex.MatchString(str)
}

func Hex2int64(hexStr string) int64 {
	// remove 0x suffix if found in the input string
	cleaned := strings.Replace(hexStr, "0x", "", -1)

	// base 16 for hexadecimal
	result, err := strconv.ParseUint(cleaned, 16, 64)
	if err != nil {
		panic(err)
	}
	return int64(result)
}

func TrimHex(hexStr string) string {
	return strings.TrimPrefix(hexStr, "0x")
}
