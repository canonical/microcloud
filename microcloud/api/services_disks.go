package api

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/state"
	"github.com/gorilla/mux"
	"github.com/lxc/lxd/lxd/resources"
	"github.com/lxc/lxd/lxd/response"
	"github.com/lxc/lxd/shared"
)

// Disks is an endpoint for special MicroCloud handling of disks.
var Disks = rest.Endpoint{
	Path: "services/disks/{deviceID}",

	Put: rest.EndpointAction{Handler: disksPut, AllowUntrusted: true, ProxyTarget: true},
}

// disksPut wipes the disk with the given device ID.
func disksPut(s *state.State, r *http.Request) response.Response {
	device, err := url.PathUnescape(mux.Vars(r)["deviceID"])
	if err != nil {
		return response.SmartError(err)
	}

	storage, err := resources.GetStorage()
	if err != nil {
		return response.SmartError(fmt.Errorf("Unable to list system disks: %w", err))
	}

	var path string
	for _, disk := range storage.Disks {
		// Check if full disk.
		if disk.DeviceID == device && len(disk.Partitions) == 0 {
			path = fmt.Sprintf("/dev/disk/by-id/%s", disk.DeviceID)
			break
		}
	}

	if path == "" {
		return response.NotFound(fmt.Errorf("Device %q not found on %q", device, s.Name()))
	}

	// Wipe the block device if requested.
	// FIXME: Do a Go implementation.
	_, err = shared.RunCommand("dd", "if=/dev/zero", fmt.Sprintf("of=%s", path), "bs=512", "count=10", "status=none")
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to wipe device %q: %w", path, err))
	}

	return response.EmptySyncResponse
}
