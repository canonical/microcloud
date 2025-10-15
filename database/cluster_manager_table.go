package database

import (
	"time"
)

//go:generate -command mapper lxd-generate db mapper -t cluster_manager.mapper.go
//go:generate mapper reset
//
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManager objects table=cluster_manager
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManager objects-by-ID table=cluster_manager
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManager objects-by-Name table=cluster_manager
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManager id table=cluster_manager
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManager delete-by-ID table=cluster_manager
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManager create table=cluster_manager
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManager update table=cluster_manager
//
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManager GetMany table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManager GetOne table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManager ID table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManager Exists table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManager Create table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManager Update table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManager DeleteOne-by-ID table=cluster_manager
//
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig objects table=cluster_manager_config
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig objects-by-ID table=cluster_manager_config
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig objects-by-ClusterManagerID table=cluster_manager_config
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig objects-by-ClusterManagerID-and-Key table=cluster_manager_config
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig id table=cluster_manager_config
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig delete-by-ClusterManagerID table=cluster_manager_config
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig create table=cluster_manager_config
//go:generate mapper stmt -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig update table=cluster_manager_config
//
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig GetMany table=cluster_manager_config
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig ID table=cluster_manager_config
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig Exists table=cluster_manager_config
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig Create table=cluster_manager_config
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig Update table=cluster_manager_config
//go:generate mapper method -i -d github.com/canonical/microcluster/v3/cluster -e ClusterManagerConfig DeleteOne-by-ID table=cluster_manager_config

// ClusterManager is used to track the cluster manager configuration.
type ClusterManager struct {
	ID                      int64 `db:"primary=yes"`
	Addresses               string
	CertificateFingerprint  string
	Name                    string
	StatusLastSuccessTime   time.Time
	StatusLastErrorTime     time.Time
	StatusLastErrorResponse string
}

// ClusterManagerFilter is used to filter cluster manager queries.
type ClusterManagerFilter struct {
	ID   *int64
	Name *string
}

// ClusterManagerConfig is used to store arbitrary cluster manager configuration.
type ClusterManagerConfig struct {
	ID               int64 `db:"primary=yes"`
	ClusterManagerID int64
	Key              string
	Value            string
}

// ClusterManagerConfigFilter is used to filter cluster manager configuration queries.
type ClusterManagerConfigFilter struct {
	ID               *int64
	ClusterManagerID *int64
	Key              *string
}
