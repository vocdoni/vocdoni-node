package vochain

import (
	"fmt"
)

var (
	ErrInvalidAddress = fmt.Errorf("invalid address")
)

// PrefixDBCacheSize is the size of the cache for the MutableTree IAVL databases
var PrefixDBCacheSize = 0

// ________________________ QUERIES ________________________

// QueryData is an abstraction of any kind of data a query request could have
type QueryData struct {
	Method      string `json:"method"`
	ProcessID   string `json:"processId,omitempty"`
	Nullifier   string `json:"nullifier,omitempty"`
	From        int64  `json:"from,omitempty"`
	ListSize    int64  `json:"listSize,omitempty"`
	Timestamp   int64  `json:"timestamp,omitempty"`
	ProcessType string `json:"type,omitempty"`
}
