package types

import "testing"

func TestHexBytes(t *testing.T) {
	input := HexBytes("hello world")

	encoded, err := input.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if want := `"68656c6c6f20776f726c64"`; string(encoded) != want {
		t.Fatalf("MarshalJSON got %q, exp %q", encoded, want)
	}

	var decoded HexBytes
	if err := decoded.UnmarshalJSON(encoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded) != string(input) {
		t.Fatalf("UnmarshalJSON got %q, exp %q", decoded, input)
	}
}
