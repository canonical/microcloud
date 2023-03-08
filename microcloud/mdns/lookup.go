package mdns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/lxc/lxd/shared/logger"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// ServerInfo is information about the server that is broadcast over mDNS.
type ServerInfo struct {
	Name        string
	Address     string
	Services    []types.ServiceType
	Fingerprint string
}

// forwardingWriter forwards the mdns log message to LXD's logger package.
type forwardingWriter struct {
	w io.Writer
}

func (f forwardingWriter) Write(p []byte) (int, error) {
	logMsg := string(p)

	if strings.Contains(logMsg, "[INFO]") {
		_, after, _ := strings.Cut(logMsg, "[INFO]")
		logger.Infof(after)
	} else if strings.Contains(logMsg, "[ERR]") {
		_, after, _ := strings.Cut(logMsg, "[ERR]")
		logger.Errorf(after)
	} else {
		return 0, fmt.Errorf("Invalid log %q", logMsg)
	}

	return len(logMsg), nil
}

// LookupPeers finds any broadcasting peers and returns a list of their names.
func LookupPeers(ctx context.Context, service string, localPeer string) (map[string]ServerInfo, error) {
	entries, err := Lookup(ctx, service, clusterSize)
	if err != nil {
		return nil, err
	}

	peers := map[string]ServerInfo{}
	for _, entry := range entries {
		if entry == nil {
			return nil, fmt.Errorf("Received empty record")
		}

		parts := strings.SplitN(entry.Name, fmt.Sprintf(".%s.local.", service), 2)
		peerName := parts[0]

		// Skip a response from ourselves.
		if localPeer == peerName {
			continue
		}

		if len(entry.InfoFields) == 0 {
			logger.Info("Received incomplete record, retrying in 5s...")
			time.Sleep(5 * time.Second)
			continue
		}

		unquoted, err := strconv.Unquote("\"" + strings.Join(entry.InfoFields, "") + "\"")
		if err != nil {
			return nil, fmt.Errorf("Failed to format DNS TXT record: %w", err)
		}

		info := ServerInfo{}
		err = json.Unmarshal([]byte(unquoted), &info)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse token map: %w", err)
		}

		peers[info.Name] = info
	}

	return peers, nil
}

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

	err := mdns.Lookup(service, entriesCh)
	if err != nil {
		return nil, fmt.Errorf("Failed lookup: %w", err)
	}

	close(entriesCh)

	return entries, nil
}
