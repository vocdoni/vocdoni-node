package types

// TODO: use an array, and possibly declare methods to encode/decode as hex.

type ProcessID = []byte

type EntityID = []byte

type CensusRoot = []byte

// TODO: consider using a database/sql interface instead?

type Nullifier = []byte

type Hash = []byte

type AccountID = HexBytes
