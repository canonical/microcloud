package controller

import (
	"github.com/canonical/lxd/shared/api"
	//nolint:staticcheck // Ginkgo's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"

	lxdframework "github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/testbed"
)

// validateLocalLXDProject reports whether every given cluster member name corresponds to a local
// LXD instance in c.cfg.LXD.Project -- i.e. whether a testbed file's lxd.project claim actually
// holds, checked against the local LXD daemon (independent of REMOTE). Returns false if the local
// daemon is unreachable or any member doesn't match, logging the reason to GinkgoWriter either
// way.
func (c *Controller) validateLocalLXDProject(members []string) bool {
	local, err := lxdframework.ConnectLocal(c.cfg.LXD.Project)
	if err != nil {
		GinkgoWriter.Printf("lxd testbed validation: local LXD daemon not reachable (%v)\n", err)
		return false
	}

	names, err := local.GetInstanceNames(api.InstanceTypeAny)
	if err != nil {
		GinkgoWriter.Printf("lxd testbed validation: cannot list instances in local project %q (%v)\n", c.cfg.LXD.Project, err)
		return false
	}

	missing, ok := membersMatchInstances(members, names)
	if !ok {
		GinkgoWriter.Printf("lxd testbed validation: no local LXD instance named %q in project %q\n", missing, c.cfg.LXD.Project)
		return false
	}

	return true
}

// membersMatchInstances reports whether every name in members is present in instanceNames. If not,
// it also returns the first missing member name for use in diagnostics; ok is true iff every
// member matched.
func membersMatchInstances(members, instanceNames []string) (missing string, ok bool) {
	present := make(map[string]bool, len(instanceNames))
	for _, n := range instanceNames {
		present[n] = true
	}

	for _, m := range members {
		if !present[m] {
			return m, false
		}
	}

	return "", true
}

// validateSSHNodes reports whether every node configured in nodes is reachable over SSH -- i.e.
// whether a testbed file's ssh.nodes claims actually hold -- dialing each once (see
// SSHController.dial) and closing the connection immediately. Returns false at the first
// unreachable node, logging the reason to GinkgoWriter either way.
func (c *Controller) validateSSHNodes(nodes map[string]testbed.SSHNodeConfig) bool {
	sc := newSSHController(nodes)

	for member := range nodes {
		client, err := sc.dial(member)
		if err != nil {
			GinkgoWriter.Printf("ssh testbed validation: cannot connect to %q (%v)\n", member, err)
			return false
		}

		client.Close()
	}

	return true
}
