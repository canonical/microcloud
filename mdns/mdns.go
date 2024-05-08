package mdns

import (
	"fmt"
	"net"

	"github.com/hashicorp/mdns"
)

// ClusterService is the service name used for broadcasting willingness to join a cluster.
const ClusterService = "_microcloud"

// ServiceSize is the maximum number of simultaneous broadcasts of the same mDNS service.
const ServiceSize = 10

// clusterSize is the maximum number of cluster members we can find.
const clusterSize = 1000

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
