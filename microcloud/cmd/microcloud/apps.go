package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/canonical/lxd/shared/api"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/microcluster/microcluster"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/canonical/microcloud/microcloud/api/types"
)

type cmdApps struct {
	common *CmdControl
}

func (c *cmdApps) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Commands for managing microcloud apps",
		RunE:  c.Run,
	}

	show := &cmdAppShow{common: c.common}
	launch := &cmdAppLaunch{common: c.common}
	list := &cmdAppList{common: c.common}
	delete := &cmdAppDelete{common: c.common}
	cmd.AddCommand(show.Command(), list.Command(), launch.Command(), delete.Command())

	return cmd
}

func (c *cmdApps) Run(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

type cmdAppShow struct {
	common      *CmdControl
	flagProject string
}

func (c *cmdAppShow) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <app_kind> <app_name>",
		Short: "Show running app info",
		RunE:  c.Run,
	}

	cmd.Flags().StringVar(&c.flagProject, "project", "default", "Project containing the microk8s cluster")
	microk8sShow := &cmdMicrok8sShow{common: c.common, appShow: c}
	cmd.AddCommand(microk8sShow.Command())
	return cmd
}

func (*cmdAppShow) Run(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

type cmdMicrok8sShow struct {
	common  *CmdControl
	appShow *cmdAppShow
}

func (c *cmdMicrok8sShow) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "microk8s <cluster_name> [--project <project_name>]",
		Short: "Show configuration of a microk8s cluster",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdMicrok8sShow) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 || c.common.FlagHelp {
		return cmd.Help()
	}

	clusterName := args[0]

	m, err := microcluster.App(context.Background(), microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	status, err := m.Status()
	if err != nil {
		return err
	}

	if !status.Ready {
		return errors.New("Microcloud is not ready, please run `microcloud init` first.")
	}

	client, err := m.LocalClient()
	if err != nil {
		return err
	}

	var resp types.AppInfo
	err = client.QueryStruct(context.Background(), http.MethodGet, "1.0", api.NewURL().Path("apps", types.AppKindMicrok8s, clusterName).Project(c.appShow.flagProject), nil, &resp)
	if err != nil {
		return err
	}

	return yaml.NewEncoder(os.Stdout).Encode(resp)
}

type cmdAppList struct {
	common      *CmdControl
	flagProject string
}

func (c *cmdAppList) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [--project <project_name>]",
		Short: "List deployed apps",
		RunE:  c.Run,
	}

	cmd.Flags().StringVar(&c.flagProject, "project", "default", "Project to list deployments within")

	microk8sList := &cmdMicrok8sList{common: c.common, appList: c}
	cmd.AddCommand(microk8sList.Command())
	return cmd
}

func (c *cmdAppList) Run(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cmd.Help()
	}

	m, err := microcluster.App(context.Background(), microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	status, err := m.Status()
	if err != nil {
		return err
	}

	if !status.Ready {
		return errors.New("Microcloud is not ready, please run `microcloud init` first.")
	}

	client, err := m.LocalClient()
	if err != nil {
		return err
	}

	var apps []types.App
	err = client.QueryStruct(context.Background(), http.MethodGet, "1.0", api.NewURL().Path("apps").Project(c.flagProject), nil, &apps)
	if err != nil {
		return err
	}

	data := make([][]string, len(apps))
	for i, app := range apps {
		data[i] = []string{app.Name, app.Kind}
	}

	header := []string{"NAME", "KIND"}
	sort.Sort(cli.SortColumnsNaturally(data))

	return cli.RenderTable(cli.TableFormatTable, header, data, apps)
}

type cmdMicrok8sList struct {
	common  *CmdControl
	appList *cmdAppList
}

func (c *cmdMicrok8sList) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "microk8s [--project <project_name>]",
		Short: "List deployed microk8s apps",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdMicrok8sList) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 || c.common.FlagHelp {
		return cmd.Help()
	}

	m, err := microcluster.App(context.Background(), microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	status, err := m.Status()
	if err != nil {
		return err
	}

	if !status.Ready {
		return errors.New("Microcloud is not ready, please run `microcloud init` first.")
	}

	client, err := m.LocalClient()
	if err != nil {
		return err
	}

	var apps []types.App
	err = client.QueryStruct(context.Background(), http.MethodGet, "1.0", api.NewURL().Path("apps", types.AppKindMicrok8s).Project(c.appList.flagProject), nil, &apps)
	if err != nil {
		return err
	}

	data := make([][]string, len(apps))
	for i, app := range apps {
		data[i] = []string{app.Name, app.Kind}
	}

	header := []string{"NAME", "KIND"}
	sort.Sort(cli.SortColumnsNaturally(data))

	return cli.RenderTable(cli.TableFormatTable, header, data, apps)
}

type cmdAppLaunch struct {
	common       *CmdControl
	flagProject  string
	flagProfiles []string
}

func (c *cmdAppLaunch) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "launch <app_kind> <app_name>",
		Short: "Launch an application",
		RunE:  c.Run,
	}

	cmd.Flags().StringVar(&c.flagProject, "project", "default", "Project to deploy microk8s cluster within")
	cmd.Flags().StringSliceVar(&c.flagProfiles, "profile", []string{"default"}, "Profiles to use when deploying microk8s VMs")

	microk8sLaunch := &cmdMicrok8sLaunch{common: c.common, appLaunch: c}
	cmd.AddCommand(microk8sLaunch.Command())
	return cmd
}

func (*cmdAppLaunch) Run(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

type cmdMicrok8sLaunch struct {
	common    *CmdControl
	appLaunch *cmdAppLaunch
}

func (c *cmdMicrok8sLaunch) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "microk8s <cluster_name> [--project <project_name>] [--profile <profile_name>...] [size=<size> channel=<channel>]",
		Short: "Launch a microk8s cluster",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdMicrok8sLaunch) Run(cmd *cobra.Command, args []string) error {
	if len(args) < 1 || c.common.FlagHelp {
		return cmd.Help()
	}

	post := types.Microk8sPost{}
	post.ClusterName = args[0]
	post.Profiles = c.appLaunch.flagProfiles

	for _, arg := range args[1:] {
		k, v, ok := strings.Cut(arg, "=")
		if !ok {
			return fmt.Errorf("Improperly formatted config argument %q", arg)
		}

		switch k {
		case "size":
			size, err := strconv.Atoi(v)
			if err != nil {
				return err
			}

			post.Size = size
		case "channel":
			post.Channel = v
		default:
			return fmt.Errorf("Unknown config key %q", k)
		}
	}

	m, err := microcluster.App(context.Background(), microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	status, err := m.Status()
	if err != nil {
		return err
	}

	if !status.Ready {
		return errors.New("Microcloud is not ready, please run `microcloud init` first.")
	}

	client, err := m.LocalClient()
	if err != nil {
		return err
	}

	var resp string
	err = client.QueryStruct(context.Background(), http.MethodPost, "1.0", api.NewURL().Path("apps", types.AppKindMicrok8s).Project(c.appLaunch.flagProject), post, &resp)
	if err != nil {
		return err
	}

	fmt.Println(resp)
	return nil
}

type cmdAppDelete struct {
	common      *CmdControl
	flagProject string
	flagForce   bool
}

func (c *cmdAppDelete) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <app_kind> <app_name> [--project <project_name>]",
		Short: "Delete an app",
		RunE:  c.Run,
	}

	cmd.Flags().BoolVarP(&c.flagForce, "force", "f", false, "Force deletion of app instances")
	cmd.Flags().StringVar(&c.flagProject, "project", "default", "Project to delete app instances within")
	return cmd
}

func (c *cmdAppDelete) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return cmd.Help()
	}

	switch args[0] {
	case types.AppKindMicrok8s:
		break
	default:
		return fmt.Errorf("Invalid app kind %q", args[0])
	}

	m, err := microcluster.App(context.Background(), microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	status, err := m.Status()
	if err != nil {
		return err
	}

	if !status.Ready {
		return errors.New("Microcloud is not ready, please run `microcloud init` first.")
	}

	client, err := m.LocalClient()
	if err != nil {
		return err
	}

	appURL := api.NewURL().Path("apps", args[0], args[1]).Project(c.flagProject)
	if c.flagForce {
		appURL.WithQuery("force", "true")
	}

	err = client.QueryStruct(context.Background(), http.MethodDelete, "1.0", appURL, nil, nil)
	if err != nil {
		return err
	}

	return nil
}
