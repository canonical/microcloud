package mdns

import (
	"fmt"
	"net"

	"github.com/hashicorp/mdns"
)

// ClusterService is the service name used for broadcasting willingness to join a cluster.
const ClusterService = "_microcloud"

// NewBroadcast returns a running mdns.Server which broadcasts the service at the given name and address.
func NewBroadcast(name string, iface *net.Interface, addr string, port int, service string, txt []byte) (*mdns.Server, error) {
	var sendTXT []string
	if txt != nil {
		sendTXT = dnsTXTSlice(txt)
	}

	config, err := mdns.NewMDNSService(name, service, "", "", port, []net.IP{net.ParseIP(addr)}, sendTXT)
	if err != nil {
		return nil, fmt.Errorf("Failed to create configuration for broadcast: %w", err)
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: config, Iface: iface})
	if err != nil {
		return nil, fmt.Errorf("Failed to begin broadcast: %w", err)
	}

	return server, nil
}

// dnsTXTSlice takes a []byte and returns a slice of 255-length strings suitable for a DNS TXT record.
func dnsTXTSlice(list []byte) []string {
	if len(list) <= 255 {
		return []string{string(list)}
	}

	parts := make([]string, 0, len(list)%255)
	parts = append(parts, string(list[:255]))
	parts = append(parts, dnsTXTSlice(list[255:])...)

	return parts
}

// checkIPStatus checks if the interface is up, has multicast, and supports IPv4/IPv6.
func checkIPStatus(iface string) (ipv4OK bool, ipv6OK bool, err error) {
	netInterface, err := net.InterfaceByName(iface)
	if err != nil {
		return false, false, err
	}

	if netInterface.Flags&net.FlagUp != net.FlagUp {
		return false, false, nil
	}

	if netInterface.Flags&net.FlagMulticast != net.FlagMulticast {
		return false, false, nil
	}

	addrs, err := netInterface.Addrs()
	if err != nil {
		return false, false, err
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		if ipNet.IP.To4() != nil {
			ipv4OK = true
		} else if ipNet.IP.To16() != nil {
			ipv6OK = true
		}
	}

	return ipv4OK, ipv6OK, nil
}
