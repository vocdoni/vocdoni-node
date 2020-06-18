package crypto

import "encoding/json"

// SortedMarshalJSON is similar to json.Marshal, but it also enforces all struct
// fields to be sorted when marshaling, just like json.Marshal does for maps.
//
// Note that this function has an added cost. It should only matter when all
// keys need to be sorted at all times.
func SortedMarshalJSON(v1 interface{}) ([]byte, error) {
	// First, marshal the data, where the keys might not be sorted if v1
	// contains a struct.
	unsorted, err := json.Marshal(v1)
	if err != nil {
		return nil, err
	}

	// Then, unmarshal back into an empty interface, which will turn the
	// structs into map[string]interface{}.
	var v2 interface{}
	if err := json.Unmarshal(unsorted, &v2); err != nil {
		return nil, err
	}

	// Marshal again; the json package will sort keys in maps, so the output
	// will have all keys sorted.
	return json.Marshal(v2)
}
