package types

// ClusterManagerPost represents the cluster manager configuration when receiving a POST request.
type ClusterManagerPost struct {
	Token string `json:"token" yaml:"token"`
}

// ClusterManager represents the configuration in MicroCloud of cluster manager
//
// swagger:model
type ClusterManager struct {
	// The remote address of cluster manager
	// Example: example.com:8443
	Addresses []string `json:"addresses" yaml:"addresses"`

	// Fingerprint of the cluster manager server certificate
	// Example: 90fedb21cda5ac6a45a878c151e6ac8fe16b4b723e44669fd113e4ea07597d83
	Fingerprint *string `json:"fingerprint" yaml:"fingerprint"`

	// Interval in seconds to send status messages to the cluster manager
	// Example: 60
	UpdateInterval *string `json:"update_interval" yaml:"update_interval"`
}
