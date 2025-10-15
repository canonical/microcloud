package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/canonical/lxd/shared/api"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/microcluster/v3/client"
	"github.com/canonical/microcluster/v3/microcluster"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/database"
)

type cmdClusterManager struct {
	common *CmdControl
}

// Command returns the subcommand to manage cluster manager configuration.
func (c *cmdClusterManager) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "cluster-manager"
	cmd.Short = "Manage cluster manager connection"
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Skip the check if this is the root command itself
		if cmd.Use == "cluster-manager" {
			return nil
		}

		return checkInitialized(c.common.FlagMicroCloudDir, true, false)
	}

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
	alias  *cmdClusterManager
}

func (c *cmdClusterManagerJoin) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "join <token>"
	cmd.Short = "Join a cluster manager"
	cmd.Example = `microcloud cluster-manager join "base64_encoded_token"`

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerJoin) run(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("Expected exactly one argument with the token")
	}

	token := args[0]

	if token == "" {
		return errors.New("Token cannot be empty")
	}

	payload := types.ClusterManagersPost{
		Name:  database.ClusterManagerDefaultName,
		Token: token,
	}

	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	err = apiClient.QueryStruct(context.Background(), "POST", "1.0", api.NewURL().Path("cluster-managers"), payload, nil)
	if err != nil {
		return err
	}

	fmt.Println("Successfully joined cluster manager")

	return nil
}

// Show.
type cmdClusterManagerShow struct {
	common *CmdControl
	alias  *cmdClusterManager
}

func (c *cmdClusterManagerShow) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show"
	cmd.Short = "Show cluster manager configuration"
	cmd.Example = `microcloud cluster-manager show`

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerShow) run(_ *cobra.Command, _ []string) error {
	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	var clusterManager *types.ClusterManager
	err = apiClient.QueryStruct(context.Background(), "GET", "1.0", api.NewURL().Path("cluster-managers", database.ClusterManagerDefaultName), nil, &clusterManager)
	if err != nil {
		return err
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
	common    *CmdControl
	alias     *cmdClusterManager
	flagForce bool
}

func (c *cmdClusterManagerDelete) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "delete"
	cmd.Short = "Clear cluster manager configuration"
	cmd.Example = `microcloud cluster-manager delete`

	cmd.Flags().BoolVarP(&c.flagForce, "force", "f", false, "Forcibly remove cluster manager configuration without notifying cluster manager.")

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerDelete) run(_ *cobra.Command, _ []string) error {
	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	payload := types.ClusterManagerDelete{
		Force: c.flagForce,
	}

	err = apiClient.QueryStruct(context.Background(), "DELETE", "1.0", api.NewURL().Path("cluster-managers", database.ClusterManagerDefaultName), payload, nil)
	if err != nil {
		return err
	}

	return nil
}

// Get.
type cmdClusterManagerGet struct {
	common *CmdControl
	alias  *cmdClusterManager
}

func (c *cmdClusterManagerGet) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "get"
	cmd.Short = "Get specific cluster manager configuration by key."
	cmd.Example = cli.FormatSection("", `microcloud cluster-manager get addresses
microcloud cluster-manager get certificate-fingerprint
microcloud cluster-manager get update-interval-seconds
microcloud cluster-manager get status-last-success-time
microcloud cluster-manager get status-last-error-time
microcloud cluster-manager get status-last-error-response`)

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerGet) run(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("Expected exactly one argument with the key")
	}

	key := args[0]

	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	var clusterManager *types.ClusterManager
	err = apiClient.QueryStruct(context.Background(), "GET", "1.0", api.NewURL().Path("cluster-managers", database.ClusterManagerDefaultName), nil, &clusterManager)
	if err != nil {
		return err
	}

	switch key {
	case "addresses":
		fmt.Printf("%s\n", strings.Join(clusterManager.Addresses, ", "))
	case "certificate-fingerprint":
		fmt.Printf("%s\n", clusterManager.CertificateFingerprint)
	case "update-interval-seconds":
		value, ok := clusterManager.Config[database.UpdateIntervalSecondsKey]
		if ok {
			fmt.Printf("%s\n", value)
		}

	case "status-last-success-time":
		fmt.Printf("%s\n", clusterManager.StatusLastSuccessTime)
	case "status-last-error-time":
		fmt.Printf("%s\n", clusterManager.StatusLastErrorTime)
	case "status-last-error-response":
		fmt.Printf("%s\n", clusterManager.StatusLastErrorResponse)
	default:
		return errors.New("Invalid key")
	}

	return nil
}

// Set.
type cmdClusterManagerSet struct {
	common *CmdControl
	alias  *cmdClusterManager
}

func (c *cmdClusterManagerSet) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "set"
	cmd.Short = "Set specific cluster manager configuration key."
	cmd.Example = cli.FormatSection("", `microcloud cluster-manager set addresses example.com:8443
microcloud cluster-manager set certificate-fingerprint abababababababababababababababababababababababababababababababab
microcloud cluster-manager set update-interval-seconds 50`)

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerSet) run(_ *cobra.Command, args []string) error {
	if len(args) != 2 {
		return errors.New("Expected exactly two arguments: key and value")
	}

	key := args[0]
	value := args[1]

	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	payload := types.ClusterManagerPut{}

	switch key {
	case "addresses":
		payload.Addresses = []string{value}
	case "certificate-fingerprint":
		payload.CertificateFingerprint = &value
	case "update-interval-seconds":
		payload.UpdateInterval = &value
	default:
		return errors.New("Invalid key")
	}

	err = apiClient.QueryStruct(context.Background(), "PUT", "1.0", api.NewURL().Path("cluster-managers", database.ClusterManagerDefaultName), payload, nil)
	if err != nil {
		return err
	}

	return nil
}

// Unset.
type cmdClusterManagerUnset struct {
	common *CmdControl
	alias  *cmdClusterManager
}

func (c *cmdClusterManagerUnset) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "unset"
	cmd.Short = "Unset specific cluster manager configuration key."
	cmd.Example = cli.FormatSection("", `microcloud cluster-manager unset update-interval-seconds`)

	cmd.RunE = c.run

	return cmd
}

func (c *cmdClusterManagerUnset) run(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("Expected exactly on argument with the key")
	}

	key := args[0]

	apiClient, err := getApiClient(c.common)
	if err != nil {
		return err
	}

	payload := types.ClusterManagerPut{}

	switch key {
	case "update-interval-seconds":
		emptyString := ""
		payload.UpdateInterval = &emptyString
	default:
		return errors.New("Invalid key")
	}

	err = apiClient.QueryStruct(context.Background(), "PUT", "1.0", api.NewURL().Path("cluster-managers", database.ClusterManagerDefaultName), payload, nil)
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
