package api

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/trust"
	"github.com/canonical/lxd/shared/ws"
	microTypes "github.com/canonical/microcluster/v3/microcluster/types"
	"golang.org/x/sync/errgroup"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/multicast"
	"github.com/canonical/microcloud/microcloud/service"
)

// HMACMicroCloud10 is the HMAC format version used during trust establishment.
const HMACMicroCloud10 trust.HMACVersion = "MicroCloud-1.0"

// SessionInitiatingCmd represents the /1.0/session/initiating API on MicroCloud.
var SessionInitiatingCmd = func(sh *service.Handler) microTypes.Endpoint {
	return microTypes.Endpoint{
		AllowedBeforeInit: true,
		Name:              "session/initiating",
		Path:              "session/initiating",

		Get: microTypes.EndpointAction{Handler: authHandlerMTLS(sh, sessionGet(sh, types.SessionInitiating))},
	}
}

// SessionJoiningCmd represents the /1.0/session/joining API on MicroCloud.
var SessionJoiningCmd = func(sh *service.Handler) microTypes.Endpoint {
	return microTypes.Endpoint{
		AllowedBeforeInit: true,
		Name:              "session/joining",
		Path:              "session/joining",

		Get: microTypes.EndpointAction{Handler: authHandlerMTLS(sh, sessionGet(sh, types.SessionJoining))},
	}
}

// sessionGet returns a MicroCloud join session.
func sessionGet(sh *service.Handler, sessionRole types.SessionRole) func(state microTypes.State, r *http.Request) microTypes.Response {
	return func(state microTypes.State, r *http.Request) microTypes.Response {
		if sh.ActiveSession() {
			return response.BadRequest(errors.New("There already is an active session"))
		}

		sessionTimeoutStr := r.URL.Query().Get("timeout")
		if sessionTimeoutStr == "" {
			sessionTimeoutStr = "10m"
		}

		sessionTimeout, err := time.ParseDuration(sessionTimeoutStr)
		if err != nil {
			return response.BadRequest(fmt.Errorf("Failed to parse timeout: %w", err))
		}

		if time.Now().Add(sessionTimeout).After(time.Now().Add(1 * time.Hour)) {
			return response.BadRequest(errors.New("Session timeout cannot exceed 60 minutes"))
		}

		return response.ManualResponse(func(w http.ResponseWriter) error {
			conn, err := ws.Upgrader.Upgrade(w, r, nil)
			if err != nil {
				return err
			}

			defer func() {
				err := conn.Close()
				if err != nil {
					// Ignore "use of closed network connection" errors as this happens normally
					// if the connection already got closed.
					if !errors.Is(err, net.ErrClosed) {
						logger.Error("Failed to close the websocket connection", logger.Ctx{"err": err})
					}
				}
			}()

			sessionCtx, cancel := context.WithTimeoutCause(r.Context(), sessionTimeout, errors.New("Session timeout exceeded"))
			defer cancel()

			gw := cloudClient.NewWebsocketGateway(sessionCtx, conn)

			switch sessionRole {
			case types.SessionInitiating:
				err = handleInitiatingSession(state, sh, gw)
			case types.SessionJoining:
				err = handleJoiningSession(state, sh, gw)
			}

			// Any errors occurring after the connection got upgraded have to be handled
			// within the websocket.
			// When writing a response to the original HTTP connection the server will
			// complain with "http: connection has been hijacked".
			if err != nil {
				controlErr := gw.WriteClose(err)
				if controlErr != nil {
					logger.Error("Failed to write close control message", logger.Ctx{"err": controlErr, "controlErr": err})
				}
			}

			return nil
		})
	}
}

func confirmedIntents(sh *service.Handler, gw *cloudClient.WebsocketGateway) ([]types.SessionJoinPost, error) {
	for {
		select {
		case intent, ok := <-sh.Session.IntentCh():
			// Session got closed, try to receive the cause from the other channels.
			if !ok {
				continue
			}

			err := gw.Write(types.Session{
				Intent: intent,
			})
			if err != nil {
				return nil, fmt.Errorf("Failed to forward join intent: %w", err)
			}

		case bytes := <-gw.Receive():
			var session types.Session
			err := json.Unmarshal(bytes, &session)
			if err != nil {
				return nil, fmt.Errorf("Failed to read confirmed intents: %w", err)
			}

			return session.ConfirmedIntents, nil
		case <-gw.Context().Done():
			return nil, fmt.Errorf("Exit waiting for intents: %w", context.Cause(gw.Context()))
		}
	}
}

func handleInitiatingSession(state microTypes.State, sh *service.Handler, gw *cloudClient.WebsocketGateway) error {
	session := types.Session{}
	err := gw.ReceiveWithContext(gw.Context(), &session)
	if err != nil {
		return fmt.Errorf("Failed to read session start message: %w", err)
	}

	err = sh.StartSession(types.SessionInitiating, session.Passphrase, gw)
	if err != nil {
		return fmt.Errorf("Failed to start session: %w", err)
	}

	defer func() {
		err := sh.StopSession(nil)
		if err != nil {
			logger.Error("Failed to stop session", logger.Ctx{"err": err})
		}
	}()

	err = sh.Session.MulticastDiscovery(state.Name(), session.Address, session.Interface)
	if err != nil {
		return fmt.Errorf("Failed to start multicast discovery: %w", err)
	}

	sessionPassphrase := sh.Session.Passphrase()
	err = gw.Write(types.Session{
		Passphrase: sessionPassphrase,
	})
	if err != nil {
		return fmt.Errorf("Failed to send session details: %w", err)
	}

	confirmedIntents, err := confirmedIntents(sh, gw)
	if err != nil {
		return fmt.Errorf("Failed waiting for the confirmed intents: %w", err)
	}

	g, ctx := errgroup.WithContext(context.Background())

	// Add systems to temporary truststore.
	for _, intent := range confirmedIntents {
		remoteCert, err := shared.ParseCert([]byte(intent.Certificate))
		if err != nil {
			return fmt.Errorf("Failed to parse certificate of confirmed intent: %w", err)
		}

		// Add system to temporary truststore.
		sh.Session.Allow(intent.Name, *remoteCert)

		cloud := sh.Services[types.MicroCloud].(*service.CloudService)
		cert, err := cloud.ServerCert()
		if err != nil {
			return fmt.Errorf("Failed to get certificate of %q: %w", types.MicroCloud, err)
		}

		joinIntent := types.SessionJoinPost{
			Version:     multicast.Version,
			Name:        state.Name(),
			Address:     session.Address,
			Certificate: string(cert.PublicKey()),
			Services:    session.Services,
		}

		h, err := trust.NewHMACArgon2([]byte(sessionPassphrase), nil, trust.NewDefaultHMACConf(HMACMicroCloud10))
		if err != nil {
			return fmt.Errorf("Failed to create a new HMAC instance using argon2: %w", err)
		}

		header, err := trust.HMACAuthorizationHeader(h, joinIntent)
		if err != nil {
			return fmt.Errorf("Failed to create HMAC for join intent: %w", err)
		}

		// Confirm join intent.
		// This request uses polling to wait for confirmation from the other side.
		g.Go(func() error {
			conf := cloudClient.AuthConfig{
				HMAC: header,
				// We already know the certificate of the joiner for TLS verification.
				TLSServerCertificate: remoteCert,
			}

			_, err := cloud.RequestJoinIntent(ctx, intent.Address, conf, joinIntent)
			if err != nil {
				return fmt.Errorf("Failed to confirm join intent of %q: %w", intent.Address, err)
			}

			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return fmt.Errorf("Failed to confirm join intents: %w", err)
	}

	err = gw.Write(types.Session{
		Accepted: true,
	})
	if err != nil {
		return fmt.Errorf("Failed to send confirmation: %w", err)
	}

	return nil
}

func handleJoiningSession(state microTypes.State, sh *service.Handler, gw *cloudClient.WebsocketGateway) error {
	session := types.Session{}
	err := gw.ReceiveWithContext(gw.Context(), &session)
	if err != nil {
		return fmt.Errorf("Failed to read session start message: %w", err)
	}

	err = sh.StartSession(types.SessionJoining, session.Passphrase, gw)
	if err != nil {
		return fmt.Errorf("Failed to start session: %w", err)
	}

	defer func() {
		err := sh.StopSession(nil)
		if err != nil {
			logger.Error("Failed to stop session", logger.Ctx{"err": err})
		}
	}()

	// No address selected, try to lookup system.
	if session.InitiatorAddress == "" {
		lookupCtx, cancel := context.WithTimeoutCause(gw.Context(), session.LookupTimeout, errors.New("Lookup timeout exceeded"))
		defer cancel()

		discovery := multicast.NewDiscovery(session.Interface, service.CloudMulticastPort)
		peer, err := discovery.Lookup(lookupCtx, multicast.Version)
		if err != nil {
			return fmt.Errorf("Failed to lookup eligible system: %w", err)
		}

		session.InitiatorAddress = peer.Address
	}

	// Get the remotes name.
	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	cert, err := cloud.ServerCert()
	if err != nil {
		return fmt.Errorf("Failed to get certificate of %q: %w", types.MicroCloud, err)
	}

	joinIntent := types.SessionJoinPost{
		Version:     multicast.Version,
		Name:        state.Name(),
		Address:     session.Address,
		Certificate: string(cert.PublicKey()),
		Services:    session.Services,
	}

	h, err := trust.NewHMACArgon2([]byte(session.Passphrase), nil, trust.NewDefaultHMACConf(HMACMicroCloud10))
	if err != nil {
		return fmt.Errorf("Failed to create a new HMAC instance using argon2: %w", err)
	}

	header, err := trust.HMACAuthorizationHeader(h, joinIntent)
	if err != nil {
		return fmt.Errorf("Failed to create HMAC for join intent: %w", err)
	}

	conf := cloudClient.AuthConfig{
		HMAC: header,
		// The certificate of the initiator isn't yet known so we have to skip any TLS verification.
		InsecureSkipVerify: true,
	}

	peerCert, err := cloud.RequestJoinIntent(context.Background(), session.InitiatorAddress, conf, joinIntent)
	if err != nil {
		// If the HMAC of the request is invalid, a generic error is returned by the API.
		// It's likely that the user provided the wrong passphrase.
		// Indicate this in the error by rewriting it.
		if err.Error() == "Invalid HMAC" {
			err = errors.New("Wrong passphrase")
		}

		return fmt.Errorf("Failed to send our intent to join %q: %w", session.InitiatorAddress, err)
	}

	session.InitiatorFingerprint = shared.CertFingerprint(peerCert)

	peerStatus, err := cloud.RemoteStatus(gw.Context(), peerCert, session.InitiatorAddress)
	if err != nil {
		return fmt.Errorf("Failed to retrieve cluster status from %q: %w", session.InitiatorAddress, err)
	}

	session.InitiatorName = peerStatus.Name

	// Notify the client we have found an eligible system.
	err = gw.Write(types.Session{
		InitiatorName:        session.InitiatorName,
		InitiatorAddress:     session.InitiatorAddress,
		InitiatorFingerprint: session.InitiatorFingerprint,
	})
	if err != nil {
		return fmt.Errorf("Failed to confirm the eligible system %q at %q: %w", session.InitiatorName, session.InitiatorAddress, err)
	}

	var ok bool
	var confirmedIntent types.SessionJoinPost

	select {
	case confirmedIntent, ok = <-sh.Session.IntentCh():
		// Session got closed.
		if !ok {
			return errors.New("Exit waiting for join confirmation")
		}

		err = gw.Write(types.Session{
			Intent: confirmedIntent,
		})
		if err != nil {
			return fmt.Errorf("Failed to forward join confirmation: %w", err)
		}

	case <-gw.Context().Done():
		return fmt.Errorf("Exit waiting for join confirmation: %w", context.Cause(gw.Context()))
	}

	certBlock, _ := pem.Decode([]byte(confirmedIntent.Certificate))
	if certBlock == nil {
		return errors.New("Invalid certificate file")
	}

	remoteCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("Failed to parse certificate: %w", err)
	}

	// Add system to temporary truststore.
	sh.Session.Allow(confirmedIntent.Name, *remoteCert)

	var errStr string
	select {
	case <-sh.Session.ExitCh():
		errStr = ""
	case <-gw.Context().Done():
		errStr = fmt.Errorf("Exit waiting for session to end: %w", context.Cause(gw.Context())).Error()
	}

	err = gw.Write(types.Session{
		Error: errStr,
	})
	if err != nil {
		return fmt.Errorf("Failed to signal final message: %w", err)
	}

	return nil
}
