package types

import (
	"time"
)

// ClusterManagersPost represents the cluster manager configuration when receiving a POST request in MicroCloud.
type ClusterManagersPost struct {
	Token string `json:"token" yaml:"token"`
}

// ClusterManagerGet represents the configuration in MicroCloud of cluster manager.
type ClusterManagerGet struct {
	// The remote address of cluster manager
	// Example: example.com:8443
	Addresses []string `json:"addresses" yaml:"addresses"`

	// CertificateFingerprint of the cluster manager server certificate
	// Example: 90fedb21cda5ac6a45a878c151e6ac8fe16b4b723e44669fd113e4ea07597d83
	CertificateFingerprint string `json:"certificate_fingerprint" yaml:"certificate_fingerprint"`

	// Interval in seconds to send status messages to the cluster manager
	// Example: 60
	UpdateInterval string `json:"update_interval" yaml:"update_interval"`

	// The time of the last successful status message sent to the cluster manager
	// Example: 2025-03-26T11:37:17.83536772Z
	StatusLastSuccessTime time.Time `json:"status_last_success_time" yaml:"status_last_success_time"`

	// The time of the last error response received on sending a status message to cluster manager
	// Example: 2025-03-26T11:37:17.83536772Z
	StatusLastErrorTime time.Time `json:"status_last_error_time" yaml:"status_last_error_time"`

	// The last error response received from the cluster manager
	// Example: Failed to send request to cluster manager: 403 Forbidden, body: {"type":"error","status":"","status_code":0,"operation":"","error_code":403,"error":"not authorized","metadata":null}
	StatusLastErrorResponse string `json:"status_last_error_response" yaml:"status_last_error_response"`
}

// ClusterManagerPut represents the payload to update one or multiple fields of the configuration.
type ClusterManagerPut struct {
	// The remote address of cluster manager
	// Example: example.com:8443
	Addresses []string `json:"addresses" yaml:"addresses"`

	// CertificateFingerprint of the cluster manager server certificate
	// Example: 90fedb21cda5ac6a45a878c151e6ac8fe16b4b723e44669fd113e4ea07597d83
	CertificateFingerprint *string `json:"certificate_fingerprint" yaml:"certificate_fingerprint"`

	// Interval in seconds to send status messages to the cluster manager
	// Example: 60
	UpdateInterval *string `json:"update_interval" yaml:"update_interval"`
}

// StatusDistribution represents the distribution of items.
type StatusDistribution struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// ClusterManagerPostStatus represents the periodic status payload sent to cluster manager.
type ClusterManagerPostStatus struct {
	CPUTotalCount     int64                `json:"cpu_total_count"`
	CPULoad1          string               `json:"cpu_load_1"`
	CPULoad5          string               `json:"cpu_load_5"`
	CPULoad15         string               `json:"cpu_load_15"`
	MemoryTotalAmount int64                `json:"memory_total_amount"`
	MemoryUsage       int64                `json:"memory_usage"`
	DiskTotalSize     int64                `json:"disk_total_size"`
	DiskUsage         int64                `json:"disk_usage"`
	MemberStatuses    []StatusDistribution `json:"member_statuses"`
	InstanceStatuses  []StatusDistribution `json:"instance_statuses"`
	Metrics           string               `json:"metrics"`
	UiUrl             string               `json:"ui_url"`
}

// ClusterManagerPostJoin represents the join payload sent to cluster manager.
type ClusterManagerPostJoin struct {
	ClusterName        string `json:"cluster_name" yaml:"cluster_name"`
	ClusterCertificate string `json:"cluster_certificate" yaml:"cluster_certificate"`
}
