package mdns

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/lxd/shared/logger"
	"github.com/hashicorp/mdns"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// ServerInfo is information about the server that is broadcast over mDNS.
type ServerInfo struct {
	Version    string
	Name       string
	Address    string
	Interface  string
	Services   []types.ServiceType
	AuthSecret string
}

// NetworkInfo represents information about a network interface broadcast by a MicroCloud peer.
type NetworkInfo struct {
	Interface net.Interface
	Address   string
	Subnet    *net.IPNet
}

// LookupKey returns a unique key representing a lookup entry.
func (s ServerInfo) LookupKey() string {
	return fmt.Sprintf("%s_%s_%s", s.Name, s.Interface, s.Address)
}

// forwardingWriter forwards the mdns log message to LXD's logger package.
type forwardingWriter struct{}

func (f forwardingWriter) Write(p []byte) (int, error) {
	logMsg := string(p)

	if strings.Contains(logMsg, "[INFO]") {
		_, after, _ := strings.Cut(logMsg, "[INFO]")
		logger.Info(after)
	} else if strings.Contains(logMsg, "[ERR]") {
		_, after, _ := strings.Cut(logMsg, "[ERR]")
		logger.Error(after)
	} else {
		return 0, fmt.Errorf("Invalid log %q", logMsg)
	}

	return len(logMsg), nil
}

// LookupPeers finds any broadcasting peers and returns a list of their names.
func LookupPeers(ctx context.Context, version string, localPeer string) (map[string]ServerInfo, error) {
	entries := []*mdns.ServiceEntry{}
	for i := 0; i < ServiceSize; i++ {
		nextEntries, err := Lookup(ctx, fmt.Sprintf("%s_%d", ClusterService, i), clusterSize)
		if err != nil {
			return nil, err
		}

		entries = append(entries, nextEntries...)
	}

	peers := map[string]ServerInfo{}
	for _, entry := range entries {
		if entry == nil {
			return nil, fmt.Errorf("Received empty record")
		}

		if !strings.HasSuffix(entry.Name, ".local.") {
			continue
		}

		serviceStr := strings.TrimSuffix(entry.Name, ".local.")
		parts := strings.SplitN(serviceStr, fmt.Sprintf(".%s_", ClusterService), 2)
		if len(parts) != 2 {
			continue
		}

		// Skip a response from ourselves.
		peerName := parts[0]
		if localPeer == peerName {
			continue
		}

		if len(entry.InfoFields) == 0 {
			logger.Infof("Received incomplete record from peer %q", peerName)
			continue
		}

		unquoted, err := strconv.Unquote("\"" + strings.Join(entry.InfoFields, "") + "\"")
		if err != nil {
			return nil, fmt.Errorf("Failed to format DNS TXT record: %w", err)
		}

		info := ServerInfo{}
		err = json.Unmarshal([]byte(unquoted), &info)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse server info: %w", err)
		}

		// Skip a response from ourselves.
		if localPeer == info.Name {
			continue
		}

		// Skip any responses from mismatched versions.
		if info.Version != version {
			logger.Infof("System %q (version %q) has a version mismatch. Expected %q", peerName, info.Version, version)
			continue
		}

		peers[info.LookupKey()] = info
	}

	return peers, nil
}

// Lookup searches for the given service name over mdns.
func Lookup(ctx context.Context, service string, size int) ([]*mdns.ServiceEntry, error) {
	log.SetOutput(forwardingWriter{})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	entriesCh := make(chan *mdns.ServiceEntry, size)
	entries := []*mdns.ServiceEntry{}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				for entry := range entriesCh {
					entries = append(entries, entry)
				}
			}
		}
	}()

	params := mdns.DefaultParams(service)
	params.Entries = entriesCh
	params.Timeout = 100 * time.Millisecond
	err := mdns.Query(params)
	if err != nil {
		return nil, fmt.Errorf("Failed lookup: %w", err)
	}

	close(entriesCh)

	return entries, nil
}

// GetNetworkInfo returns a slice of NetworkInfo to be included in the mDNS broadcast.
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
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			if !ipNet.IP.IsGlobalUnicast() {
				continue
			}

			networks = append(networks, NetworkInfo{Interface: iface, Address: ipNet.IP.String(), Subnet: ipNet})
		}
	}

	return networks, nil
}
