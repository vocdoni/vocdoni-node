// Package data provides an abstraction layer for distributed data storage providers (currently only IPFS)
package data

import (
	"context"
	"io"
)

// Storage is the interface that wraps the basic methods for a distributed data storage provider.
type Storage interface {
	Init() error
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
