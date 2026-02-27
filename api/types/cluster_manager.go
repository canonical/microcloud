package types

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ClusterManagersPost represents the cluster manager configuration when receiving a POST request in MicroCloud.
type ClusterManagersPost struct {
	Name  string `json:"name" yaml:"name"`
	Token string `json:"token" yaml:"token"`
}

// ClusterManagerDelete represents the payload to delete a cluster manager configuration.
type ClusterManagerDelete struct {
	Force bool `json:"force" yaml:"force"`
}

// ClusterManager represents the configuration in MicroCloud of cluster manager.
type ClusterManager struct {
	// The remote address of cluster manager
	// Example: example.com:8443
	Addresses []string `json:"addresses" yaml:"addresses"`

	// CertificateFingerprint of the cluster manager server certificate
	// Example: 90fedb21cda5ac6a45a878c151e6ac8fe16b4b723e44669fd113e4ea07597d83
	CertificateFingerprint string `json:"certificate_fingerprint" yaml:"certificate_fingerprint"`

	// The time of the last successful status message sent to the cluster manager
	// Example: 2025-03-26T11:37:17.83536772Z
	StatusLastSuccessTime time.Time `json:"status_last_success_time" yaml:"status_last_success_time"`

	// The time of the last error response received on sending a status message to cluster manager
	// Example: 2025-03-26T11:37:17.83536772Z
	StatusLastErrorTime time.Time `json:"status_last_error_time" yaml:"status_last_error_time"`

	// The last error response received from the cluster manager
	// Example: Failed to send request to cluster manager: 403 Forbidden, body: {"type":"error","status":"","status_code":0,"operation":"","error_code":403,"error":"not authorized","metadata":null}
	StatusLastErrorResponse string `json:"status_last_error_response" yaml:"status_last_error_response"`

	// Additional configuration for the cluster manager
	// Example: update_interval: 60
	Config map[string]string `json:"config" yaml:"config,inline"`
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

	// Enables or disables the reverse tunnel to the cluster manager
	// Example: true, false
	ReverseTunnel *string `json:"reverse_tunnel" yaml:"reverse_tunnel"`
}

// StatusDistribution represents the distribution of items.
type StatusDistribution struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// StoragePoolUsage represents the storage usage distribution.
type StoragePoolUsage struct {
	Name   string `json:"name"`
	Member string `json:"member"`
	Total  uint64 `json:"total"`
	Usage  uint64 `json:"usage"`
}

// ServerMetrics represents the metrics from one cluster member service.
type ServerMetrics struct {
	Member  string      `json:"member"`
	Metrics string      `json:"metrics"`
	Service ServiceType `json:"service"`
}

// ClusterManagerPostStatus represents the periodic status payload sent to cluster manager.
type ClusterManagerPostStatus struct {
	CephStatuses      []StatusDistribution `json:"ceph_statuses"`
	CPUTotalCount     int64                `json:"cpu_total_count"`
	CPULoad1          string               `json:"cpu_load_1"`
	CPULoad5          string               `json:"cpu_load_5"`
	CPULoad15         string               `json:"cpu_load_15"`
	MemoryTotalAmount int64                `json:"memory_total_amount"`
	MemoryUsage       int64                `json:"memory_usage"`
	StoragePoolUsages []StoragePoolUsage   `json:"storage_pool_usages"`
	MemberStatuses    []StatusDistribution `json:"member_statuses"`
	InstanceStatuses  []StatusDistribution `json:"instance_statuses"`
	ServerMetrics     []ServerMetrics      `json:"server_metrics"`
	UIURL             string               `json:"ui_url"`
}

// ClusterManagerJoin represents the join payload sent to cluster manager.
type ClusterManagerJoin struct {
	ClusterName        string `json:"cluster_name" yaml:"cluster_name"`
	ClusterCertificate string `json:"cluster_certificate" yaml:"cluster_certificate"`
	Token              string `json:"token" yaml:"token"`
}

// ClusterManagerTunnel represents the tunnel connection the cluster manager.
type ClusterManagerTunnel struct {
	Mu     sync.RWMutex
	WsConn *websocket.Conn
}

// ClusterManagerTunnelRequest represents the request received through the tunnel.
type ClusterManagerTunnelRequest struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
}

// ClusterManagerTunnelResponse represents the response sent through the tunnel.
type ClusterManagerTunnelResponse struct {
	ID      string            `json:"id"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
}
