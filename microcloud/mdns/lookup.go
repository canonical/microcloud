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
	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/logger"
)

// JoinConfig represents configuration broadcast to signal MicroCloud to begin join operations.
type JoinConfig struct {
	Token     string
	LXDConfig []api.ClusterMemberConfigKey
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
func LookupPeers(ctx context.Context, service string, localPeer string) (map[string]string, error) {
	entries, err := Lookup(ctx, service, clusterSize)
	if err != nil {
		return nil, err
	}

	peers := map[string]string{}
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

		if entry.AddrV6 != nil {
			peers[peerName] = entry.AddrV6.String()

		} else {
			peers[peerName] = entry.AddrV4.String()
		}

	}

	return peers, nil
}

// Lookup any records with join tokens matching the peer name. Accepts a function for acting on the token.
func LookupJoinToken(ctx context.Context, peer string, f func(token map[string]JoinConfig) error) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				entries, err := Lookup(ctx, TokenService, 1)
				if err != nil {
					logger.Error("Failed lookup", logger.Ctx{"name": peer, "error": err})

					return
				}

				if len(entries) == 0 || entries[0] == nil || len(entries[0].InfoFields) == 0 {
					logger.Info("Received incomplete record, retrying in 5s...")
					time.Sleep(5 * time.Second)
					continue
				}

				token, err := parseJoinToken(peer, entries[0])
				if err != nil {
					logger.Error("Failed to parse join token", logger.Ctx{"name": peer, "error": err})
					return
				}

				if token == nil {
					logger.Warnf("Peer %q was not found in the token broadcast", peer)
					continue
				}

				err = f(token)
				if err != nil {
					logger.Error("Failed to handle join token", logger.Ctx{"name": peer, "error": err})
					return
				}

				return
			}
		}
	}()
}

func parseJoinToken(peer string, entry *mdns.ServiceEntry) (map[string]JoinConfig, error) {
	var tokensByName map[string]map[string]JoinConfig
	unquoted, err := strconv.Unquote("\"" + strings.Join(entry.InfoFields, "") + "\"")
	if err != nil {
		return nil, fmt.Errorf("Failed to format DNS TXT record: %w", err)
	}

	err = json.Unmarshal([]byte(unquoted), &tokensByName)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse token map: %w", err)
	}

	record, ok := tokensByName[peer]
	if !ok {
		return nil, nil
	}

	return record, nil
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
