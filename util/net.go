package util

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"

	externalip "gitlab.com/vocdoni/go-external-ip"
)

// GetPublicIP returns the external/public IP of the host
// For now, let's only support IPv4. Hope we can change this in the future...
//
// If a nil error is returned, the returned IP must be valid.
func GetPublicIP() (net.IP, error) {
	consensus := externalip.DefaultConsensus(nil, nil)
	ip, err := consensus.ExternalIP(4)
	// if the IP isn't a valid ipv4, To4 will return nil
	if ip = ip.To4(); ip == nil {
		return nil, fmt.Errorf("public IP discovery failed: %v", err)
	}
	return ip, nil
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
