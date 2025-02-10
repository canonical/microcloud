package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/database"
	"github.com/canonical/microcloud/microcloud/service"
)

// ClusterManagerCmd represents the manage cluster manager configuration.
var ClusterManagerCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		Path: "cluster-manager",

		Get:    rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerGet(sh))},
		Post:   rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerPost(sh))},
		Put:    rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerPut)},
		Delete: rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerDelete(sh))},
	}
}

// swagger:operation GET /1.0/cluster-manager
//
//	Get cluster manager configuration
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func clusterManagerGet(sh *service.Handler) func(state state.State, r *http.Request) response.Response {
	return func(state state.State, r *http.Request) response.Response {
		var clusterManager *database.ClusterManager
		var err error

		err = state.Database().Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
			clusterManagerId, err := getClusterManagerId(state)
			if err != nil {
				return err
			}

			clusterManager, err = database.GetClusterManager(ctx, tx, clusterManagerId)
			if err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			return response.InternalError(err)
		}

		if clusterManager.Addresses == "" {
			return response.SyncResponse(true, types.ClusterManager{})
		}

		cloud := sh.Services[types.MicroCloud].(*service.CloudService)
		certInfo, err := cloud.ServerCert()
		if err != nil {
			return response.InternalError(err)
		}

		resp := types.ClusterManager{
			ClusterManagerAddresses: []string{clusterManager.Addresses},
			LocalCertFingerprint:    certInfo.Fingerprint(),
			ServerCertFingerprint:   clusterManager.ServerCertFingerprint,
		}

		return response.SyncResponse(true, resp)
	}
}

// swagger:operation POST /1.0/cluster-manager token
//
//	Configure cluster manager
//
//	Join a remote cluster manager with a token.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    token: string
//	    required: true
//	    schema:
//	      $ref: "#/definitions/ClusterManagerPost"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"

func clusterManagerPost(sh *service.Handler) func(state state.State, r *http.Request) response.Response {
	return func(state state.State, r *http.Request) response.Response {
		args := types.ClusterManagerPost{}
		err := json.NewDecoder(r.Body).Decode(&args)
		if err != nil {
			return response.BadRequest(err)
		}

		if args.Token == "" {
			return response.BadRequest(fmt.Errorf("No token provided"))
		}

		joinToken, err := shared.JoinTokenDecode(args.Token)
		if err != nil {
			return response.BadRequest(err)
		}

		clusterManager := database.ClusterManager{
			Addresses:             strings.Join(joinToken.Addresses, ","),
			ServerCertFingerprint: joinToken.Fingerprint,
			UpdateIntervalSeconds: 60,
		}

		err = state.Database().Transaction(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
			clusterManagerId, err := getClusterManagerId(state)
			if err != nil {
				return err
			}

			if clusterManagerId > 0 {
				return fmt.Errorf("Cluster manager already configured.")
			}

			_, err = database.CreateClusterManager(ctx, tx, clusterManager)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return response.InternalError(err)
		}

		err = doPostJoinClusterManager(sh, joinToken)
		if err != nil {
			return response.InternalError(err)
		}

		return response.SyncResponse(true, nil)
	}
}

func doPostJoinClusterManager(sh *service.Handler, joinToken *api.ClusterMemberJoinToken) error {
	client, publicKey := NewClusterManagerClient(sh, joinToken.Fingerprint)

	payload := ClusterManagerPostCluster{
		ClusterName:        joinToken.ServerName,
		ClusterCertificate: publicKey,
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := "https://" + joinToken.Addresses[0] + "/1.0/remote-cluster" // todo we should retry with the other addresses if this one fails
	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	mac := hmac.New(sha256.New, []byte(joinToken.Secret))
	mac.Write(reqBody)
	req.Header.Set("X-CLUSTER-SIGNATURE", base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	body := new(bytes.Buffer)
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to register in cluster manager: %s %s", resp.Status, body.String())
	}

	return nil
}

// swagger:operation PUT /1.0/cluster-manager :key :value
//
//	Update cluster manager configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    token: string
//	    required: true
//	    schema:
//	      $ref: "#/definitions/ClusterManagerPut"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"

func clusterManagerPut(state state.State, r *http.Request) response.Response {
	args := types.ClusterManagerPut{}
	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		return response.BadRequest(err)
	}

	// Get the cluster manager addresses
	var clusterManager *database.ClusterManager

	err = state.Database().Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		clusterManagerId, err := getClusterManagerId(state)
		if err != nil {
			return err
		}

		clusterManager, err = database.GetClusterManager(ctx, tx, clusterManagerId)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		logger.Error("Failed to get cluster manager", logger.Ctx{"err": err})
		return response.InternalError(err)
	}

	if args.ClusterManagerAddresses != nil {
		clusterManager.Addresses = strings.Join(args.ClusterManagerAddresses, ",")
	}

	if args.ServerCertFingerprint != "" {
		clusterManager.ServerCertFingerprint = args.ServerCertFingerprint
	}

	err = state.Database().Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		clusterManagerId, err := getClusterManagerId(state)
		if err != nil {
			return err
		}

		err = database.UpdateClusterManager(ctx, tx, clusterManagerId, *clusterManager)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.InternalError(err)
	}

	return response.SyncResponse(true, nil)
}

// ClusterManagerPostCluster represents the payload when sending a POST request to cluster manager.
type ClusterManagerPostCluster struct {
	ClusterName        string `json:"cluster_name" yaml:"cluster_name"`
	ClusterCertificate string `json:"cluster_certificate" yaml:"cluster_certificate"`
}

// NewClusterManagerClient returns a cluster manager client.
func NewClusterManagerClient(sh *service.Handler, serverFingerPrint string) (*http.Client, string) {
	client := &http.Client{}

	// get local cert
	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	localCert, err := cloud.ServerCert()
	if err != nil {
		logger.Error("Failed to get server certificate", logger.Ctx{"err": err})
		return nil, ""
	}

	cert := localCert.KeyPair()
	publicKey := string(localCert.PublicKey())

	tlsConfig := shared.InitTLSConfig()

	tlsConfig.GetClientCertificate = func(info *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		// GetClientCertificate is called if not nil instead of performing the default selection of an appropriate
		// certificate from the `Certificates` list. We only have one-key pair to send, and we always want to send it
		// because this is what uniquely identifies the caller to the server.
		return &cert, nil
	}

	// the server certificate is not signed by a CA, so we need to skip verification
	// we do validate it by checking the fingerprint with VerifyPeerCertificate
	//tlsConfig.InsecureSkipVerify = true // todo: this is not secure, can we use the ca cert to verify the server cert?
	tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		// Extract the certificate
		if len(rawCerts) == 0 {
			return fmt.Errorf("No server certificate provided")
		}

		cert := rawCerts[0]

		// Calculate the fingerprint
		h := sha256.New()
		h.Write(cert)
		actualFingerprint := hex.EncodeToString(h.Sum(nil))

		// Compare with the expected fingerprint
		if !strings.EqualFold(actualFingerprint, serverFingerPrint) {
			return fmt.Errorf("Unexpected certificate fingerprint: %s, expected: %s", actualFingerprint, serverFingerPrint)
		}

		return nil
	}

	client.Transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return client, publicKey
}

// swagger:operation DELETE /1.0/cluster-manager
//
//	Delete cluster manager configuration
//
//	Remove this cluster from cluster manager
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func clusterManagerDelete(sh *service.Handler) func(state state.State, _ *http.Request) response.Response {
	return func(state state.State, _ *http.Request) response.Response {
		var clusterManager *database.ClusterManager

		err := state.Database().Transaction(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
			var err error
			clusterManagerId, err := getClusterManagerId(state)
			if err != nil {
				return err
			}

			clusterManager, err = database.GetClusterManager(ctx, tx, clusterManagerId)
			if err != nil {
				return err
			}

			err = database.DeleteClusterManager(ctx, tx, clusterManagerId)
			if err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			return response.InternalError(err)
		}

		serverCert := clusterManager.ServerCertFingerprint

		logger.Error("Deleting cluster manager configuration", logger.Ctx{"serverCert": serverCert, "addresses": clusterManager.Addresses})
		if serverCert == "" {
			logger.Error("No cluster manager certificate configured")
			return response.SyncResponse(true, nil)
		}

		url := "https://" + clusterManager.Addresses + "/1.0/remote-cluster" // todo we should retry with the other addresses if this one fails
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			return response.InternalError(err)
		}

		client, _ := NewClusterManagerClient(sh, serverCert)
		resp, err := client.Do(req)
		if err != nil {
			return response.InternalError(err)
		}

		if resp.StatusCode != http.StatusOK {
			return response.InternalError(fmt.Errorf("Invalid status code received from cluster manager: %s", resp.Status))
		}

		certFilename := filepath.Join(state.FileSystem().StateDir, "cluster-manager.crt")
		keyFilename := filepath.Join(state.FileSystem().StateDir, "cluster-manager.key")
		if shared.PathExists(certFilename) {
			err := os.Remove(certFilename)
			if err != nil {
				return nil
			}
		}
		if shared.PathExists(keyFilename) {
			err := os.Remove(keyFilename)
			if err != nil {
				return nil
			}
		}
		return response.SyncResponse(true, nil)
	}
}

func getClusterManagerId(state state.State) (int64, error) {
	var maxId int64

	err := state.Database().Transaction(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		managers, err := database.GetClusterManagers(ctx, tx)
		if err != nil {
			return err
		}

		// get max id from cluster managers
		for _, manager := range managers {
			if manager.ID > maxId {
				maxId = manager.ID
			}
		}

		return nil
	})
	if err != nil {
		return -1, err
	}

	return maxId, nil
}

// StatusDistribution represents the status distribution of items.
type StatusDistribution struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// ClusterManagerStatusPost represents the status message sent to cluster manager.
type ClusterManagerStatusPost struct {
	CPUTotalCount     int64                `json:"cpu_total_count"`
	CPULoad1          string               `json:"cpu_load_1"`
	CPULoad5          string               `json:"cpu_load_5"`
	CPULoad15         string               `json:"cpu_load_15"`
	MemoryTotalAmount int64                `json:"memory_total_amount"`
	MemoryUsage       int64                `json:"memory_usage"`
	DiskTotalSize     int64                `json:"disk_total_size"`
	DiskUsage         int64                `json:"disk_usage"`
	MemberStatuses    []StatusDistribution `json:"member_statuses"`
	InstanceStatuses  []StatusDistribution `json:"instance_status"`
	Metrics           string               `json:"metrics"`
}

func sendClusterManagerStatusMessage(ctx context.Context, sh *service.Handler, s state.State) {
	logger.Debug("Running sendClusterManagerStatusMessage")

	// Get the cluster manager addresses
	var clusterManager *database.ClusterManager
	var err error

	err = s.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		clusterManagerId, err := getClusterManagerId(s)
		if err != nil {
			return err
		}

		clusterManager, err = database.GetClusterManager(ctx, tx, clusterManagerId)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		logger.Error("Failed to get cluster manager", logger.Ctx{"err": err})
		return
	}

	if len(clusterManager.Addresses) == 0 {
		logger.Debug("No cluster manager address configured")
		return
	}

	serverCert := clusterManager.ServerCertFingerprint

	if serverCert == "" {
		logger.Debug("No cluster manager certificate configured")
		return
	}

	client, _ := NewClusterManagerClient(sh, serverCert)

	payload := ClusterManagerStatusPost{}

	err = enrichClusterMemberMetrics(sh, &payload)
	if err != nil {
		logger.Error("Failed to enrich cluster member metrics", logger.Ctx{"err": err})
		return
	}

	err = enrichInstanceMetrics(sh, &payload)
	if err != nil {
		logger.Error("Failed to enrich instance metrics", logger.Ctx{"err": err})
		return
	}

	err = enrichServerMetrics(sh, &payload)
	if err != nil {
		logger.Error("Failed to enrich cluster member metrics", logger.Ctx{"err": err})
		return
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal status message", logger.Ctx{"err": err})
		return
	}

	logger.Debug("Sending status message to cluster manager", logger.Ctx{"reqBody": string(reqBody)})

	url := "https://" + clusterManager.Addresses + "/1.0/remote-cluster/status" // todo we should retry with the other addresses if this one fails
	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		logger.Error("Failed to create request", logger.Ctx{"err": err})
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to send status message to cluster manager", logger.Ctx{"err": err})
		return
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("Invalid status code received from cluster manager", logger.Ctx{"status": resp.Status})
		return
	}

	logger.Debug("Done sending status message to cluster manager")
}

func enrichInstanceMetrics(sh *service.Handler, result *ClusterManagerStatusPost) error {
	instanceFrequencies := make(map[string]int64)

	lxd := sh.Services[types.LXD].(*service.LXDService)
	lxdClient, err := lxd.Client(context.Background())

	if err != nil {
		return fmt.Errorf("Failed to get LXD client: %w", err)
	}

	instanceList, err := lxdClient.GetInstances(api.InstanceTypeAny)
	for i := range instanceList {
		inst := instanceList[i]
		instanceFrequencies[inst.Status]++
	}

	for status, count := range instanceFrequencies {
		result.InstanceStatuses = append(result.InstanceStatuses, StatusDistribution{
			Status: status,
			Count:  count,
		})
	}

	return err
}

func enrichServerMetrics(sh *service.Handler, result *ClusterManagerStatusPost) error {
	lxd := sh.Services[types.LXD].(*service.LXDService)
	lxdClient, err := lxd.Client(context.Background())

	if err != nil {
		return fmt.Errorf("Failed to get LXD client: %w", err)
	}

	metrics, err := lxdClient.GetMetrics()
	if err != nil {
		return fmt.Errorf("Failed to get LXD metrics: %w", err)
	}

	result.Metrics = metrics

	return nil
}

func enrichClusterMemberMetrics(sh *service.Handler, result *ClusterManagerStatusPost) error {
	var err error

	lxd := sh.Services[types.LXD].(*service.LXDService)
	lxdClient, err := lxd.Client(context.Background())

	if err != nil {
		return fmt.Errorf("Failed to get LXD client: %w", err)
	}

	lxdMembers, err := lxdClient.GetClusterMembers()

	if err != nil {
		return fmt.Errorf("Failed to get LXD cluster members: %w", err)
	}

	var cpuLoad1 float64
	var cpuLoad5 float64
	var cpuLoad15 float64
	statusFrequencies := make(map[string]int64)
	for i := range lxdMembers {
		member := lxdMembers[i]

		statusFrequencies[member.Status]++
		memberState, _, err := lxdClient.GetClusterMemberState(member.ServerName)
		if err != nil {
			return err
		}

		result.MemoryTotalAmount += int64(memberState.SysInfo.TotalRAM)
		result.MemoryUsage += int64(memberState.SysInfo.TotalRAM - memberState.SysInfo.FreeRAM)

		cpuLoad1 += memberState.SysInfo.LoadAverages[0]
		cpuLoad5 += memberState.SysInfo.LoadAverages[1]
		cpuLoad15 += memberState.SysInfo.LoadAverages[2]

		for _, poolsState := range memberState.StoragePools {
			result.DiskTotalSize += int64(poolsState.Space.Total)
			result.DiskUsage += int64(poolsState.Space.Used)
		}
	}

	for status, count := range statusFrequencies {
		result.MemberStatuses = append(result.MemberStatuses, StatusDistribution{
			Status: status,
			Count:  count,
		})
	}

	if result.CPUTotalCount > 0 {
		result.CPULoad1 = fmt.Sprintf("%.2f", cpuLoad1/float64(result.CPUTotalCount))
		result.CPULoad5 = fmt.Sprintf("%.2f", cpuLoad5/float64(result.CPUTotalCount))
		result.CPULoad15 = fmt.Sprintf("%.2f", cpuLoad15/float64(result.CPUTotalCount))
	} else {
		result.CPULoad1 = fmt.Sprintf("%.2f", cpuLoad1)
		result.CPULoad5 = fmt.Sprintf("%.2f", cpuLoad5)
		result.CPULoad15 = fmt.Sprintf("%.2f", cpuLoad15)
	}

	return nil
}

// SendClusterManagerStatusMessageTask returns a function that sends the cluster manager status message.
func SendClusterManagerStatusMessageTask(ctx context.Context, sh *service.Handler, s state.State) {
	go func(ctx context.Context, sh *service.Handler, s state.State) {
		for {
			time.Sleep(30 * time.Second)
			sendClusterManagerStatusMessage(ctx, sh, s)
		}
	}(ctx, sh, s)
}
