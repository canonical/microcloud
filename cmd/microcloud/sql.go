package main

import (
	"context"
	"fmt"
	"os"

	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

type cmdSQL struct {
	common *CmdControl
}

func (c *cmdSQL) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a SQL query against the daemon",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdSQL) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		err := cmd.Help()
		if err != nil {
			return fmt.Errorf("Unable to load help: %w", err)
		}

		if len(args) == 0 {
			return nil
		}
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir}
	m, err := microcluster.App(options)
	if err != nil {
		return err
	}

	query := args[0]
	dump, batch, err := m.SQL(context.Background(), query)
	if err != nil {
		return err
	}

	if dump != "" {
		fmt.Print(dump)
		return nil
	}

	for i, result := range batch.Results {
		if len(batch.Results) > 1 {
			fmt.Printf("=> Query %d:\n\n", i)
		}

		if result.Type == "select" {
			sqlPrintSelectResult(result.Columns, result.Rows)
		} else {
			fmt.Printf("Rows affected: %d\n", result.RowsAffected)
		}

		if len(batch.Results) > 1 {
			fmt.Printf("\n")
		}
	}
	return nil
}

func sqlPrintSelectResult(columns []string, rows [][]any) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)
	table.SetHeader(columns)
	for _, row := range rows {
		data := []string{}
		for _, col := range row {
			data = append(data, fmt.Sprintf("%v", col))
		}

		table.Append(data)
	}

	table.Render()
}
