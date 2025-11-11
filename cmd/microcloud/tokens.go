package main

import (
	"context"
	"fmt"
	"sort"

	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/microcluster/v3/microcluster"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/cmd/tui"
)

type cmdSecrets struct {
	common *CmdControl
}

// command returns the tokens subcommand.
func (c *cmdSecrets) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Manage join tokens",
		RunE:  c.run,
	}

	var cmdList = cmdTokensList{common: c.common}
	cmd.AddCommand(cmdList.command())

	var cmdRevoke = cmdTokensRevoke{common: c.common}
	cmd.AddCommand(cmdRevoke.command())

	return cmd
}

// run runs the tokens subcommand.
func (c *cmdSecrets) run(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

type cmdTokensList struct {
	common     *CmdControl
	flagFormat string
}

// command returns the subcommand for listing tokens.
func (c *cmdTokensList) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List join tokens available for use",
		RunE:  c.run,
	}

	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", tui.TableFormatTable, "Format (csv|json|table|yaml|compact)")

	return cmd
}

// run runs the subcommand for listing tokens.
func (c *cmdTokensList) run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir}
	m, err := microcluster.App(options)
	if err != nil {
		return err
	}

	err = m.Ready(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to wait for MicroCloud to get ready: %w", err)
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

	table, err := tui.FormatData(c.flagFormat, header, data, records)
	if err != nil {
		return err
	}

	fmt.Println(table)

	return nil
}

type cmdTokensRevoke struct {
	common *CmdControl
}

// command returns the subcommand for revoking tokens by name.
func (c *cmdTokensRevoke) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke <name>",
		Short: "Revoke the specified join token",
		RunE:  c.run,
	}

	return cmd
}

// run runs the subcommand for revoking tokens by name.
func (c *cmdTokensRevoke) run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Help()
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir}
	m, err := microcluster.App(options)
	if err != nil {
		return err
	}

	err = m.Ready(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to wait for MicroCloud to get ready: %w", err)
	}

	err = m.RevokeJoinToken(context.Background(), args[0])
	if err != nil {
		return err
	}

	return nil
}
