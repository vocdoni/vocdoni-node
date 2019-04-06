package data

import (
	"github.com/vocdoni/go-dvote/types"
)

type BZZHandle struct {
	s *types.Store
}

func (b *BZZHandle) Init() {
	return
}

func (b *BZZHandle) Publish(object []byte) string {
	//publish, return hash
	//return cid
	return ""
}

func (b *BZZHandle) Retrieve(hash string) []byte {
	//fetch content by cid
	//return content
	var dummy []byte
	return dummy
}
