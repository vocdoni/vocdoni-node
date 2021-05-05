package types

type DataStore struct {
	Datadir string
}

type Key struct {
	Idx int    `json:"idx"`
	Key string `json:"key"`
}
