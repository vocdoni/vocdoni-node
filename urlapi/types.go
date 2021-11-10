package urlapi

import (
	"time"

	"go.vocdoni.io/dvote/types"
)

type ErrorMsg struct {
	Error string `json:"error"`
}

type EntitiesMsg struct {
	EntityID  types.HexBytes    `json:"entityID"`
	Processes []*ProcessSummary `json:"processes,omitempty"`
}

type ProcessSummary struct {
	ProcessID types.HexBytes `json:"processId,omitempty"`
	Status    string         `json:"status,omitempty"`
	StartDate time.Time      `json:"startDate,omitempty"`
	EndDate   time.Time      `json:"endDate,omitempty"`
}

type Process struct {
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Header      string     `json:"header"`
	StreamURI   string     `json:"streamUri"`
	Status      string     `json:"status"`
	VoteCount   uint64     `json:"voteCount"`
	Questions   []Question `json:"questions"`
	Results     []Result   `json:"result"`
}

type Result struct {
	Title []string `json:"title"`
	Value []string `json:"value"`
}

type Question struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Choices     []string `json:"choices"`
}
