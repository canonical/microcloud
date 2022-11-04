package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/canonical/microcluster/microcluster"
	"github.com/lxc/lxd/lxc/utils"
	"github.com/spf13/cobra"
)

type cmdSecrets struct {
	common *CmdControl
}

func (c *cmdSecrets) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Manage join tokens for MicroCluster",
		RunE:  c.Run,
	}

	var cmdAdd = cmdTokensAdd{common: c.common}
	cmd.AddCommand(cmdAdd.Command())

	var cmdList = cmdTokensList{common: c.common}
	cmd.AddCommand(cmdList.Command())

	var cmdRevoke = cmdTokensRevoke{common: c.common}
	cmd.AddCommand(cmdRevoke.Command())

	return cmd
}

func (c *cmdSecrets) Run(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

type cmdTokensAdd struct {
	common *CmdControl
}

func (c *cmdTokensAdd) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new join token under the given name",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdTokensAdd) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Help()
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir, Verbose: c.common.FlagLogVerbose, Debug: c.common.FlagLogDebug}
	m, err := microcluster.App(context.Background(), options)
	if err != nil {
		return err
	}

	token, err := m.NewJoinToken(args[0])
	if err != nil {
		return err
	}

	fmt.Println(token)

	return nil
}

type cmdTokensList struct {
	common *CmdControl
}

func (c *cmdTokensList) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List join tokens available for use",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdTokensList) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir, Verbose: c.common.FlagLogVerbose, Debug: c.common.FlagLogDebug}
	m, err := microcluster.App(context.Background(), options)
	if err != nil {
		return err
	}

	records, err := m.ListJoinTokens()
	if err != nil {
		return err
	}

	data := make([][]string, len(records))
	for i, record := range records {
		data[i] = []string{record.Name, record.Token}
	}

	header := []string{"NAME", "TOKENS"}
	sort.Sort(utils.ByName(data))

	return utils.RenderTable(utils.TableFormatTable, header, data, records)
}

type cmdTokensRevoke struct {
	common *CmdControl
}

func (c *cmdTokensRevoke) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke <name>",
		Short: "Revoke the join token with the given name",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdTokensRevoke) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Help()
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir, Verbose: c.common.FlagLogVerbose, Debug: c.common.FlagLogDebug}
	m, err := microcluster.App(context.Background(), options)
	if err != nil {
		return err
	}

	err = m.RevokeJoinToken(args[0])
	if err != nil {
		return err
	}

	return nil
}
