package mdns

import (
	"context"
	"crypto/x509"
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
	Version     string
	Name        string
	Address     string
	Services    []types.ServiceType
	Certificate *x509.Certificate
}

// NetworkInfo represents information about a network interface broadcast by a MicroCloud peer.
type NetworkInfo struct {
	Interface net.Interface
	Address   string
	Subnet    *net.IPNet
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

// LookupPeer finds any broadcasting peer and returns its info.
func LookupPeer(ctx context.Context, iface *net.Interface, version string) (*ServerInfo, error) {
	entry, err := Lookup(ctx, iface, ClusterService)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, fmt.Errorf("Received empty record")
	}

	if !strings.HasSuffix(entry.Name, ".local.") {
		return nil, fmt.Errorf("Invalid service name %q", entry.Name)
	}

	serviceStr := strings.TrimSuffix(entry.Name, ".local.")
	parts := strings.SplitN(serviceStr, ClusterService, 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid service name %q", entry.Name)
	}

	peerName := parts[0]
	if len(entry.InfoFields) == 0 {
		return nil, fmt.Errorf("Received incomplete record from peer %q", peerName)
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

	// Skip any responses from mismatched versions.
	if info.Version != version {
		return nil, fmt.Errorf("System %q (version %q) has a version mismatch. Expected %q", peerName, info.Version, version)
	}

	return &info, nil
}

// Lookup searches for the given service name over mdns.
func Lookup(ctx context.Context, iface *net.Interface, service string) (*mdns.ServiceEntry, error) {
	log.SetOutput(forwardingWriter{})

	entriesCh := make(chan *mdns.ServiceEntry, 1)
	defer close(entriesCh)

	params := mdns.DefaultParams(service)
	params.Interface = iface
	params.Entries = entriesCh
	params.Timeout = 100 * time.Millisecond
	ipv4Supported, ipv6Supported, err := checkIPStatus(iface.Name)
	if err != nil {
		return nil, fmt.Errorf("Failed to check IP status: %w", err)
	}

	if !ipv4Supported {
		logger.Info("IPv4 is not supported on this system network interface: disabling IPv4 mDNS", logger.Ctx{"iface": iface.Name})
		params.DisableIPv4 = true
	}

	if !ipv6Supported {
		logger.Info("IPv6 is not supported on this system network interface: disabling IPv6 mDNS", logger.Ctx{"iface": iface.Name})
		params.DisableIPv6 = true
	}

	if params.DisableIPv4 && params.DisableIPv6 {
		return nil, fmt.Errorf("No supported IP versions on the network interface %q", iface.Name)
	}

	// Return the first peer that gets found.
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("Failed lookup: %w", ctx.Err())
		case entry := <-entriesCh:
			return entry, nil
		default:
			err = mdns.Query(params)
			if err != nil {
				return nil, fmt.Errorf("Failed lookup: %w", err)
			}
		}
	}
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
