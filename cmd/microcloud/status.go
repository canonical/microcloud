package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/canonical/microcluster/v2/microcluster"
	microTypes "github.com/canonical/microcluster/v2/rest/types"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/cmd/tui"
	"github.com/canonical/microcloud/microcloud/component"
)

// Warning represents a warning message with a severity level.
type Warning struct {
	Level   StatusLevel
	Message string
}

// Warnings is a list of warnings.
type Warnings []Warning

// Status returns the overall status of the warning list.
// If there are any Error level warnings, the status will be error.
// Otherwise, if there are any Warn level warnings, the status will be warn.
// Finally, the status will be Success, implying no warnings.
func (w Warnings) Status() StatusLevel {
	if len(w) == 0 {
		return Success
	}

	for _, warning := range w {
		if warning.Level == Error {
			return Error
		}
	}

	return Warn
}

// StatusLevel represents the severity level of warnings.
type StatusLevel int

const (
	// Success represents a lack of warnings.
	Success StatusLevel = iota

	// Warn represents a medium severity warning.
	Warn

	// Error represents a critical warning.
	Error
)

// Symbol returns the single-character symbol representing the StatusLevel, color coded.
func (s StatusLevel) Symbol() string {
	switch s {
	case Success:
		return tui.SuccessSymbol()
	case Warn:
		return tui.WarningSymbol()
	case Error:
		return tui.ErrorSymbol()
	}

	return ""
}

// Symbol returns a word representing the StatusLevel, color coded.
func (s StatusLevel) String() string {
	switch s {
	case Success:
		return tui.SuccessColor("HEALTHY", true)
	case Warn:
		return tui.WarningColor("WARNING", true)
	case Error:
		return tui.ErrorColor("ERROR", true)
	}

	return ""
}

type cmdStatus struct {
	common *CmdControl
}

// Command returns the subcommand for the deployment status.
func (c *cmdStatus) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Deployment status with configuration warnings",
		RunE:  c.Run,
	}

	return cmd
}

// Run runs the subcommand for the deployment status.
func (c *cmdStatus) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	cloudApp, err := microcluster.App(microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	err = cloudApp.Ready(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to wait for MicroCloud to get ready: %w", err)
	}

	status, err := cloudApp.Status(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get MicroCloud status: %w", err)
	}

	if !status.Ready {
		return fmt.Errorf("MicroCloud is uninitialized, run 'microcloud init' first")
	}

	cfg := initConfig{
		autoSetup: true,
		bootstrap: false,
		common:    c.common,
		asker:     c.common.asker,
		systems:   map[string]InitSystem{},
		state:     map[string]component.SystemInformation{},
	}

	cfg.name = status.Name
	cfg.address = status.Address.Addr().String()

	components := []types.ComponentType{types.MicroCloud, types.LXD}
	optionalComponents := map[types.ComponentType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	components, err = cfg.askMissingComponents(components, optionalComponents)
	if err != nil {
		return err
	}

	// Instantiate a handler for the components.
	sh, err := component.NewHandler(status.Name, status.Address.Addr().String(), c.common.FlagMicroCloudDir, components...)
	if err != nil {
		return err
	}

	cloudClient, err := sh.Components[types.MicroCloud].(*component.CloudComponent).Client()
	if err != nil {
		return err
	}

	// Query the status API for the cluster.
	statuses, err := client.GetStatus(context.Background(), cloudClient)
	if err != nil {
		return err
	}

	// compile all warning messages.
	warnings := compileWarnings(cfg.name, statuses)

	// Print the warning summary, and all warnings.
	fmt.Println("")
	fmt.Printf(" %s: %s\n", tui.SetColor(tui.Bright, "Status", true), warnings.Status().String())
	fmt.Println("")
	for _, w := range warnings {
		fmt.Printf(" %s %s %s\n", tui.SetColor(tui.Bright, "┃", true), w.Level.Symbol(), w.Message)
	}

	if len(warnings) > 0 {
		fmt.Println("")
	}

	headers := []string{"Name", "Address", "OSDs", "MicroCeph Units", "MicroOVN Units", "Status"}

	statusByName := make(map[string]types.Status, len(statuses))
	var localStatus types.Status
	for _, s := range statuses {
		if s.Name == cfg.name {
			localStatus = s
		}

		statusByName[s.Name] = s
	}

	// Format and colorize cells of the table.
	rows := make([][]string, 0, len(statuses))
	for _, s := range statuses {
		rows = append(rows, formatStatusRow(localStatus, s))
	}

	for _, member := range localStatus.Clusters[types.MicroCloud] {
		_, ok := statusByName[member.Name]
		if ok {
			continue
		}

		status := types.Status{
			Name:     member.Name,
			Address:  member.Address.Addr().String(),
			Clusters: localStatus.Clusters,
		}

		rows = append(rows, formatStatusRow(localStatus, status))
	}

	// Sort the rows by the Name column.
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0] < rows[j][0]
	})

	// Print the table.
	fmt.Println(tui.NewTable(headers, rows))

	return nil
}

// compileWarnings returns a set of warnings based on the given set of statuses. The name supplied should be the local cluster name.
func compileWarnings(name string, statuses []types.Status) Warnings {
	// Systems that exist in other clusters but not in MicroCloud.
	unmanagedSystems := map[types.ComponentType]map[string]bool{}

	// Systems that exist in MicroCloud, but not other clusters.
	orphanedSystems := map[types.ComponentType]map[string]bool{}

	// Components that are uninitialized on a system.
	uninstalledComponents := map[types.ComponentType][]string{}

	// Components undergoing schema/API upgrades.
	upgradingComponents := map[types.ComponentType]bool{}

	// Systems that are offline on at least one component.
	offlineSystems := map[string][]string{}

	osdsConfigured := false
	clusterSize := 0
	osdCount := 0

	for _, s := range statuses {
		if s.Name == name {
			clusterSize = len(s.Clusters[types.MicroCloud])
			for component, clusterMembers := range s.Clusters {
				for _, member := range clusterMembers {
					if member.Status == microTypes.MemberNeedsUpgrade || member.Status == microTypes.MemberUpgrading {
						upgradingComponents[component] = true
					} else if member.Status != microTypes.MemberOnline {
						if offlineSystems[member.Name] == nil {
							offlineSystems[member.Name] = []string{}
						}

						offlineSystems[member.Name] = append(offlineSystems[member.Name], string(component))
					}
				}
			}
		}

		osdCount = osdCount + len(s.OSDs)
		allComponents := []types.ComponentType{types.LXD, types.MicroCeph, types.MicroOVN, types.MicroCloud}
		cloudMembers := make(map[string]bool, len(s.Clusters[types.MicroCloud]))
		for _, member := range s.Clusters[types.MicroCloud] {
			cloudMembers[member.Name] = true
		}

		for _, component := range allComponents {
			members, ok := s.Clusters[component]
			if !ok || len(members) == 0 {
				if uninstalledComponents[component] == nil {
					uninstalledComponents[component] = []string{}
				}

				uninstalledComponents[component] = append(uninstalledComponents[component], s.Name)
			}

			if component == types.MicroCloud || s.Name != name {
				continue
			}

			for _, member := range s.Clusters[component] {
				if !cloudMembers[member.Name] {
					if unmanagedSystems[component] == nil {
						unmanagedSystems[component] = map[string]bool{}
					}

					unmanagedSystems[component][member.Name] = true
				}
			}

			if len(s.Clusters[component]) > 0 {
				clusterMap := make(map[string]bool, len(s.Clusters[component]))
				for _, member := range s.Clusters[component] {
					clusterMap[member.Name] = true
				}

				for name := range cloudMembers {
					if !clusterMap[name] {
						if orphanedSystems[component] == nil {
							orphanedSystems[component] = map[string]bool{}
						}

						orphanedSystems[component][name] = true
					}
				}
			}
		}

		if osdCount > 0 && len(s.Clusters[types.MicroCeph]) > 0 {
			osdsConfigured = true
		}
	}

	// Format the actual warnings based on the collected data.
	warnings := Warnings{}
	if clusterSize < 3 {
		tmpl := tui.Fmt{Arg: "%s: %d systems are required for effective fault tolerance"}
		msg := tui.Printf(tmpl,
			tui.Fmt{Color: tui.Red, Arg: "Reliability risk", Bold: true},
			tui.Fmt{Color: tui.Bright, Arg: 3, Bold: true},
		)

		warnings = append(warnings, Warning{Level: Warn, Message: msg})
	}

	if osdCount < 3 && osdsConfigured {
		tmpl := tui.Fmt{Arg: "%s: MicroCeph OSD replication recommends at least %d disks across %d systems"}
		msg := tui.Printf(tmpl,
			tui.Fmt{Color: tui.Red, Arg: "Data loss risk", Bold: true},
			tui.Fmt{Color: tui.Bright, Arg: 3, Bold: true},
			tui.Fmt{Color: tui.Bright, Arg: 3, Bold: true},
		)

		warnings = append(warnings, Warning{Level: Warn, Message: msg})
	}

	if len(uninstalledComponents[types.LXD]) > 0 {
		tmpl := tui.Fmt{Arg: "LXD is not found on %s"}
		msg := tui.Printf(tmpl, tui.Fmt{Color: tui.Bright, Arg: strings.Join(uninstalledComponents[types.LXD], ", "), Bold: true})
		warnings = append(warnings, Warning{Level: Error, Message: msg})
	}

	for component, systems := range orphanedSystems {
		list := make([]string, 0, len(systems))
		for name := range systems {
			list = append(list, name)
		}

		tmpl := tui.Fmt{Arg: "MicroCloud members not found in %s: %s"}
		msg := tui.Printf(tmpl,
			tui.Fmt{Color: tui.Bright, Arg: component, Bold: true},
			tui.Fmt{Color: tui.Bright, Bold: true, Arg: strings.Join(list, ", ")})
		warnings = append(warnings, Warning{Level: Error, Message: msg})
	}

	if !osdsConfigured && len(uninstalledComponents[types.MicroCeph]) < clusterSize {
		warnings = append(warnings, Warning{Level: Warn, Message: "No MicroCeph OSDs configured"})
	}

	for name, components := range offlineSystems {
		tmpl := tui.Fmt{Arg: "%s is not available on %s"}
		msg := tui.Printf(tmpl, tui.Fmt{Color: tui.Bright, Bold: true, Arg: strings.Join(components, ", ")}, tui.Fmt{Color: tui.Bright, Bold: true, Arg: name})
		warnings = append(warnings, Warning{Level: Error, Message: msg})
	}

	for component := range upgradingComponents {
		tmpl := tui.Fmt{Arg: "%s upgrade in progress"}
		msg := tui.Printf(tmpl, tui.Fmt{Color: tui.Bright, Bold: true, Arg: component})
		warnings = append(warnings, Warning{Level: Warn, Message: msg})
	}

	for component, names := range uninstalledComponents {
		if component == types.LXD || component == types.MicroCloud {
			continue
		}

		tmpl := tui.Fmt{Arg: "%s is not found on %s"}
		msg := tui.Printf(tmpl,
			tui.Fmt{Color: tui.Bright, Bold: true, Arg: component},
			tui.Fmt{Color: tui.Bright, Bold: true, Arg: strings.Join(names, ", ")})
		warnings = append(warnings, Warning{Level: Warn, Message: msg})
	}

	for component, systems := range unmanagedSystems {
		list := make([]string, 0, len(systems))
		for name := range systems {
			list = append(list, name)
		}

		tmpl := tui.Fmt{Arg: "Found %s systems not managed by MicroCloud: %s"}
		msg := tui.Printf(tmpl,
			tui.Fmt{Color: tui.Bright, Bold: true, Arg: component},
			tui.Fmt{Color: tui.Bright, Bold: true, Arg: strings.Join(list, ",")})
		warnings = append(warnings, Warning{Level: Warn, Message: msg})
	}

	return warnings
}

// formatStatusRow formats the given status data for a cluster member into a row of the table.
// Also takes the local system's status which will be used as the source of truth for cluster member responsiveness.
func formatStatusRow(localStatus types.Status, s types.Status) []string {
	osds := tui.WarningColor("0", false)
	if len(s.OSDs) > 0 {
		osds = strconv.Itoa(len(s.OSDs))
	}

	cephComponents := tui.WarningColor("-", false)

	if len(s.CephComponents) > 0 {
		components := make([]string, 0, len(s.CephComponents))
		for _, component := range s.CephComponents {
			components = append(components, component.Service)
		}

		cephComponents = strings.Join(components, ",")
	}

	ovnComponents := tui.WarningColor("-", false)
	if len(s.OVNComponents) > 0 {
		components := make([]string, 0, len(s.OVNComponents))
		for _, component := range s.OVNComponents {
			components = append(components, component.Service)
		}

		ovnComponents = strings.Join(components, ",")
	}

	if len(s.Clusters[types.MicroOVN]) == 0 {
		ovnComponents = tui.ErrorColor("-", false)
	}

	if len(s.Clusters[types.MicroCeph]) == 0 {
		cephComponents = tui.ErrorColor("-", false)
		osds = tui.ErrorColor("-", false)
	}

	status := tui.SuccessColor(string(microTypes.MemberOnline), false)
	for _, members := range localStatus.Clusters {
		for _, member := range members {
			if member.Name != s.Name {
				continue
			}

			// Only set the component status to upgrading if no other member has a more urgent status.
			if member.Status == microTypes.MemberUpgrading || member.Status == microTypes.MemberNeedsUpgrade {
				if status == tui.SuccessColor(string(microTypes.MemberOnline), false) {
					status = tui.WarningColor(string(member.Status), false)
				}
			} else if member.Status != microTypes.MemberOnline {
				status = tui.ErrorColor(string(member.Status), false)
			}
		}
	}

	return []string{s.Name, s.Address, osds, cephComponents, ovnComponents, status}
}
