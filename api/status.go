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
	cephClient "github.com/canonical/microceph/microceph/client"
	microClient "github.com/canonical/microcluster/v2/client"
	"github.com/canonical/microcluster/v2/rest"
	microTypes "github.com/canonical/microcluster/v2/rest/types"
	"github.com/canonical/microcluster/v2/state"
	ovnTypes "github.com/canonical/microovn/microovn/api/types"
	ovnClient "github.com/canonical/microovn/microovn/client"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/component"
)

// StatusCmd represents the /1.0/status API on MicroCloud.
var StatusCmd = func(sh *component.Handler) rest.Endpoint {
	return rest.Endpoint{
		Name: "status",
		Path: "status",

		Get: rest.EndpointAction{Handler: statusGet(sh), ProxyTarget: true},
	}
}

func statusGet(sh *component.Handler) endpointHandler {
	// statusMu is used to synchronize map writes to the returned status information, as we populate cluster members for each component concurrently.
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
			Name:           s.Name(),
			Address:        address,
			Clusters:       make(map[types.ComponentType][]microTypes.ClusterMember, len(sh.Components)),
			OSDs:           []cephTypes.Disk{},
			CephComponents: []cephTypes.Service{},
			OVNComponents:  []ovnTypes.Service{},
		}

		err = sh.RunConcurrent("", "", func(s component.Component) error {
			switch s.Type() {
			case types.LXD:
				clusterMembers, err := lxdStatus(r.Context(), s)
				if err != nil {
					logger.Error("Failed to get component status", logger.Ctx{"type": s.Type(), "name": sh.Name})
				}

				statusMu.Lock()
				status.Clusters[s.Type()] = clusterMembers
				statusMu.Unlock()
			case types.MicroCeph:
				clusterMembers, osds, cephComponents, err := cephStatus(r.Context(), s)
				if err != nil {
					logger.Error("Failed to get component status", logger.Ctx{"type": s.Type(), "name": sh.Name})
				}

				status.OSDs = osds
				status.CephComponents = cephComponents

				statusMu.Lock()
				status.Clusters[s.Type()] = clusterMembers
				statusMu.Unlock()
			case types.MicroOVN:
				clusterMembers, ovnComponents, err := ovnStatus(r.Context(), s)
				if err != nil {
					logger.Error("Failed to get component status", logger.Ctx{"type": s.Type(), "name": sh.Name})
				}

				status.OVNComponents = ovnComponents

				statusMu.Lock()
				status.Clusters[s.Type()] = clusterMembers
				statusMu.Unlock()
			case types.MicroCloud:
				microClient, err := s.(*component.CloudComponent).Client()
				if err != nil {
					return err
				}

				clusterMembers, err := microStatus(r.Context(), microClient, s)
				if err != nil {
					logger.Error("Failed to get component status", logger.Ctx{"type": s.Type(), "name": sh.Name})
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

func cephStatus(ctx context.Context, s component.Component) (clusterMembers []microTypes.ClusterMember, osds []cephTypes.Disk, cephComponents []cephTypes.Service, err error) {
	microClient, err := s.(*component.CephComponent).Client("")
	if err != nil {
		return nil, nil, nil, err
	}

	clusterMembers, err = microStatus(ctx, microClient, s)
	if err != nil {
		return nil, nil, nil, err
	}

	disks, err := cephClient.GetDisks(ctx, microClient)
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

	components, err := cephClient.GetServices(ctx, microClient)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, component := range components {
		if component.Location == s.Name() {
			if cephComponents == nil {
				cephComponents = []cephTypes.Service{}
			}

			cephComponents = append(cephComponents, component)
		}
	}

	return clusterMembers, osds, cephComponents, nil
}

func ovnStatus(ctx context.Context, s component.Component) (clusterMembers []microTypes.ClusterMember, ovnComponents []ovnTypes.Service, err error) {
	microClient, err := s.(*component.OVNComponent).Client()
	if err != nil {
		return nil, nil, err
	}

	clusterMembers, err = microStatus(ctx, microClient, s)
	if err != nil {
		return nil, nil, err
	}

	components, err := ovnClient.GetServices(ctx, microClient)
	if err != nil {
		return nil, nil, err
	}

	for _, component := range components {
		if component.Location == s.Name() {
			if ovnComponents == nil {
				ovnComponents = []ovnTypes.Service{}
			}

			ovnComponents = append(ovnComponents, component)
		}
	}

	return clusterMembers, ovnComponents, nil
}

func microStatus(ctx context.Context, microClient *microClient.Client, s component.Component) ([]microTypes.ClusterMember, error) {
	clusterMembers, err := microClient.GetClusterMembers(context.Background())
	if err != nil && !lxdAPI.StatusErrorCheck(err, http.StatusServiceUnavailable) {
		return nil, err
	}

	return clusterMembers, nil
}

func lxdStatus(ctx context.Context, s component.Component) ([]microTypes.ClusterMember, error) {
	lxdClient, err := s.(*component.LXDComponent).Client(ctx)
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

			addrPort, err := microTypes.ParseAddrPort(util.CanonicalNetworkAddress(url.Host, component.LXDPort))
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
