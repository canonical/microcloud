package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/microcluster/v2/client"
	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/database"
)

type cmdClusterManger struct {
	common *CmdControl
}

// Command returns the subcommand to manage cluster manager configuration.
func (c *cmdClusterManger) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "cluster-manager"
	cmd.Short = "Manage cluster-manager connection"

	// Join
	clusterManagerJoinCmd := cmdClusterManagerJoin{common: c.common, alias: c}
	cmd.AddCommand(clusterManagerJoinCmd.command())

	// Show
	clusterManagerShowCmd := cmdClusterManagerShow{common: c.common, alias: c}
	cmd.AddCommand(clusterManagerShowCmd.command())

	// Delete
	clusterManagerDeleteCmd := cmdClusterManagerDelete{common: c.common, alias: c}
	cmd.AddCommand(clusterManagerDeleteCmd.command())

	// Get
	clusterManagerGetCmd := cmdClusterManagerGet{common: c.common, alias: c}
	cmd.AddCommand(clusterManagerGetCmd.command())

	// Set
	clusterManagerSetCmd := cmdClusterManagerSet{common: c.common, alias: c}
	cmd.AddCommand(clusterManagerSetCmd.command())

	// Unset
	clusterManagerUnsetCmd := cmdClusterManagerUnset{common: c.common, alias: c}
	cmd.AddCommand(clusterManagerUnsetCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }
	return cmd
}

// Join.
type cmdClusterManagerJoin struct {
	common *CmdControl
	alias  *cmdClusterManger
}

func (c *cmdClusterManagerJoin) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "join <token>"
	cmd.Short = "Join a cluster manager"
	cmd.Example = `lxc cluster-manager join "base64_encoded_token"`

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerJoin) run(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Expected exactly one argument with the token")
	}

	token := args[0]

	if token == "" {
		return fmt.Errorf("Token cannot be empty")
	}

	payload := types.ClusterManagerPost{
		Token: token,
	}

	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	var clusterManager *types.ClusterManager
	err = apiClient.QueryStruct(context.Background(), "POST", "1.0", api.NewURL().Path("cluster-manager"), payload, &clusterManager)
	if err != nil {
		return err
	}

	fmt.Println("Successfully joined cluster manager.")

	return nil
}

// Show.
type cmdClusterManagerShow struct {
	common *CmdControl
	alias  *cmdClusterManger
}

func (c *cmdClusterManagerShow) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show"
	cmd.Short = "Show cluster manager configuration"
	cmd.Example = `lxc cluster-manager show`

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerShow) run(_ *cobra.Command, _ []string) error {
	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	var clusterManager *types.ClusterManager
	err = apiClient.QueryStruct(context.Background(), "GET", "1.0", api.NewURL().Path("cluster-manager"), nil, &clusterManager)
	if err != nil {
		return err
	}

	if len(clusterManager.Addresses) == 0 {
		fmt.Println("cluster manager not configured")
		return nil
	}

	yamlConfig, err := yaml.Marshal(clusterManager)
	if err != nil {
		return err
	}

	fmt.Println(string(yamlConfig))

	return nil
}

// Delete.
type cmdClusterManagerDelete struct {
	common *CmdControl
	alias  *cmdClusterManger
}

func (c *cmdClusterManagerDelete) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "delete"
	cmd.Short = "Clear cluster manager configuration"
	cmd.Example = `lxc cluster-manager delete`

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerDelete) run(_ *cobra.Command, _ []string) error {
	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	var clusterManager *database.ClusterManager
	err = apiClient.QueryStruct(context.Background(), "DELETE", "1.0", api.NewURL().Path("cluster-manager"), nil, &clusterManager)
	if err != nil {
		return err
	}

	fmt.Println("Cluster manager configuration cleared")

	return nil
}

// Get.
type cmdClusterManagerGet struct {
	common *CmdControl
	alias  *cmdClusterManger
}

func (c *cmdClusterManagerGet) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "get"
	cmd.Short = "Get specific cluster manager configuration by key. Available keys are: addresses, fingerprint and update-interval-seconds."
	cmd.Example = `lxc cluster-manager get addresses`

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerGet) run(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Expected exactly one argument with the key.")
	}

	key := args[0]

	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	var clusterManager *types.ClusterManager
	err = apiClient.QueryStruct(context.Background(), "GET", "1.0", api.NewURL().Path("cluster-manager"), nil, &clusterManager)
	if err != nil {
		return err
	}

	if len(clusterManager.Addresses) == 0 {
		fmt.Println("Cluster manager not configured")
		return nil
	}

	switch key {
	case "addresses":
		fmt.Printf("%s\n", strings.Join(clusterManager.Addresses, ", "))
	case "fingerprint":
		fmt.Printf("%s\n", *clusterManager.Fingerprint)
	case "update-interval-seconds":
		fmt.Printf("%s\n", *clusterManager.UpdateInterval)
	default:
		return fmt.Errorf("Invalid key.")
	}

	return nil
}

// Set.
type cmdClusterManagerSet struct {
	common *CmdControl
	alias  *cmdClusterManger
}

func (c *cmdClusterManagerSet) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "set"
	cmd.Short = "Set specific cluster manager configuration key. Available keys are: addresses, fingerprint and update-interval-seconds."
	cmd.Example = `lxc cluster-manager set addresses example.com:8443`

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerSet) run(_ *cobra.Command, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("Expected exactly two arguments: key and value.")
	}

	key := args[0]
	value := args[1]

	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	var clusterManager *types.ClusterManager
	err = apiClient.QueryStruct(context.Background(), "GET", "1.0", api.NewURL().Path("cluster-manager"), nil, &clusterManager)
	if err != nil {
		return err
	}

	if len(clusterManager.Addresses) == 0 {
		return fmt.Errorf("Cluster manager not configured.")
	}

	switch key {
	case "addresses":
		clusterManager.Addresses = []string{value}
	case "fingerprint":
		clusterManager.Fingerprint = &value
	case "update-interval-seconds":
		clusterManager.UpdateInterval = &value
	default:
		return fmt.Errorf("Invalid key.")
	}

	err = apiClient.QueryStruct(context.Background(), "PUT", "1.0", api.NewURL().Path("cluster-manager"), clusterManager, nil)
	if err != nil {
		return err
	}

	return nil
}

// Unset.
type cmdClusterManagerUnset struct {
	common *CmdControl
	alias  *cmdClusterManger
}

func (c *cmdClusterManagerUnset) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "unset"
	cmd.Short = "Unset specific cluster manager configuration key. Available keys are: update-interval-seconds."
	cmd.Example = `lxc cluster-manager unset update-interval-seconds`

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerUnset) run(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Expected exactly on argument with the key.")
	}

	key := args[0]

	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	var clusterManager *types.ClusterManager
	err = apiClient.QueryStruct(context.Background(), "GET", "1.0", api.NewURL().Path("cluster-manager"), nil, &clusterManager)
	if err != nil {
		return err
	}

	if len(clusterManager.Addresses) == 0 {
		return fmt.Errorf("Cluster manager not configured.")
	}

	switch key {
	case "addresses":
		return fmt.Errorf("Cannot unset addresses.")
	case "fingerprint":
		return fmt.Errorf("Cannot unset fingerprint.")
	case "update-interval-seconds":
		emptyString := ""
		clusterManager.UpdateInterval = &emptyString
	default:
		return fmt.Errorf("Invalid key.")
	}

	err = apiClient.QueryStruct(context.Background(), "PUT", "1.0", api.NewURL().Path("cluster-manager"), clusterManager, nil)
	if err != nil {
		return err
	}

	return nil
}

func getApiClient(common *CmdControl) (*client.Client, error) {
	cloudApp, err := microcluster.App(microcluster.Args{StateDir: common.FlagMicroCloudDir})
	if err != nil {
		return nil, err
	}

	apiClient, err := cloudApp.LocalClient()
	if err != nil {
		return nil, err
	}

	return apiClient, nil
}
