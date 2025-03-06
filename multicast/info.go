package multicast

import (
	"fmt"
	"net"
)

// NetworkInfo represents information about a network interface.
type NetworkInfo struct {
	Interface net.Interface
	Address   string
	Subnet    *net.IPNet
}

// GetNetworkInfo returns a slice of NetworkInfo.
func GetNetworkInfo() ([]NetworkInfo, error) {
	networks := []NetworkInfo{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("Failed to get network interfaces: %w", err)
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		if len(addrs) == 0 {
			continue
		}

		for _, addr := range addrs {
			ip, ipNet, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}

			if !ip.IsGlobalUnicast() {
				continue
			}

			networks = append(networks, NetworkInfo{Interface: iface, Address: ip.String(), Subnet: ipNet})
		}
	}

	return networks, nil
}
