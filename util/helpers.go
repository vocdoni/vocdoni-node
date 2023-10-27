package util

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"path"
	"strings"
)

func TrimHex(s string) string {
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		return s[2:]
	}
	return s
}

func RandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func Random32() [32]byte {
	var bytes [32]byte
	copy(bytes[:], RandomBytes(32))
	return bytes
}

func RandomHex(n int) string {
	return fmt.Sprintf("%x", RandomBytes(n))
}

func RandomInt(min, max int) int {
	num, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	if err != nil {
		panic(err)
	}
	return int(num.Int64()) + min
}

// BuildURL constructs a URL from parts. It handles any irregularities like missing or extra slashes.
func BuildURL(parts ...string) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("no parts provided")
	}

	// Parse the base URL (the first part).
	base, err := url.Parse(parts[0])
	if err != nil {
		return "", fmt.Errorf("could not parse base URL: %v", err)
	}

	// Create a path by joining all parts with a "/". This will also clean the path,
	otherParts := parts[1:]
	for i, part := range otherParts {
		cleanPart := path.Clean("/" + part)
		otherParts[i] = strings.TrimPrefix(cleanPart, "/") // Remove the leading slash added for cleaning.
	}

	// Now, construct the full path with the base path and the other parts.
	fullPath := path.Join(base.Path, path.Join(otherParts...))
	base.Path = fullPath
	return base.String(), nil
}
