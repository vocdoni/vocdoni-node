package types

import "testing"

func TestMetaRequestString(t *testing.T) {
	tests := []struct {
		in   MetaRequest
		want string
	}{
		{MetaRequest{}, ":{}"},
		{MetaRequest{Method: "test1", Type: "foo"}, "test1:{Type:foo}"},
		{MetaRequest{Method: "test2", Type: "foo", Payload: []byte("\x01\xFF")}, "test2:{Payload:01ff Type:foo}"},
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
