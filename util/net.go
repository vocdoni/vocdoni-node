package util

import (
	"net"
	"time"

	externalip "github.com/glendc/go-external-ip"
)

// By default, externalip uses a 30s timeout, which is far too long.
// If one of the "external IP" provider hosts isn't responding,
// we don't want to hang around for that long.
// Three seconds seems plenty, given that network roundtrips between the
// majority of servers should be in the tens to hundreds of milliseconds.
const publicIPTimeout = 3 * time.Second

// PublicIP returns the external/public IP (v4 or v6) of the host.
// ipversion can be 4 to require ipv4, 6 to require ipv6, or 0 for either.
//
// If a nil error is returned, the returned IP must be valid.
func PublicIP(ipversion uint) (net.IP, error) {
	config := externalip.DefaultConsensusConfig().WithTimeout(publicIPTimeout)

	consensus := externalip.DefaultConsensus(config, nil)
	consensus.UseIPProtocol(ipversion)
	return consensus.ExternalIP()
}
