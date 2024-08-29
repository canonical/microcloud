package main

import (
	"context"
	"sort"

	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/spf13/cobra"
)

type cmdSecrets struct {
	common *CmdControl
}

func (c *cmdSecrets) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Manage join tokens",
		RunE:  c.Run,
	}

	var cmdList = cmdTokensList{common: c.common}
	cmd.AddCommand(cmdList.Command())

	var cmdRevoke = cmdTokensRevoke{common: c.common}
	cmd.AddCommand(cmdRevoke.Command())

	return cmd
}

func (c *cmdSecrets) Run(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

type cmdTokensList struct {
	common     *CmdControl
	flagFormat string
}

func (c *cmdTokensList) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List join tokens available for use",
		RunE:  c.Run,
	}

	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", cli.TableFormatTable, "Format (csv|json|table|yaml|compact)")

	return cmd
}

func (c *cmdTokensList) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir}
	m, err := microcluster.App(options)
	if err != nil {
		return err
	}

	records, err := m.ListJoinTokens(context.Background())
	if err != nil {
		return err
	}

	data := make([][]string, len(records))
	for i, record := range records {
		data[i] = []string{record.Name, record.Token}
	}

	header := []string{"NAME", "TOKENS"}
	sort.Sort(cli.SortColumnsNaturally(data))

	return cli.RenderTable(c.flagFormat, header, data, records)
}

type cmdTokensRevoke struct {
	common *CmdControl
}

func (c *cmdTokensRevoke) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke <name>",
		Short: "Revoke the specified join token",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdTokensRevoke) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Help()
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir}
	m, err := microcluster.App(options)
	if err != nil {
		return err
	}

	err = m.RevokeJoinToken(context.Background(), args[0])
	if err != nil {
		return err
	}

	return nil
}
