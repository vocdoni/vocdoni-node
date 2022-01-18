package util

import (
	"context"
	"math/rand"
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

var resolverList = []string{
	"8.8.8.8", "1.0.0.1", "9.9.9.9", "64.6.64.6", "176.103.130.130", "198.101.242.72",
}

// Resolve resolves a domain name using custom hardcoded public nameservers
// If domain cannot be solved, returns an empty string
// Only ipv4 support
func Resolve(host string) string {
	var ip string
	rl := StrShuffle(resolverList)
	for _, ns := range rl {
		ip = ResolveCustom(ns, host)
		if ip != "" {
			return ip
		}
	}
	return ""
}

// ResolveCustom resolves a domain name using provided nameserver
// If domain cannot be solved, returns an empty string
// Only ipv4 support
func ResolveCustom(nameserver string, host string) string {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", net.JoinHostPort(nameserver, "53"))
		},
	}
	ips, err := resolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		if isV4(ip.IP) {
			return ip.String()
		}
	}
	return ""
}

func isV4(ip net.IP) bool { return ip.To4() != nil }

// StrShuffle reandomizes the order of a string array
func StrShuffle(vals []string) []string {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ret := make([]string, len(vals))
	perm := r.Perm(len(vals))
	for i, randIndex := range perm {
		ret[i] = vals[randIndex]
	}
	return ret
}
