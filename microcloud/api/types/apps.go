package types

import (
	"github.com/canonical/lxd/shared/api"
)

const (
	// AppKindMicrok8s is used when setting the MicrocloudAppKindConfigKey for microk8s applications.
	AppKindMicrok8s = "microk8s"

	// MicrocloudAppNameConfigKey is used to filter instances belonging to a particular app.
	MicrocloudAppNameConfigKey = "user.microcloud_app_name"

	// MicrocloudAppKindConfigKey is used to filter instances belonging to apps of a particular kind.
	MicrocloudAppKindConfigKey = "user.microcloud_app_kind"

	// MicrocloudAppConfigKey is used to filter all app instances that have been deployed via microcloud.
	MicrocloudAppConfigKey = "user.microcloud_app"
)

// App represents a single app deployed via microcloud.
type App struct {
	Name    string `json:"name" yaml:"name"`
	Kind    string `json:"kind" yaml:"kind"`
	Project string `json:"project" yaml:"project"`
}

// AppInfo includes an App, but also contains and api.Instance's that contain the app, as well as app specific metadata.
type AppInfo struct {
	App       `yaml:",inline"`
	Instances []api.Instance `json:"instances" yaml:"instances"`
	Metadata  map[string]any `json:"metadata" yaml:"metadata"`
}

// Microk8sPost contains the configuration for setting up a microk8s cluster.
type Microk8sPost struct {
	ClusterName string   `json:"cluster_name" yaml:"cluster_name"`
	Profiles    []string `json:"profiles" yaml:"profiles"`
	Size        int      `json:"size" yaml:"size"`
	Channel     string   `json:"channel" yaml:"channel"`
}
