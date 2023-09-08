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

	useTestConsole := os.Getenv("TEST_CONSOLE")
	if useTestConsole == "1" {
		fmt.Printf("%s\n\n", `
  Detected 'TEST_CONSOLE=1', MicroCloud CLI is in testing mode. Terminal interactivity is disabled.

  Interactive microcloud commands will read text instructions by line:

cat << EOF | microcloud init
select                # selects an element in the table
select-all            # selects all elements in the table
select-none           # de-selects all elements in the table
up                    # move up in the table
down                  # move down in the table
wait <time.Duration>  # waits before the next instruction
expect <count>        # waits until exactly <count> peers are available, and errors out if more are found
---                   # confirms the table selection and exits the table
clear                 # clears the last line
anything else         # will be treated as a raw string. This is useful for filtering a table and text entry
EOF`)

		commonCmd.asker = prepareTestAsker(os.Stdin)
	}

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
