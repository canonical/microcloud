package mdns

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/microcluster/state"
	"github.com/hashicorp/mdns"
	"github.com/lxc/lxd/shared/logger"
)

// LookupPeers finds any broadcasting peers and returns a list of their names.
func LookupPeers(s *state.State) ([]string, error) {
	entries, err := Lookup(s.Context, ClusterService, clusterSize)
	if err != nil {
		return nil, err
	}

	peers := make([]string, 0, clusterSize)
	for _, entry := range entries {
		if !s.Database.IsOpen() {
			return nil, fmt.Errorf("Daemon is uninitialized")
		}

		if entry == nil {
			return nil, fmt.Errorf("Received empty record")
		}

		parts := strings.SplitN(entry.Name, fmt.Sprintf(".%s.local.", ClusterService), 2)
		peerName := parts[0]

		// Skip a response from ourselves.
		if s.Name() == peerName {
			continue
		}

		peers = append(peers, peerName)

	}

	return peers, nil
}

// Lookup any records with join tokens matching the peer name. Accepts a function for acting on the token.
func LookupJoinToken(ctx context.Context, peer string, f func(token string) error) {
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

func parseJoinToken(peer string, entry *mdns.ServiceEntry) (string, error) {
	var tokensByName map[string]string
	unquoted, err := strconv.Unquote("\"" + strings.Join(entry.InfoFields, "") + "\"")
	if err != nil {
		return "", fmt.Errorf("Failed to format DNS TXT record: %w", err)
	}

	err = json.Unmarshal([]byte(unquoted), &tokensByName)
	if err != nil {
		return "", fmt.Errorf("Failed to parse token map: %w", err)
	}

	record, ok := tokensByName[peer]
	if !ok {
		return "", fmt.Errorf("Found no token matching the peer %q", peer)
	}

	return record, nil
}

func Lookup(ctx context.Context, service string, size int) ([]*mdns.ServiceEntry, error) {
	// Start the lookup

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
