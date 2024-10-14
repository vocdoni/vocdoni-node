package arbo

import (
	"encoding/json"
)

// CircomVerifierProof contains the needed data to check a Circom Verifier Proof
// inside a circom circuit.  CircomVerifierProof allow to verify through a
// zkSNARK proof the inclusion/exclusion of a leaf in a tree.
type CircomVerifierProof struct {
	Root     []byte   `json:"root"`
	Siblings [][]byte `json:"siblings"`
	OldKey   []byte   `json:"oldKey"`
	OldValue []byte   `json:"oldValue"`
	IsOld0   bool     `json:"isOld0"`
	Key      []byte   `json:"key"`
	Value    []byte   `json:"value"`
	Fnc      int      `json:"fnc"` // 0: inclusion, 1: non inclusion
}

// MarshalJSON implements the JSON marshaler
func (cvp CircomVerifierProof) MarshalJSON() ([]byte, error) {
	m := make(map[string]any)

	m["root"] = BytesLEToBigInt(cvp.Root).String()
	m["siblings"] = siblingsToStringArray(cvp.Siblings)
	m["oldKey"] = BytesLEToBigInt(cvp.OldKey).String()
	m["oldValue"] = BytesLEToBigInt(cvp.OldValue).String()
	if cvp.IsOld0 {
		m["isOld0"] = "1"
	} else {
		m["isOld0"] = "0"
	}
	m["key"] = BytesLEToBigInt(cvp.Key).String()
	m["value"] = BytesLEToBigInt(cvp.Value).String()
	m["fnc"] = cvp.Fnc

	return json.Marshal(m)
}

func siblingsToStringArray(s [][]byte) []string {
	var r []string
	for i := 0; i < len(s); i++ {
		r = append(r, BytesLEToBigInt(s[i]).String())
	}
	return r
}

// FillMissingEmptySiblings adds the empty values to the array of siblings for
// the Tree number of max levels
func (t *Tree) FillMissingEmptySiblings(s [][]byte) [][]byte {
	for i := len(s); i < t.maxLevels; i++ {
		s = append(s, emptyValue)
	}
	return s
}

// GenerateCircomVerifierProof generates a CircomVerifierProof for a given key
// in the Tree
func (t *Tree) GenerateCircomVerifierProof(k []byte) (*CircomVerifierProof, error) {
	kAux, v, siblings, existence, err := t.GenProof(k)
	if err != nil && err != ErrKeyNotFound {
		return nil, err
	}
	var cp CircomVerifierProof
	cp.Root, err = t.Root()
	if err != nil {
		return nil, err
	}
	s, err := UnpackSiblings(t.hashFunction, siblings)
	if err != nil {
		return nil, err
	}
	cp.Siblings = t.FillMissingEmptySiblings(s)
	if !existence {
		cp.OldKey = kAux
		cp.OldValue = v
	} else {
		cp.OldKey = emptyValue
		cp.OldValue = emptyValue
	}
	cp.Key = k
	cp.Value = v
	if existence {
		cp.Fnc = 0 // inclusion
	} else {
		cp.Fnc = 1 // non inclusion
	}

	return &cp, nil
}
