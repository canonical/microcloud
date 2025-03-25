package types

// ClusterManagerPost represents the cluster manager configuration when receiving a POST request.
type ClusterManagerPost struct {
	Token string `json:"token" yaml:"token"`
}

// ClusterManager represents the configuration in MicroCloud of cluster manager.
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

// StatusDistribution represents the distribution of items.
type StatusDistribution struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// ClusterManagerStatusPost represents the periodic status message sent to cluster manager.
type ClusterManagerStatusPost struct {
	CPUTotalCount     int64                `json:"cpu_total_count"`
	CPULoad1          string               `json:"cpu_load_1"`
	CPULoad5          string               `json:"cpu_load_5"`
	CPULoad15         string               `json:"cpu_load_15"`
	MemoryTotalAmount int64                `json:"memory_total_amount"`
	MemoryUsage       int64                `json:"memory_usage"`
	DiskTotalSize     int64                `json:"disk_total_size"`
	DiskUsage         int64                `json:"disk_usage"`
	MemberStatuses    []StatusDistribution `json:"member_statuses"`
	InstanceStatuses  []StatusDistribution `json:"instance_status"`
	Metrics           string               `json:"metrics"`
	UiUrl             string               `json:"ui_url"`
}

// ClusterManagerJoinPost represents the payload when sending a POST to join cluster manager.
type ClusterManagerJoinPost struct {
	ClusterName        string `json:"cluster_name" yaml:"cluster_name"`
	ClusterCertificate string `json:"cluster_certificate" yaml:"cluster_certificate"`
}
