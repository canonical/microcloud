package main

import (
	"context"
	"fmt"

	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/cmd/tui"
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

	err = m.Ready(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to wait for MicroCloud to get ready: %w", err)
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
			rows := make([][]string, len(result.Rows))
			for i, row := range result.Rows {
				rowStr := make([]string, len(row))
				for j, c := range row {
					rowStr[j] = fmt.Sprintf("%v", c)
				}

				rows[i] = rowStr
			}

			fmt.Println(tui.NewTable(result.Columns, rows))
		} else {
			fmt.Printf("Rows affected: %d\n", result.RowsAffected)
		}

		if len(batch.Results) > 1 {
			fmt.Printf("\n")
		}
	}
	return nil
}
