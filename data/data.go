// Package data provides an abstraction layer for distributed data storage providers (currently only IPFS)
package data

import (
	"context"
	"fmt"
	"io"

	"go.vocdoni.io/dvote/types"
)

// Storage is the interface that wraps the basic methods for a distributed data storage provider.
type Storage interface {
	Init(d *types.DataStore) error
	Publish(ctx context.Context, data []byte) (string, error)
	PublishReader(ctx context.Context, data io.Reader) (string, error)
	Retrieve(ctx context.Context, id string, maxSize int64) ([]byte, error)
	RetrieveDir(ctx context.Context, id string, maxSize int64) (map[string][]byte, error)
	Pin(ctx context.Context, path string) error
	Unpin(ctx context.Context, path string) error
	ListPins(ctx context.Context) (map[string]string, error)
	URIprefix() string
	Stats() map[string]any
	Stop() error
}

var (
	// ErrInvalidPath is returned when the path provided is not valid.
	ErrInvalidPath = fmt.Errorf("invalid storage path")
	// ErrUnavailable is returned when the storage path is not available.
	ErrUnavailable = fmt.Errorf("storage path is unavailable")
	// ErrNotFound is returned when the file is not found (cannot be fetch).
	ErrNotFound = fmt.Errorf("storage file not found")
	// ErrTimeout is returned when the storage context times out.
	ErrTimeout = fmt.Errorf("storage context timeout")
)
