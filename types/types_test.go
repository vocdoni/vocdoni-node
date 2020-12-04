package types

import "testing"

func TestMetaRequestString(t *testing.T) {
	tests := []struct {
		in   MetaRequest
		want string
	}{
		{MetaRequest{}, "{}"},
		{MetaRequest{Type: "foo"}, "{Type:foo}"},
		{MetaRequest{Type: "foo", Payload: []byte("\x01\xFF")}, "{Payload:01FF Type:foo}"},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			got := test.in.String()
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
