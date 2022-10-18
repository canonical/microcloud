package service

import (
	"encoding/pem"
	"fmt"

	"github.com/lxc/lxd/client"
	"github.com/lxc/lxd/lxd/revert"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/logger"
	"github.com/lxc/lxd/shared/version"
)

type initDataNode struct {
	api.ServerPut `yaml:",inline"`
	StoragePools  []api.StoragePoolsPost `json:"storage_pools" yaml:"storage_pools"`
	Profiles      []api.ProfilesPost     `json:"profiles" yaml:"profiles"`
}

// Helper to initialize node-specific entities on a LXD instance using the
// definitions from the given initDataNode object.
//
// It's used both by the 'lxd init' command and by the PUT /1.0/cluster API.
//
// In case of error, the returned function can be used to revert the changes.
func initDataNodeApply(d lxd.InstanceServer, config initDataNode) (func(), error) {
	revert := revert.New()
	defer revert.Fail()

	// Apply server configuration.
	if config.Config != nil && len(config.Config) > 0 {
		// Get current config.
		currentServer, etag, err := d.GetServer()
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve current server configuration: %w", err)
		}

		// Setup reverter.
		revert.Add(func() { _ = d.UpdateServer(currentServer.Writable(), "") })

		// Prepare the update.
		newServer := api.ServerPut{}
		err = shared.DeepCopy(currentServer.Writable(), &newServer)
		if err != nil {
			return nil, fmt.Errorf("Failed to copy server configuration: %w", err)
		}

		for k, v := range config.Config {
			newServer.Config[k] = fmt.Sprintf("%v", v)
		}

		// Apply it.
		err = d.UpdateServer(newServer, etag)
		if err != nil {
			return nil, fmt.Errorf("Failed to update server configuration: %w", err)
		}
	}

	// Apply storage configuration.
	if config.StoragePools != nil && len(config.StoragePools) > 0 {
		// Get the list of storagePools.
		storagePoolNames, err := d.GetStoragePoolNames()
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of storage pools: %w", err)
		}

		// StoragePool creator
		createStoragePool := func(storagePool api.StoragePoolsPost) error {
			// Create the storagePool if doesn't exist.
			err := d.CreateStoragePool(storagePool)
			if err != nil {
				return fmt.Errorf("Failed to create storage pool %q: %w", storagePool.Name, err)
			}

			// Setup reverter.
			revert.Add(func() { _ = d.DeleteStoragePool(storagePool.Name) })
			return nil
		}

		// StoragePool updater.
		updateStoragePool := func(storagePool api.StoragePoolsPost) error {
			// Get the current storagePool.
			currentStoragePool, etag, err := d.GetStoragePool(storagePool.Name)
			if err != nil {
				return fmt.Errorf("Failed to retrieve current storage pool %q: %w", storagePool.Name, err)
			}

			// Quick check.
			if currentStoragePool.Driver != storagePool.Driver {
				return fmt.Errorf("Storage pool %q is of type %q instead of %q", currentStoragePool.Name, currentStoragePool.Driver, storagePool.Driver)
			}

			// Setup reverter.
			revert.Add(func() { _ = d.UpdateStoragePool(currentStoragePool.Name, currentStoragePool.Writable(), "") })

			// Prepare the update.
			newStoragePool := api.StoragePoolPut{}
			err = shared.DeepCopy(currentStoragePool.Writable(), &newStoragePool)
			if err != nil {
				return fmt.Errorf("Failed to copy configuration of storage pool %q: %w", storagePool.Name, err)
			}

			// Description override.
			if storagePool.Description != "" {
				newStoragePool.Description = storagePool.Description
			}

			// Config overrides.
			for k, v := range storagePool.Config {
				newStoragePool.Config[k] = fmt.Sprintf("%v", v)
			}

			// Apply it.
			err = d.UpdateStoragePool(currentStoragePool.Name, newStoragePool, etag)
			if err != nil {
				return fmt.Errorf("Failed to update storage pool %q: %w", storagePool.Name, err)
			}

			return nil
		}

		for _, storagePool := range config.StoragePools {
			// New storagePool.
			if !shared.StringInSlice(storagePool.Name, storagePoolNames) {
				err := createStoragePool(storagePool)
				if err != nil {
					return nil, err
				}

				continue
			}

			// Existing storagePool.
			err := updateStoragePool(storagePool)
			if err != nil {
				return nil, err
			}
		}
	}

	// Apply profile configuration.
	if config.Profiles != nil && len(config.Profiles) > 0 {
		// Get the list of profiles.
		profileNames, err := d.GetProfileNames()
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of profiles: %w", err)
		}

		// Profile creator.
		createProfile := func(profile api.ProfilesPost) error {
			// Create the profile if doesn't exist.
			err := d.CreateProfile(profile)
			if err != nil {
				return fmt.Errorf("Failed to create profile %q: %w", profile.Name, err)
			}

			// Setup reverter.
			revert.Add(func() { _ = d.DeleteProfile(profile.Name) })
			return nil
		}

		// Profile updater.
		updateProfile := func(profile api.ProfilesPost) error {
			// Get the current profile.
			currentProfile, etag, err := d.GetProfile(profile.Name)
			if err != nil {
				return fmt.Errorf("Failed to retrieve current profile %q: %w", profile.Name, err)
			}

			// Setup reverter.
			revert.Add(func() { _ = d.UpdateProfile(currentProfile.Name, currentProfile.Writable(), "") })

			// Prepare the update.
			newProfile := api.ProfilePut{}
			err = shared.DeepCopy(currentProfile.Writable(), &newProfile)
			if err != nil {
				return fmt.Errorf("Failed to copy configuration of profile %q: %w", profile.Name, err)
			}

			// Description override.
			if profile.Description != "" {
				newProfile.Description = profile.Description
			}

			// Config overrides.
			for k, v := range profile.Config {
				newProfile.Config[k] = fmt.Sprintf("%v", v)
			}

			// Device overrides.
			for k, v := range profile.Devices {
				// New device.
				_, ok := newProfile.Devices[k]
				if !ok {
					newProfile.Devices[k] = v
					continue
				}

				// Existing device.
				for configKey, configValue := range v {
					newProfile.Devices[k][configKey] = fmt.Sprintf("%v", configValue)
				}
			}

			// Apply it.
			err = d.UpdateProfile(currentProfile.Name, newProfile, etag)
			if err != nil {
				return fmt.Errorf("Failed to update profile %q: %w", profile.Name, err)
			}

			return nil
		}

		for _, profile := range config.Profiles {
			// New profile.
			if !shared.StringInSlice(profile.Name, profileNames) {
				err := createProfile(profile)
				if err != nil {
					return nil, err
				}

				continue
			}

			// Existing profile.
			err := updateProfile(profile)
			if err != nil {
				return nil, err
			}
		}
	}

	cleanup := revert.Clone().Fail // Clone before calling revert.Success() so we can return the Fail func.
	revert.Success()
	return cleanup, nil
}

func (s *LXDService) configFromToken(token string) (*api.ClusterPut, error) {
	joinToken, err := shared.JoinTokenDecode(token)
	if err != nil {
		return nil, fmt.Errorf("Invalid cluster join token: %w", err)
	}
	config := &api.ClusterPut{
		Cluster:       api.Cluster{ServerName: s.name, Enabled: true},
		ServerAddress: util.CanonicalNetworkAddress(s.address, s.port),
	}

	// Attempt to find a working cluster member to use for joining by retrieving the
	// cluster certificate from each address in the join token until we succeed.
	for _, clusterAddress := range joinToken.Addresses {
		// Cluster URL
		config.ClusterAddress = util.CanonicalNetworkAddress(clusterAddress, s.port)

		// Cluster certificate
		cert, err := shared.GetRemoteCertificate(fmt.Sprintf("https://%s", config.ClusterAddress), version.UserAgent)
		if err != nil {
			logger.Warnf("Error connecting to existing cluster member %q: %v\n", clusterAddress, err)
			continue
		}

		certDigest := shared.CertFingerprint(cert)
		if joinToken.Fingerprint != certDigest {
			return nil, fmt.Errorf("Certificate fingerprint mismatch between join token and cluster member %q", clusterAddress)
		}

		config.ClusterCertificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))

		break // We've found a working cluster member.
	}

	if config.ClusterCertificate == "" {
		return nil, fmt.Errorf("Unable to connect to any of the cluster members specified in join token")
	}

	return config, nil
}
