package api

import (
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"
	"github.com/gorilla/mux"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/service"
)

// ServicesClusterCmd represents the /1.0/services/cluster/{name} API on MicroCloud.
var ServicesClusterCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "services/cluster/{name}",
		Path:              "services/cluster/{name}",

		Delete: rest.EndpointAction{Handler: authHandlerMTLS(sh, removeClusterMember)},
	}
}

// removeClusterMember removes the given cluster member from all services that it exists in.
func removeClusterMember(state state.State, r *http.Request) response.Response {
	force := r.URL.Query().Get("force") == "1"
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.BadRequest(err)
	}

	supportedServices := map[types.ServiceType]string{
		types.MicroOVN:  MicroOVNDir,
		types.MicroCeph: MicroCephDir,
		types.LXD:       LXDDir,
	}

	existingServices := []types.ServiceType{types.MicroCloud}
	for serviceType, stateDir := range supportedServices {
		if service.Exists(serviceType, stateDir) {
			existingServices = append(existingServices, serviceType)
		}
	}

	addr, _, err := net.SplitHostPort(state.Address().URL.Host)
	if err != nil {
		return response.SmartError(fmt.Errorf("State address %q is invalid: %w", state.Address().String(), err))
	}

	sh, err := service.NewHandler(state.Name(), addr, state.FileSystem().StateDir, existingServices...)
	if err != nil {
		return response.SmartError(err)
	}

	// Remove the node from services in the following order:
	// 1. Remove from LXD first as it may have storage & networks that depend on the others for cleanup.
	// 2. Remove from MicroCeph and MicroOVN next, concurrently.
	// 3. Remove from MicroCloud last so that if there were any errors causing the other services to fail, MicroCloud will still know about the node.
	var memberExists bool
	err = sh.RunConcurrent(types.LXD, types.MicroCloud, func(s service.Service) error {
		existingMembers, err := s.ClusterMembers(r.Context())
		if err != nil && !api.StatusErrorCheck(err, http.StatusServiceUnavailable) {
			return err
		}

		// If we got a 503 error back, that means the service is installed, but hasn't been set up yet, so there are no cluster members to remove.
		if err != nil {
			return nil
		}

		// The cluster member may not exist for this service if it was removed manually, so skip removal for that service.
		_, ok := existingMembers[name]
		if !ok {
			logger.Warn("Cluster member not found for service", logger.Ctx{"service": s.Type(), "member": name})
			return nil
		}

		if !memberExists && ok {
			memberExists = ok
		}

		return s.DeleteClusterMember(r.Context(), name, force)
	})
	if err != nil {
		return response.SmartError(err)
	}

	if !memberExists {
		return response.NotFound(fmt.Errorf("Cluster member %q not found on any service", name))
	}

	return response.EmptySyncResponse
}
