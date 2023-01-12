package rpcapi

import (
	"fmt"

	psload "github.com/shirou/gopsutil/load"
	psmem "github.com/shirou/gopsutil/mem"
	psnet "github.com/shirou/gopsutil/net"
	api "go.vocdoni.io/dvote/rpctypes"
)

const (
	healthMemMax   = 100
	healthLoadMax  = 10
	healthSocksMax = 10000
)

func (a *RPCAPI) info(request *api.APIrequest) (*api.APIresponse, error) {
	response := api.APIresponse{}
	response.APIList = a.APIs
	if a.vocapp != nil {
		response.ChainID = a.vocapp.ChainID()
	}
	if health, err := getHealth(); err == nil {
		response.Health = health
	} else {
		response.Health = -1
		return nil, fmt.Errorf("cannot get health status: (%s)", err)
	}
	return &response, nil
}

// Health is a number between 0 and 99 that represents the status of the node, as bigger the better
// The formula ued to calculate health is: 100* (1- ( Sum(weight[0..1] * value/value_max) ))
// Weight is a number between 0 and 1 used to give a specific weight to a value.
//
//	The sum of all weights used must be equal to 1 so 0.2*value1 + 0.8*value2 would give 20% of
//	weight to value1 and 80% of weight to value2
//
// Each value must be represented as a number between 0 and 1. To this aim the value might be
//
//	divided by its maximum value so if the mettered value is cpuLoad, a maximum must be defined
//	in order to give a normalized value between 0 and 1 i.e cpuLoad=2 and maxCpuLoad=10.
//	The value is: 2/10 (where cpuLoad<10) = 0.2
//
// The last operation includes the reverse of the values, so 1- (result).
//
//	And its *100 multiplication and trunking in order to provide a natural number between 0 and 99
func getHealth() (int32, error) {
	v, err := psmem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	memUsed := v.UsedPercent
	l, err := psload.Avg()
	if err != nil {
		return 0, err
	}
	load15 := l.Load15
	n, err := psnet.Connections("tcp")
	if err != nil {
		return 0, err
	}
	sockets := float64(len(n))

	// ensure maximums are not overflow
	if memUsed > healthMemMax {
		memUsed = healthMemMax
	}
	if load15 > healthLoadMax {
		load15 = healthLoadMax
	}
	if sockets > healthSocksMax {
		sockets = healthSocksMax
	}
	result := int32((1 - (0.33*(memUsed/healthMemMax) +
		0.33*(load15/healthLoadMax) +
		0.33*(sockets/healthSocksMax))) * 100)
	if result < 0 || result >= 100 {
		return 0, fmt.Errorf("expected health to be between 0 and 99: %d", result)
	}
	return result, nil
}
