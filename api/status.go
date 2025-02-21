package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/util"
	lxdAPI "github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	microClient "github.com/canonical/microcluster/v2/client"
	"github.com/canonical/microcluster/v2/rest"
	microTypes "github.com/canonical/microcluster/v2/rest/types"
	"github.com/canonical/microcluster/v2/state"
	ovnTypes "github.com/canonical/microovn/microovn/api/types"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/service"
)

// StatusCmd represents the /1.0/status API on MicroCloud.
var StatusCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		Name: "status",
		Path: "status",

		Get: rest.EndpointAction{Handler: statusGet(sh), ProxyTarget: true},
	}
}

func statusGet(sh *service.Handler) endpointHandler {
	// statusMu is used to synchronize map writes to the returned status information, as we populate cluster members for each service concurrently.
	var statusMu sync.Mutex

	return func(s state.State, r *http.Request) response.Response {
		statuses := []types.Status{}

		if !microClient.IsNotification(r) {
			cluster, err := s.Cluster(true)
			if err != nil {
				return response.SmartError(err)
			}

			err = cluster.Query(r.Context(), true, func(ctx context.Context, c *microClient.Client) error {
				memberStatuses, err := client.GetStatus(ctx, c)
				if err != nil {
					logger.Error("Failed to get status for cluster member", logger.Ctx{"error": err, "address": c.URL()})

					return nil
				}

				statuses = append(statuses, memberStatuses...)

				return nil
			})
			if err != nil {
				return response.SmartError(err)
			}
		}

		var address string
		addrPort, err := microTypes.ParseAddrPort(s.Address().URL.Host)
		if err != nil {
			return response.SmartError(fmt.Errorf("Failed to parse MicroCloud listen address: %w", err))
		}

		// The address may be empty if we haven't initialized MicroCloud yet.
		address = addrPort.String()
		if address != "" {
			address = addrPort.Addr().String()
		}

		status := &types.Status{
			Name:         s.Name(),
			Address:      address,
			Clusters:     make(map[types.ServiceType][]microTypes.ClusterMember, len(sh.Services)),
			OSDs:         []cephTypes.Disk{},
			CephServices: []cephTypes.Service{},
			OVNServices:  []ovnTypes.Service{},
		}

		err = sh.RunConcurrent("", "", func(s service.Service) error {
			switch s.Type() {
			case types.LXD:
				clusterMembers, err := lxdStatus(r.Context(), s)
				if err != nil {
					logger.Error("Failed to get service status", logger.Ctx{"type": s.Type(), "name": sh.Name})
				}

				statusMu.Lock()
				status.Clusters[s.Type()] = clusterMembers
				statusMu.Unlock()
			case types.MicroCeph:
				clusterMembers, osds, cephServices, err := cephStatus(r.Context(), s)
				if err != nil {
					logger.Error("Failed to get service status", logger.Ctx{"type": s.Type(), "name": sh.Name})
				}

				status.OSDs = osds
				status.CephServices = cephServices

				statusMu.Lock()
				status.Clusters[s.Type()] = clusterMembers
				statusMu.Unlock()
			case types.MicroOVN:
				clusterMembers, ovnServices, err := ovnStatus(r.Context(), s)
				if err != nil {
					logger.Error("Failed to get service status", logger.Ctx{"type": s.Type(), "name": sh.Name})
				}

				status.OVNServices = ovnServices

				statusMu.Lock()
				status.Clusters[s.Type()] = clusterMembers
				statusMu.Unlock()
			case types.MicroCloud:
				microClient, err := s.(*service.CloudService).Client()
				if err != nil {
					return err
				}

				clusterMembers, err := microStatus(r.Context(), microClient, s)
				if err != nil {
					logger.Error("Failed to get service status", logger.Ctx{"type": s.Type(), "name": sh.Name})
				}

				statusMu.Lock()
				status.Clusters[s.Type()] = clusterMembers
				statusMu.Unlock()
			}

			return nil
		})
		if err != nil {
			return response.SmartError(err)
		}

		statuses = append(statuses, *status)

		return response.SyncResponse(true, statuses)
	}
}

func cephStatus(ctx context.Context, s service.Service) (clusterMembers []microTypes.ClusterMember, osds []cephTypes.Disk, cephServices []cephTypes.Service, err error) {
	cephService := s.(*service.CephService)

	microClient, err := cephService.Client("")
	if err != nil {
		return nil, nil, nil, err
	}

	clusterMembers, err = microStatus(ctx, microClient, s)
	if err != nil {
		return nil, nil, nil, err
	}

	disks, err := cephService.GetDisks(ctx, "")
	if err != nil {
		return nil, nil, nil, err
	}

	for _, disk := range disks {
		if disk.Location == s.Name() {
			if osds == nil {
				osds = []cephTypes.Disk{}
			}

			osds = append(osds, disk)
		}
	}

	services, err := cephService.GetServices(ctx, "")
	if err != nil {
		return nil, nil, nil, err
	}

	for _, service := range services {
		if service.Location == s.Name() {
			if cephServices == nil {
				cephServices = []cephTypes.Service{}
			}

			cephServices = append(cephServices, service)
		}
	}

	return clusterMembers, osds, cephServices, nil
}

func ovnStatus(ctx context.Context, s service.Service) (clusterMembers []microTypes.ClusterMember, ovnServices []ovnTypes.Service, err error) {
	serviceOVN := s.(*service.OVNService)

	microClient, err := serviceOVN.Client()
	if err != nil {
		return nil, nil, err
	}

	clusterMembers, err = microStatus(ctx, microClient, s)
	if err != nil {
		return nil, nil, err
	}

	services, err := serviceOVN.GetServices(ctx)
	if err != nil {
		return nil, nil, err
	}

	for _, service := range services {
		if service.Location == s.Name() {
			if ovnServices == nil {
				ovnServices = []ovnTypes.Service{}
			}

			ovnServices = append(ovnServices, service)
		}
	}

	return clusterMembers, ovnServices, nil
}

func microStatus(ctx context.Context, microClient *microClient.Client, s service.Service) ([]microTypes.ClusterMember, error) {
	clusterMembers, err := microClient.GetClusterMembers(context.Background())
	if err != nil && !lxdAPI.StatusErrorCheck(err, http.StatusServiceUnavailable) {
		return nil, err
	}

	return clusterMembers, nil
}

func lxdStatus(ctx context.Context, s service.Service) ([]microTypes.ClusterMember, error) {
	lxdClient, err := s.(*service.LXDService).Client(ctx)
	if err != nil {
		return nil, err
	}

	server, _, err := lxdClient.GetServer()
	if err != nil {
		return nil, err
	}

	var microMembers []microTypes.ClusterMember
	if server.Environment.ServerClustered {
		clusterMembers, err := lxdClient.GetClusterMembers()
		if err != nil {
			return nil, err
		}

		certs, err := lxdClient.GetCertificates()
		if err != nil {
			return nil, err
		}

		microMembers = make([]microTypes.ClusterMember, 0, len(clusterMembers))
		for _, member := range clusterMembers {
			url, err := url.Parse(member.URL)
			if err != nil {
				return nil, err
			}

			addrPort, err := microTypes.ParseAddrPort(util.CanonicalNetworkAddress(url.Host, service.LXDPort))
			if err != nil {
				return nil, err
			}

			// Microcluster requires a certificate to be specified in types.ClusterMemberLocal.
			var serverCert *microTypes.X509Certificate
			for _, cert := range certs {
				if cert.Type == "server" && cert.Name == member.ServerName {
					serverCert, err = microTypes.ParseX509Certificate(cert.Certificate)
					if err != nil {
						return nil, err
					}
				}
			}

			microMember := microTypes.ClusterMember{
				ClusterMemberLocal: microTypes.ClusterMemberLocal{
					Name:        member.ServerName,
					Address:     addrPort,
					Certificate: *serverCert,
				},
				Role:       strings.Join(member.Roles, ","),
				Status:     microTypes.MemberStatus(member.Status),
				Extensions: []string{},
			}

			// If the status is Online, use the microcluster representation, all other cluster states will be considered invalid and be treated like an offline state.
			if member.Status == "Online" {
				microMember.Status = microTypes.MemberOnline
			}

			microMembers = append(microMembers, microMember)
		}
	}

	return microMembers, nil
}
