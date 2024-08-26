package types

import (
	"time"
)

// SessionRole indicates the role when participating in a trust establishment session.
type SessionRole string

const (
	// SessionInitiating represents the session of the initiator.
	SessionInitiating SessionRole = "initiating"

	// SessionJoining represents the session of the joiner.
	SessionJoining SessionRole = "joining"
)

// Session represents the websocket protocol used during trust establishment between the client and server.
// Empty fields are omitted to require sending only the necessary information.
type Session struct {
	Address              string            `json:"address,omitempty"`
	InitiatorAddress     string            `json:"initiator_address,omitempty"`
	InitiatorName        string            `json:"initiator_name,omitempty"`
	InitiatorFingerprint string            `json:"initiator_fingerprint,omitempty"`
	Interface            string            `json:"interface,omitempty"`
	Passphrase           string            `json:"passphrase,omitempty"`
	Services             []ServiceType     `json:"services,omitempty"`
	Intent               SessionJoinPost   `json:"intent,omitempty"`
	ConfirmedIntents     []SessionJoinPost `json:"confirmed_intents,omitempty"`
	Accepted             bool              `json:"accepted,omitempty"`
	LookupTimeout        time.Duration     `json:"lookup_timeout,omitempty"`
	Error                string            `json:"error,omitempty"`
}

// SessionJoinPost represents a request made to join an active session.
type SessionJoinPost struct {
	Name        string        `json:"name" yaml:"name"`
	Version     string        `json:"version" yaml:"version"`
	Address     string        `json:"address" yaml:"address"`
	Certificate string        `json:"certificate" yaml:"certificate"`
	Services    []ServiceType `json:"services" yaml:"services"`
}
