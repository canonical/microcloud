package multicast

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/canonical/lxd/shared/logger"
	"golang.org/x/net/ipv4"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// ServerInfo is information about the server that is discovered using multicast.
type ServerInfo struct {
	Version     string                         `json:"version"`
	Name        string                         `json:"name,omitempty"`
	Address     string                         `json:"address,omitempty"`
	Components  map[types.ComponentType]string `json:"components,omitempty"`
	Certificate *x509.Certificate              `json:"certificates,omitempty"`
}

// Discovery represents the information used for discovering peers using multicast.
type Discovery struct {
	iface           string
	port            int64
	group           net.IP
	responderConn   *ipv4.PacketConn
	responderCancel context.CancelFunc
}

// NewDiscovery returns a new instance of Discovery which allows to lookup peers
// and to respond on multicast queries.
func NewDiscovery(iface string, port int64) *Discovery {
	return &Discovery{
		iface: iface,
		port:  port,
		// This uses an address of the organization-local scope which isn't reserved for any public protocol.
		// See https://www.iana.org/assignments/multicast-addresses/multicast-addresses.xhtml#multicast-addresses-12.
		group: net.IPv4(239, 100, 100, 100),
	}
}

// Respond starts a new server that listens for datagrams on the configured multicast group
// and sends the given info in response until the context is cancelled.
func (d *Discovery) Respond(ctx context.Context, info ServerInfo) error {
	iface, err := net.InterfaceByName(d.iface)
	if err != nil {
		return fmt.Errorf("Failed to resolve server interface %q: %w", d.iface, err)
	}

	// The PacketConn gets closed when calling Close on the derived IPv4 PacketConn.
	receiver, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", d.port))
	if err != nil {
		return fmt.Errorf("Failed to listen on %d: %w", d.port, err)
	}

	ctx, d.responderCancel = context.WithCancel(ctx)
	d.responderConn = ipv4.NewPacketConn(receiver)
	err = d.responderConn.JoinGroup(iface, &net.UDPAddr{IP: d.group})
	if err != nil {
		return fmt.Errorf("Failed to join multicast group %q: %w", d.group.String(), err)
	}

	err = d.responderConn.SetControlMessage(ipv4.FlagDst, true)
	if err != nil {
		return fmt.Errorf("Failed to set IPv4 control flag for destination address: %w", err)
	}

	// Close the network endpoint if the outer context got cancelled.
	// This allows existing the endpoint's blocking read using ReadFrom.
	go func() {
		<-ctx.Done()
		err := d.responderConn.Close()
		if err != nil {
			logger.Error("Failed to close network endpoint after context got cancelled", logger.Ctx{"err": err})
		}
	}()

	// Respond on received multicast datagrams.
	// The routine exits if the connection gets closed.
	go func() {
		for {
			// See the comment on the sender (lookup) for the reasoning about using 500.
			b := make([]byte, 500)
			n, cm, src, err := d.responderConn.ReadFrom(b)
			if err != nil {
				// Ignore "use of closed network connection" errors as this happens normally
				// if the outer context gets cancelled in the connection closer go routine.
				if !errors.Is(err, net.ErrClosed) {
					logger.Error("Failed to read from network endpoint", logger.Ctx{"err": err})
				}

				return
			}

			receivedInfo := ServerInfo{}

			// Reslice the byte slice with the actual amount of bytes read from the datagram.
			err = json.Unmarshal(b[:n], &receivedInfo)
			if err != nil {
				logger.Error("Failed to parse received multicast server info", logger.Ctx{"err": err})
				continue
			}

			// Don't respond on this request as the peer is using a different version.
			if receivedInfo.Version != info.Version {
				logger.Warnf("Don't respond to multicast server info from %q as its using version %q", src.String(), receivedInfo.Version)
				continue
			}

			if cm.Dst.IsMulticast() {
				if cm.Dst.Equal(d.group) {
					bytes, err := json.Marshal(info)
					if err != nil {
						logger.Error("Failed to marshal server info", logger.Ctx{"err": err})
						continue
					}

					// Send a unicast message back to the source.
					_, err = d.responderConn.WriteTo(bytes, nil, src)
					if err != nil {
						logger.Error("Failed to send reply", logger.Ctx{"dest": src.String(), "err": err})
						continue
					}
				} else {
					logger.Warnf("Received multicast message from non recognized group %q", cm.Dst.String())
				}
			}
		}
	}()

	return nil
}

// StopResponder stops the responder server and cancels it's inner context.
func (d *Discovery) StopResponder() error {
	// Check if this instance of discovery has an active responder server connection.
	if d.responderConn != nil {
		err := d.responderConn.Close()
		// Ignore errors if the connection is already closed.
		// This can happen if the responders context already got cancelled
		// which also triggers a close of the connection.
		if err != nil && !errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("Failed to stop responder: %w", err)
		}

		// Cancel the inner context too and release all routines of the responder.
		if d.responderCancel != nil {
			d.responderCancel()
		}
	}

	return nil
}

// Lookup finds a listening peer matching the given version and returns its info.
func (d *Discovery) Lookup(ctx context.Context, version string) (*ServerInfo, error) {
	// Use a random port for sending the multicast message.
	// The PacketConn gets closed when calling Close on the derived IPv4 PacketConn.
	sender, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("Failed to listen: %w", err)
	}

	iface, err := net.InterfaceByName(d.iface)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve lookup interface %q: %w", d.iface, err)
	}

	senderP := ipv4.NewPacketConn(sender)
	err = senderP.SetMulticastInterface(iface)
	if err != nil {
		return nil, fmt.Errorf("Failed to set multicast interface %q: %w", iface.Name, err)
	}

	lookupInfo := ServerInfo{
		Version: version,
	}

	lookupInfoBytes, err := json.Marshal(lookupInfo)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal lookup info: %w", err)
	}

	go func() {
		dst := &net.UDPAddr{IP: d.group, Port: int(d.port)}

		for {
			select {
			case <-ctx.Done():
				// Close the network endpoint if the lookup context got cancelled.
				senderP.Close()
				return
			default:
				// Repeatedly send multicast message with our lookup info containing only our protocol version.
				// The response contains the name, address and version which we use to validate if we want to join this peer.
				_, err = senderP.WriteTo(lookupInfoBytes, nil, dst)
				if err != nil {
					logger.Error("Failed to send multicast message", logger.Ctx{"err": err})
				}

				time.Sleep(time.Second)
			}
		}
	}()

	// 500 bytes should always make it through the network regardless of the MTU setting
	// as Internet Protocol requires hosts to be able to process datagrams of at least 576 bytes.
	// Subtracting the maximum IP header of size 60 bytes and the UDP header of size 8 bytes we are
	// left with 508 bytes for the actual payload.
	// We expect a response that contains the name, address and version.
	// As the name correlates to the peers hostname, 255 may be occupied by it which leaves another
	// 245 bytes for the address (IPv4 or IPv6) and used multicast discovery version (including some JSON formatting).
	b := make([]byte, 500)

	// Block until the read succeeds or the connection is closed.
	// The latter happens in case the context gets cancelled.
	n, _, _, err := senderP.ReadFrom(b)
	if err != nil {
		// In case the connection got closed due to a cancelled context,
		// try to return the cause from the context instead.
		ctxErr := context.Cause(ctx)
		if errors.Is(err, net.ErrClosed) && ctxErr != nil {
			err = ctxErr
		}

		return nil, fmt.Errorf("Failed to read from multicast network endpoint: %w", err)
	}

	receivedInfo := ServerInfo{}

	// Reslice the byte slice with the actual amount of bytes read from the datagram.
	err = json.Unmarshal(b[:n], &receivedInfo)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse received multicast server info: %w", err)
	}

	// Exit if peer has mismatched version.
	if receivedInfo.Version != version {
		return nil, fmt.Errorf("System %q (version %q) has a version mismatch: Expected %q", receivedInfo.Name, receivedInfo.Version, version)
	}

	return &receivedInfo, nil
}
