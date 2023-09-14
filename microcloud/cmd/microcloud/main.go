// Package microcloud provides the main client tool.
package main

import (
	"bufio"
	"fmt"
	"os"

	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/version"
)

// CmdControl has functions that are common to the microcloud commands.
// command line tools.
type CmdControl struct {
	cmd *cobra.Command //nolint:structcheck,unused // FIXME: Remove the nolint flag when this is in use.

	FlagHelp          bool
	FlagVersion       bool
	FlagLogDebug      bool
	FlagLogVerbose    bool
	FlagMicroCloudDir string

	asker cli.Asker
}

func main() {
	// Only root should run this
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "This must be run as root")
		os.Exit(1)
	}

	// common flags.
	commonCmd := CmdControl{asker: cli.NewAsker(bufio.NewReader(os.Stdin))}

	app := &cobra.Command{
		Use:               "microcloud",
		Short:             "Command for managing the MicroCloud daemon",
		Version:           version.Version,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}

	app.PersistentFlags().StringVar(&commonCmd.FlagMicroCloudDir, "state-dir", "", "Path to store MicroCloud state information"+"``")
	app.PersistentFlags().BoolVarP(&commonCmd.FlagHelp, "help", "h", false, "Print help")
	app.PersistentFlags().BoolVar(&commonCmd.FlagVersion, "version", false, "Print version number")
	app.PersistentFlags().BoolVarP(&commonCmd.FlagLogDebug, "debug", "d", false, "Show all debug messages")
	app.PersistentFlags().BoolVarP(&commonCmd.FlagLogVerbose, "verbose", "v", false, "Show all information messages")

	app.SetVersionTemplate("{{.Version}}\n")

	var cmdInit = cmdInit{common: &commonCmd}
	app.AddCommand(cmdInit.Command())

	var cmdAdd = cmdAdd{common: &commonCmd}
	app.AddCommand(cmdAdd.Command())

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

	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}
