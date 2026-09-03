package api

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateOrganizationNameFilter(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty", "", false},
		{"100 ascii", strings.Repeat("a", 100), false},
		{"101 ascii", strings.Repeat("a", 101), true},
		// 100 multi-byte runes: >100 bytes but <=100 runes, must pass (the regression fixed here).
		{"100 multibyte runes", strings.Repeat("é", 100), false},
		{"101 multibyte runes", strings.Repeat("é", 101), true},
		{"non-printable null", "abc\x00def", true},
		{"non-printable newline", "abc\ndef", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOrganizationNameFilter(tc.input)
			if tc.wantErr {
				if !errors.Is(err, ErrParamNameInvalid) {
					t.Fatalf("got err %v, want ErrParamNameInvalid", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("got err %v, want nil", err)
			}
		})
	}
}
