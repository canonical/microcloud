package database

//go:generate -command mapper lxd-generate db mapper -t sites.mapper.go
//go:generate mapper reset
//
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e Site objects table=sites
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e Site objects-by-Name table=sites
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e Site id table=sites
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e Site create table=sites
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e Site delete-by-Name table=sites
//go:generate mapper stmt -d github.com/canonical/microcluster/v2/cluster -e Site update table=sites
//
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e Site GetMany table=sites
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e Site GetOne table=sites
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e Site ID table=sites
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e Site Exists table=sites
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e Site Create table=sites
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e Site Update table=sites
//go:generate mapper method -i -d github.com/canonical/microcluster/v2/cluster -e Site DeleteOne-by-Name table=sites

// Site is used to track the Sites.
type Site struct {
	ID          int
	Name        string
	Addresses   string
	Type        string
	Description string
}

// SiteFilter is used for filtering fields on database fetches.
type SiteFilter struct {
	Type *string
	Name *string
}

// SiteConfig is used for additional site configuration.
type SiteConfig struct {
	ID     int `db:"primary=yes"`
	SiteId string
	Key    string
	Value  string
}

// SiteConfigFilter is used for filtering fields on the SiteConfig.
type SiteConfigFilter struct {
	ID     *int
	SiteId *string
	Key    *string
}
