package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/revert"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/state"
	"github.com/gorilla/mux"
	"gopkg.in/yaml.v2"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// AppsCmd handles /1.0/apps.
var AppsCmd = rest.Endpoint{
	Name: "apps",
	Path: "apps",
	Get:  rest.EndpointAction{Handler: appsGet},
}

// Microk8sCmd handles /1.0/apps/microk8s.
var Microk8sCmd = rest.Endpoint{
	Name: "microk8s",
	Path: "apps/microk8s",
	Get:  rest.EndpointAction{Handler: microk8sGet},
	Post: rest.EndpointAction{Handler: microk8sPost},
}

// Microk8sClusterCmd handles /1.0/apps/microk8s/{clusterName}.
var Microk8sClusterCmd = rest.Endpoint{
	Name:   "app",
	Path:   "apps/microk8s/{clusterName}",
	Get:    rest.EndpointAction{Handler: microk8sClusterGet},
	Delete: rest.EndpointAction{Handler: microk8sClusterDelete},
}

func appsGet(s *state.State, r *http.Request) response.Response {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Malformed request URL: %w", err))
	}

	project := "default"
	if values.Has("project") {
		project = values.Get("project")
	}

	apps, err := listApps(project, "")
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, apps)
}

func microk8sGet(s *state.State, r *http.Request) response.Response {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Malformed request URL: %w", err))
	}

	project := "default"
	if values.Has("project") {
		project = values.Get("project")
	}

	apps, err := listApps(project, types.AppKindMicrok8s)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, apps)
}

func microk8sPost(s *state.State, r *http.Request) response.Response {
	var config types.Microk8sPost
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Malformatted request body: %w", err))
	}

	if config.ClusterName == "" {
		return response.BadRequest(fmt.Errorf("You must provide a cluster name"))
	}

	projectName := "default"
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Malformed URL: %w", err))
	}

	if values.Has("project") {
		projectName = values.Get("project")
	}

	if len(config.Profiles) == 0 {
		config.Profiles = []string{"default"}
	}

	if config.Channel == "" {
		config.Channel = "latest/stable"
	}

	// Default to 3 nodes for HA. Otherwise, double-check the size is not silly.
	if config.Size == 0 {
		config.Size = 3
	} else if config.Size < 3 {
		return response.BadRequest(errors.New("Cannot create a microk8s cluster of size < 3"))
	} else if config.Size > 40 {
		return response.BadRequest(errors.New("Cannot create a microk8s cluster of size > 40"))
	}

	instanceServer, err := lxdClient()
	if err != nil {
		return response.SmartError(err)
	}

	go func() {
		// Create the cloud init config for the first node.
		initialNodeCloudInit, joinTokens, err := initialMicrok8sNodeCloudInit(config)
		if err != nil {
			logger.Error("Failed to create initial microk8s node", logger.Ctx{"error": err})
			return
		}

		// Create the initial node.
		instanceServer = instanceServer.UseProject(projectName)
		initialNodeName := fmt.Sprintf("%s-node01", config.ClusterName)
		initialNodePost := microk8sInstancesPost(initialNodeName, initialNodeCloudInit, config)

		reverter := revert.New()
		defer reverter.Fail()

		// Create the initial node.
		op, err := instanceServer.CreateInstance(initialNodePost)
		if err != nil {
			logger.Error("Failed to create microk8s cluster initial node", logger.Ctx{"error": err})
			return
		}

		reverter.Add(func() {
			op, err := instanceServer.DeleteInstance(initialNodeName)
			if err != nil {
				logger.Error("Failed to revert creation of initial microk8s node", logger.Ctx{"error": err})
				return
			}

			err = op.Wait()
			if err != nil {
				logger.Error("Failed to revert creation of initial microk8s node", logger.Ctx{"error": err})
			}
		})

		err = op.Wait()
		if err != nil {
			logger.Error("Failed to create microk8s cluster initial node", logger.Ctx{"error": err})
			return
		}

		// Start the first node.
		op, err = instanceServer.UpdateInstanceState(initialNodeName, api.InstanceStatePut{
			Action:  "start",
			Timeout: -1,
		}, "")
		if err != nil {
			logger.Error("Failed to start microk8s cluster initial node", logger.Ctx{"error": err})
			return
		}

		err = op.Wait()
		if err != nil {
			logger.Error("Failed to start microk8s cluster initial node", logger.Ctx{"error": err})
			return
		}

		// Wait for microk8s to install (unlikely this will take less than 30 seconds).
		time.Sleep(30 * time.Second)

		// Poll the instance until it's status is set to "READY".
		var initialNodeState *api.InstanceState
		for {
			initialNodeState, _, err = instanceServer.GetInstanceState(initialNodeName)
			if err != nil {
				logger.Error("Failed to poll initial microk8s node state", logger.Ctx{"error": err})
				return
			}

			if initialNodeState.Status == "Ready" {
				break
			}

			time.Sleep(10 * time.Second)
		}

		enp5s0, ok := initialNodeState.Network["enp5s0"]
		if !ok {
			logger.Error("Failed to detect address of initial microk8s node")
			return
		}

		var initialNodeAddress string
		for _, address := range enp5s0.Addresses {
			if address.Family == "inet" && address.Scope == "global" {
				initialNodeAddress = address.Address
			}
		}

		if initialNodeAddress == "" {
			logger.Error("Failed to detect address of initial microk8s node")
			return
		}

		for i, joinToken := range joinTokens {
			joiningNodeCloudInit, err := joiningMicrok8sNodeCloudInit(config.Channel, initialNodeAddress, joinToken)
			if err != nil {
				logger.Error("Failed to create joining microk8s node", logger.Ctx{"error": err})
				return
			}

			joiningNodeName := fmt.Sprintf("%s-node%02d", config.ClusterName, i+2)
			joiningNodeInstancesPost := microk8sInstancesPost(joiningNodeName, joiningNodeCloudInit, config)

			op, err := instanceServer.CreateInstance(joiningNodeInstancesPost)
			if err != nil {
				logger.Error("Failed to create joining microk8s node", logger.Ctx{"error": err})
				return
			}

			reverter.Add(func() {
				op, err := instanceServer.DeleteInstance(joiningNodeName)
				if err != nil {
					logger.Error("Failed to revert creation of initial microk8s node", logger.Ctx{"error": err})
					return
				}

				err = op.Wait()
				if err != nil {
					logger.Error("Failed to revert creation of initial microk8s node", logger.Ctx{"error": err})
				}
			})

			err = op.Wait()
			if err != nil {
				logger.Error("Failed to create joining microk8s node", logger.Ctx{"error": err})
				return
			}

			op, err = instanceServer.UpdateInstanceState(joiningNodeName, api.InstanceStatePut{
				Action:  "start",
				Timeout: -1,
			}, "")
			if err != nil {
				logger.Error("Failed to start joining microk8s node", logger.Ctx{"error": err})
				return
			}

			err = op.Wait()
			if err != nil {
				logger.Error("Failed to start joining microk8s node", logger.Ctx{"error": err})
				return
			}
		}

		reverter.Success()
	}()

	return response.ManualResponse(func(w http.ResponseWriter) error {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		err := json.NewEncoder(w).Encode(api.ResponseRaw{
			Type:       api.AsyncResponse,
			Status:     "Running",
			StatusCode: http.StatusAccepted,
			Metadata:   fmt.Sprintf("Deploying microk8s cluster %q of size \"%d\" using channel %q", config.ClusterName, config.Size, config.Channel),
		})
		if err != nil {
			return err
		}

		return nil
	})
}

func microk8sClusterGet(s *state.State, r *http.Request) response.Response {
	clusterName, err := url.PathUnescape(mux.Vars(r)["clusterName"])
	if err != nil {
		return response.InternalError(fmt.Errorf("Failed to get path parameter: %w", err))
	}

	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return response.InternalError(fmt.Errorf("Failed to parse query parameters: %w", err))
	}

	projectName := "default"
	if values.Has("project") {
		projectName = values.Get("project")
	}

	instanceServer, err := lxdClient()
	if err != nil {
		return response.SmartError(err)
	}

	instanceServer = instanceServer.UseProject(projectName)
	instances, err := instanceServer.GetInstancesWithFilter(api.InstanceTypeVM, []string{fmt.Sprintf("config.%s=%s", types.MicrocloudAppNameConfigKey, clusterName)})
	if err != nil {
		return response.SmartError(err)
	}

	if len(instances) == 0 {
		return response.NotFound(fmt.Errorf("No cluster found with name %q", clusterName))
	}

	instance := instances[rand.Intn(len(instances))]
	kubeconfig, err := instanceExec(instanceServer, instance.Name, "microk8s", "config")
	if err != nil {
		return response.SmartError(err)
	}

	appInfo := types.AppInfo{
		App: types.App{
			Name:    clusterName,
			Kind:    types.AppKindMicrok8s,
			Project: projectName,
		},
		Instances: instances,
		Metadata: map[string]any{
			"kubeconfig": string(kubeconfig),
		},
	}

	return response.SyncResponse(true, appInfo)
}

func microk8sClusterDelete(s *state.State, r *http.Request) response.Response {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Malformed URL: %w", err))
	}

	projectName := "default"
	if values.Has("project") {
		projectName = values.Get("project")
	}

	force := shared.IsTrue(values.Get("force"))

	clusterName, err := url.PathUnescape(mux.Vars(r)["clusterName"])
	if err != nil {
		return response.InternalError(fmt.Errorf("Failed to get path parameter: %w", err))
	}

	err = deleteApp(clusterName, types.AppKindMicrok8s, projectName, force)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

func listApps(projectName string, appKind string) ([]types.App, error) {
	instanceServer, err := lxdClient()
	if err != nil {
		return nil, err
	}

	instanceServer.UseProject(projectName)
	instances, err := instanceServer.GetInstancesWithFilter(api.InstanceTypeAny, appFilter("", appKind))
	if err != nil {
		return nil, err
	}

	// Use a map to avoid duplicates.
	appMap := make(map[types.App]struct{}, len(instances))
	for _, instance := range instances {
		appMap[types.App{
			Name:    instance.Config[types.MicrocloudAppNameConfigKey],
			Kind:    instance.Config[types.MicrocloudAppKindConfigKey],
			Project: projectName,
		}] = struct{}{}
	}

	// Convert map to slice.
	apps := make([]types.App, len(appMap))
	i := 0
	for app := range appMap {
		apps[i] = app
		i++
	}

	return apps, nil
}

func deleteApp(appName string, appKind string, projectName string, force bool) error {
	switch appKind {
	case types.AppKindMicrok8s:
		break
	default:
		return fmt.Errorf("Invalid app kind %q", appKind)
	}

	instanceServer, err := lxdClient()
	if err != nil {
		return err
	}

	instanceServer.UseProject(projectName)
	instances, err := instanceServer.GetInstancesWithFilter(api.InstanceTypeAny, appFilter(appName, appKind))
	if err != nil {
		return err
	}

	if len(instances) == 0 {
		return api.StatusErrorf(http.StatusNotFound, "No cluster found with name %q", appName)
	}

	for _, instance := range instances {
		if force {
			req := api.InstanceStatePut{
				Action:  "stop",
				Timeout: -1,
				Force:   true,
			}

			op, err := instanceServer.UpdateInstanceState(instance.Name, req, "")
			if err != nil {
				return err
			}

			err = op.Wait()
			if err != nil {
				return err
			}
		}

		op, err := instanceServer.DeleteInstance(instance.Name)
		if err != nil {
			return err
		}

		err = op.Wait()
		if err != nil {
			return err
		}
	}

	return nil
}

func lxdClient() (lxd.InstanceServer, error) {
	instanceServer, err := lxd.ConnectLXDUnix("/var/snap/lxd/common/lxd/unix.socket", nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to get LXD client: %w", err)
	}

	return instanceServer, nil
}

type nopCloser struct {
	*bytes.Buffer
}

// Close is a no-op.
func (nopCloser) Close() error {
	return nil
}

func instanceExec(instanceServer lxd.InstanceServer, instanceName string, command ...string) ([]byte, error) {
	stdout := nopCloser{Buffer: bytes.NewBuffer(nil)}
	stderr := nopCloser{Buffer: bytes.NewBuffer(nil)}
	execArgs := &lxd.InstanceExecArgs{
		Stdout:   stdout,
		Stderr:   stderr,
		DataDone: make(chan bool),
	}

	_, err := instanceServer.ExecInstance(instanceName, api.InstanceExecPost{
		Command:   command,
		WaitForWS: true,
	}, execArgs)
	if err != nil {
		return nil, err
	}

	<-execArgs.DataDone
	if stderr.Len() > 0 {
		cmdErr, err := io.ReadAll(stderr)
		if err != nil {
			return nil, fmt.Errorf("Unexpected error occurred reading stderr output: %w", err)
		}

		return nil, fmt.Errorf("Failed to execute microk8s config command on instance %q: %s", instanceName, string(cmdErr))
	}

	stdoutBytes, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("Unexpected error occurred reading command output: %w", err)
	}

	return stdoutBytes, nil
}

var microk8sWaitReadyCmd = "microk8s status --wait-ready"
var instanceReadyCmd = `curl --unix-socket /dev/lxd/sock lxd/1.0 -X PATCH -d '{"state":"Ready"}'`

func microk8sInstallCmd(channel string) string {
	return fmt.Sprintf("snap install microk8s --classic --channel=%s", channel)
}

func initialMicrok8sNodeCloudInit(config types.Microk8sPost) (cloudInitConfig string, joinTokens []string, err error) {
	cmds := []string{microk8sInstallCmd(config.Channel), microk8sWaitReadyCmd}

	// Create a join token for each joining node.
	joinTokens = make([]string, config.Size-1)
	for i := 0; i < config.Size-1; i++ {
		cryptoString, err := shared.RandomCryptoString()
		if err != nil {
			return "", nil, fmt.Errorf("Could not generate microk8s join token: %w", err)
		}

		// Join tokens are 32 character random strings.
		joinTokens[i] = cryptoString[:32]
	}

	for _, token := range joinTokens {
		cmds = append(cmds, fmt.Sprintf("microk8s add-node -t %s", token))
	}

	cmds = append(cmds, instanceReadyCmd)
	cloudInitConfig, err = generateCloudInitConfig(map[string]any{
		"runcmd": cmds,
	})
	if err != nil {
		return "", nil, fmt.Errorf("Could not generate cloud config for initial node: %w", err)
	}

	return cloudInitConfig, joinTokens, nil
}

func joiningMicrok8sNodeCloudInit(channel string, initialNodeAddress string, joinToken string) (string, error) {
	cloudInitConfig, err := generateCloudInitConfig(map[string]any{
		"runcmd": []string{
			microk8sInstallCmd(channel),
			microk8sWaitReadyCmd,
			fmt.Sprintf("microk8s join %s:25000/%s", initialNodeAddress, joinToken),
			instanceReadyCmd,
		},
	})
	if err != nil {
		return "", fmt.Errorf("Could not generate cloud config for joining node: %w", err)
	}

	return cloudInitConfig, nil
}

func generateCloudInitConfig(config map[string]any) (string, error) {
	buf := bytes.NewBufferString("#cloud-config\n")
	err := yaml.NewEncoder(buf).Encode(config)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func appFilter(name string, kind string) []string {
	filter := []string{fmt.Sprintf("config.%s=true", types.MicrocloudAppConfigKey)}
	if name != "" {
		filter = append(filter, fmt.Sprintf("config.%s=%s", types.MicrocloudAppNameConfigKey, name))
	}

	if kind != "" {
		filter = append(filter, fmt.Sprintf("config.%s=%s", types.MicrocloudAppKindConfigKey, kind))
	}

	return filter
}

func microk8sInstancesPost(name string, cloudInitConfig string, config types.Microk8sPost) api.InstancesPost {
	return api.InstancesPost{
		InstancePut: api.InstancePut{
			Config: map[string]string{
				"cloud-init.user-data":           cloudInitConfig,
				types.MicrocloudAppNameConfigKey: config.ClusterName,
				types.MicrocloudAppKindConfigKey: types.AppKindMicrok8s,
				types.MicrocloudAppConfigKey:     "true",
			},
			Profiles:    config.Profiles,
			Description: "Microk8s node",
		},
		Name: name,
		// For now, only using official jammy images.
		Source: api.InstanceSource{
			Type:     "image",
			Alias:    "jammy",
			Server:   "https://cloud-images.ubuntu.com/releases",
			Protocol: "simplestreams",
			Mode:     "pull",
		},
		Type: "virtual-machine",
	}
}
