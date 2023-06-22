package state

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func Test_encodeHysteresis(t *testing.T) {
	input := uint32(3498223)
	output := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 239, 96, 53, 0}
	qt.Assert(t, encodeHysteresis(input), qt.ContentEquals, output)

	input = 0
	output = make([]byte, hysteresisLen)
	qt.Assert(t, encodeHysteresis(input), qt.ContentEquals, output)

	input = 4294967294
	output = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 254, 255, 255, 255}
	qt.Assert(t, encodeHysteresis(input), qt.ContentEquals, output)
	
	input = 16777472
	output = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1}
	qt.Assert(t, encodeHysteresis(input), qt.ContentEquals, output)
}

func Test_decodeHysteresis(t *testing.T) {
	input := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 239, 96, 53, 0}
	output := uint32(3498223)

	qt.Assert(t, decodeHysteresis(input), qt.Equals, output)

	input = make([]byte, hysteresisLen)
	output = 0
	qt.Assert(t, decodeHysteresis(input), qt.Equals, output)

	input = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 254, 255, 255, 255}
	output = 4294967294
	qt.Assert(t, decodeHysteresis(input), qt.Equals, output)

	input = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1}
	output = 16777472
	qt.Assert(t, decodeHysteresis(input), qt.Equals, output)
}
