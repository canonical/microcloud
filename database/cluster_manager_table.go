package database

//go:generate -command mapper lxd-generate db mapper -t cluster_manager.mapper.go
//go:generate mapper reset
//
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e ClusterManager objects table=cluster_manager
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e ClusterManager objects-by-ID table=cluster_manager
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e ClusterManager id table=cluster_manager
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e ClusterManager delete-by-ID table=cluster_manager
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e ClusterManager create table=cluster_manager
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e ClusterManager update table=cluster_manager
//
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e ClusterManager GetMany table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e ClusterManager GetOne table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e ClusterManager ID table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e ClusterManager Exists table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e ClusterManager Create table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e ClusterManager Update table=cluster_manager
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e ClusterManager DeleteOne-by-ID table=cluster_manager

// ClusterManager is used to track the cluster manager configuration.
type ClusterManager struct {
	ID                    int64 `db:"primary=yes"`
	Addresses             string
	ServerCertFingerprint string
	UpdateIntervalSeconds int64
}

// ClusterManagerFilter is used to filter cluster manager queries.
type ClusterManagerFilter struct {
	ID *int64
}
