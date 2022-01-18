package util

import (
	"errors"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	externalip "github.com/glendc/go-external-ip"
)

func TestPublicIP(t *testing.T) {
	t.Parallel()

	for _, version := range []uint{4, 6, 0} {
		t.Run(fmt.Sprintf("protocol-%d", version), func(t *testing.T) {
			t.Parallel()
			start := time.Now()

			ip, err := PublicIP(version)
			// Skip the checks if the host doesn't have IPv6
			if errors.Is(err, externalip.ErrNoIP) {
				t.Skip("no IP found; lacking internet access?")
			}
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, ip, qt.Not(qt.IsNil))
			switch version {
			case 4:
				qt.Assert(t, ip.To4(), qt.Not(qt.IsNil))
			case 6:
				qt.Assert(t, ip.To4(), qt.IsNil)
			}

			// If we took more than the expected timeout plus some margin,
			// something went wrong.
			elapsed := time.Since(start)
			qt.Assert(t, elapsed < publicIPTimeout+time.Second, qt.IsTrue)
		})
	}
}
