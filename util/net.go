package util

import (
	"context"
	"math/rand"
	"net"
	"strings"
	"time"

	externalip "github.com/glendc/go-external-ip"
)

func GetPublicIP() (net.IP, error) {
	consensus := externalip.DefaultConsensus(nil, nil)
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
	var resolver *net.Resolver
	resolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", net.JoinHostPort(nameserver, "53"))
		}}
	ips, err := resolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		if isV4(ip.String()) {
			return ip.String()
		}
	}
	return ""
}

func isV4(address string) bool {
	return strings.Count(address, ".") == 3
}

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
