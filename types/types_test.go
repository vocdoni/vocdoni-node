package types

import (
	"testing"

	"go.vocdoni.io/dvote/api"
)

func TestMetaRequestString(t *testing.T) {
	tests := []struct {
		in   api.MetaRequest
		want string
	}{
		{api.MetaRequest{}, ":{}"},
		{api.MetaRequest{Method: "test1", Type: "foo"}, "test1:{Type:foo}"},
		{
			api.MetaRequest{Method: "test2", Type: "foo", Payload: []byte("\x01\xFF")},
			"test2:{Payload:01ff Type:foo}",
		},
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
