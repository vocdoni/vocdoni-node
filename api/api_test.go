package api

import (
	"testing"
)

func TestAPIrequestString(t *testing.T) {
	tests := []struct {
		in   APIrequest
		want string
	}{
		{APIrequest{}, ":{}"},
		{APIrequest{Method: "test1", Type: "foo"}, "test1:{Type:foo}"},
		{
			APIrequest{Method: "test2", Type: "foo", Payload: []byte("\x01\xFF")},
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
