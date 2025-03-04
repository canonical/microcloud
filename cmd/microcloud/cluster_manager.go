package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/microcluster/v2/client"
	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/database"
)

type cmdClusterManger struct {
	common *CmdControl
}

// Command is a method of the cmdAlias structure that returns a new cobra Command for managing command aliases.
// This includes commands for adding, listing, renaming, and removing aliases, along with their usage and descriptions.
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

// Command is a method of the cmdAliasAdd structure that returns a new cobra Command for adding new command aliases.
// It specifies the command usage, description, and examples, and links it to the RunE method for execution logic.
func (c *cmdClusterManagerJoin) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "join [<remote>:]<token>"
	cmd.Short = "Join a cluster manager"
	cmd.Example = `lxc cluster-manager join "base64_encoded_token"`

	cmd.RunE = c.run

	return cmd
}

// Run is a method of the cmdAliasAdd structure. It implements the logic to add a new alias command.
// The function checks for valid arguments, verifies if the alias already exists, and if not, adds the new alias to the configuration.
func (c *cmdClusterManagerJoin) run(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Expected exactly one argument")
	}

	token := args[0]

	if token == "" {
		return fmt.Errorf("Missing token")
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

	fmt.Printf("Joined cluster manager with token" + "\n")

	return nil
}

// Show.
type cmdClusterManagerShow struct {
	common *CmdControl
	alias  *cmdClusterManger
}

// Command is a method of the cmdAliasAdd structure that returns a new cobra Command for adding new command aliases.
// It specifies the command usage, description, and examples, and links it to the RunE method for execution logic.
func (c *cmdClusterManagerShow) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show"
	cmd.Short = "Show cluster manager configuration"
	cmd.Example = `lxc cluster-manager show`

	cmd.RunE = c.run

	return cmd
}

// Run is a method of the cmdAliasAdd structure. It implements the logic to add a new alias command.
// The function checks for valid arguments, verifies if the alias already exists, and if not, adds the new alias to the configuration.
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

	if clusterManager.ServerCertFingerprint == "" {
		fmt.Printf("cluster manager not configured" + "\n")
		return nil
	}

	fmt.Printf(
		"cluster manager configuration:\n addresses"+": %s\n local"+": %s\n server"+": %s\n",
		strings.Join(clusterManager.ClusterManagerAddresses, ", "),
		clusterManager.LocalCertFingerprint,
		clusterManager.ServerCertFingerprint,
	)

	return nil
}

// Delete.
type cmdClusterManagerDelete struct {
	common *CmdControl
	alias  *cmdClusterManger
}

// Command is a method of the cmdAliasAdd structure that returns a new cobra Command for adding new command aliases.
// It specifies the command usage, description, and examples, and links it to the RunE method for execution logic.
func (c *cmdClusterManagerDelete) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "delete"
	cmd.Short = "Delete cluster manager configuration"
	cmd.Example = `lxc cluster-manager delete`

	cmd.RunE = c.run

	return cmd
}

// Run is a method of the cmdAliasAdd structure. It implements the logic to add a new alias command.
// The function checks for valid arguments, verifies if the alias already exists, and if not, adds the new alias to the configuration.
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

	fmt.Printf("cluster manager config cleared" + "\n")

	return nil
}

// Get.
type cmdClusterManagerGet struct {
	common *CmdControl
	alias  *cmdClusterManger
}

// Command is a method of the cmdAliasAdd structure that returns a new cobra Command for adding new command aliases.
// It specifies the command usage, description, and examples, and links it to the RunE method for execution logic.
func (c *cmdClusterManagerGet) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "get"
	cmd.Short = "Get specific cluster manager configuration. Available keys are addresses, server-cert-fingerprint and local-cert-fingerprint."
	cmd.Example = `lxc cluster-manager get addresses`

	cmd.RunE = c.run

	return cmd
}

// Run is a method of the cmdAliasAdd structure. It implements the logic to add a new alias command.
func (c *cmdClusterManagerGet) run(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Expected exactly one argument: addresses, server-cert-fingerprint or local-cert-fingerprint.")
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

	if clusterManager.ServerCertFingerprint == "" {
		fmt.Printf("cluster manager not configured" + "\n")
		return nil
	}

	switch key {
	case "addresses":
		fmt.Printf("%s\n", strings.Join(clusterManager.ClusterManagerAddresses, ", "))
	case "server-cert-fingerprint":
		fmt.Printf("%s\n", clusterManager.ServerCertFingerprint)
	case "local-cert-fingerprint":
		fmt.Printf("%s\n", clusterManager.LocalCertFingerprint)
	}

	return nil
}

// Set.
type cmdClusterManagerSet struct {
	common *CmdControl
	alias  *cmdClusterManger
}

// Command is a method of the cmdAliasAdd structure that returns a new cobra Command for adding new command aliases.
// It specifies the command usage, description, and examples, and links it to the RunE method for execution logic.
func (c *cmdClusterManagerSet) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "set"
	cmd.Short = "Set specific cluster manager configuration. Available keys are addresses and servers-cert-fingerprint."
	cmd.Example = `lxc cluster-manager set addresses example.com:8443`

	cmd.RunE = c.run

	return cmd
}

// Run is a method of the cmdAliasAdd structure. It implements the logic to add a new alias command.
func (c *cmdClusterManagerSet) run(_ *cobra.Command, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("Expected exactly two arguments: key and value.")
	}

	key := args[0]
	value := args[0]

	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	var clusterManager *types.ClusterManager
	err = apiClient.QueryStruct(context.Background(), "GET", "1.0", api.NewURL().Path("cluster-manager"), nil, &clusterManager)
	if err != nil {
		return err
	}

	if clusterManager.ServerCertFingerprint == "" {
		return fmt.Errorf("Cluster manager not configured.")
	}

	switch key {
	case "addresses":
		clusterManager.ClusterManagerAddresses = []string{value}
	case "server-cert-fingerprint":
		clusterManager.ServerCertFingerprint = value
	default:
		return fmt.Errorf("Invalid key, valid keys are: addresses, server-cert-fingerprint.")
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
