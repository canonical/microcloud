package types

// System represents the preseed configuration for a single cluster member.
type System struct {
	Name            string      `json:"name,omitempty" yaml:"name"`
	Address         string      `json:"address,omitempty" yaml:"address"`
	UplinkInterface string      `json:"ovn_uplink_interface,omitempty" yaml:"ovn_uplink_interface"`
	UnderlayIP      string      `json:"ovn_underlay_ip,omitempty" yaml:"ovn_underlay_ip"`
	Storage         InitStorage `json:"storage,omitempty" yaml:"storage"`
}

// InitStorage separates the direct paths used for local and ceph disks.
type InitStorage struct {
	Local DirectStorage   `json:"local,omitempty" yaml:"local"`
	Ceph  []DirectStorage `json:"ceph,omitempty" yaml:"ceph"`
}

// DirectStorage is a direct path to a disk, to be used to override a disk filter.
type DirectStorage struct {
	Path    string `json:"path,omitempty" yaml:"path"`
	Wipe    bool   `json:"wipe,omitempty" yaml:"wipe"`
	Encrypt bool   `json:"encrypt,omitempty" yaml:"encrypt"`
}
