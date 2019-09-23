package vochain

type QueryData struct {
	Method    string `json:"method"`
	ProcessID string `json:"processId"`
	Nullifier string `json:"nullifier"`
	From      int64  `json:"from"`
	ListSize  int64  `json:"listSize"`
	Timestamp int64  `json:"timestamp"`
}
