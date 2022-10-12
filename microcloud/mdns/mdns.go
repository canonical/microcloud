package mdns

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/hashicorp/mdns"
)

// TokenService is the service name used for broadcasting the list of tokens.
const TokenService = "_microcloud_token"

// ClusterService is the service name used for broadcasting willingness to join a cluster.
const ClusterService = "_microcloud"

// clusterSize is the maximum number of cluster members we can find.
const clusterSize = 30

func NewBroadcast(service string, name string, addr string, txt []byte) (*mdns.Server, error) {
	i, p, _ := strings.Cut(addr, ":")
	port, _ := strconv.Atoi(p)
	ip := net.ParseIP(i)

	var sendTXT []string
	if txt != nil {
		sendTXT = dnsTXTSlice(txt)
	}

	config, err := mdns.NewMDNSService(name, service, "", "", port, []net.IP{ip}, sendTXT)
	if err != nil {
		return nil, fmt.Errorf("Failed to create configuration for broadcast: %w", err)
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: config})
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
