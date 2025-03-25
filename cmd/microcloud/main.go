// Package microcloud provides the main client tool.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/cmd/tui"
	"github.com/canonical/microcloud/microcloud/version"
)

// CmdControl has functions that are common to the microcloud commands.
// command line tools.
type CmdControl struct {
	cmd *cobra.Command //nolint:unused // FIXME: Remove the nolint flag when this is in use.

	FlagHelp          bool
	FlagVersion       bool
	FlagMicroCloudDir string
	FlagNoColor       bool

	asker *tui.InputHandler
}

func main() {
	// Only root should run this
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "This must be run as root")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	asker, err := setupAsker(ctx)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	commonCmd := CmdControl{asker: asker}
	app := &cobra.Command{
		Use:               "microcloud",
		Short:             "Command for managing the MicroCloud daemon",
		Version:           version.Version(),
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if commonCmd.FlagNoColor {
				tui.DisableColors()
			}
		},
	}

	app.PersistentFlags().StringVar(&commonCmd.FlagMicroCloudDir, "state-dir", "", "Path to store MicroCloud state information"+"``")
	app.PersistentFlags().BoolVarP(&commonCmd.FlagHelp, "help", "h", false, "Print help")
	app.PersistentFlags().BoolVar(&commonCmd.FlagVersion, "version", false, "Print version number")
	app.PersistentFlags().BoolVar(&commonCmd.FlagNoColor, "no-color", false, "Disable colorization of the CLI")

	app.SetVersionTemplate("{{.Version}}\n")

	var cmdInit = cmdInit{common: &commonCmd}
	app.AddCommand(cmdInit.Command())

	var cmdAdd = cmdAdd{common: &commonCmd}
	app.AddCommand(cmdAdd.Command())

	var cmdJoin = cmdJoin{common: &commonCmd}
	app.AddCommand(cmdJoin.Command())

	var cmdPreseed = cmdPreseed{common: &commonCmd}
	app.AddCommand(cmdPreseed.Command())

	var cmdRemove = cmdRemove{common: &commonCmd}
	app.AddCommand(cmdRemove.Command())

	var cmdService = cmdServices{common: &commonCmd}
	app.AddCommand(cmdService.Command())

	var cmdStatus = cmdStatus{common: &commonCmd}
	app.AddCommand(cmdStatus.Command())

	var cmdPeers = cmdClusterMembers{common: &commonCmd}
	app.AddCommand(cmdPeers.Command())

	var cmdShutdown = cmdShutdown{common: &commonCmd}
	app.AddCommand(cmdShutdown.Command())

	var cmdSQL = cmdSQL{common: &commonCmd}
	app.AddCommand(cmdSQL.Command())

	var cmdSecrets = cmdSecrets{common: &commonCmd}
	app.AddCommand(cmdSecrets.Command())

	var cmdWaitready = cmdWaitready{common: &commonCmd}
	app.AddCommand(cmdWaitready.Command())

	app.InitDefaultHelpCmd()

	app.SetErr(&tui.ColorErr{})

	err = app.Execute()
	if err != nil {
		os.Exit(1)
	}
}
