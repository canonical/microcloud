package framework

// ProjectName returns the LXD project all e2e resources are created in. It is
// fixed to "e2e-testing", matching the project name created by
// test/e2e/terraform/main.tf.
func ProjectName() string {
	return "e2e-testing"
}

// LocalProjectName returns the local LXD project searched when validating a
// testbed file's lxd section.
func LocalProjectName() string {
	return Testbed().LXD.Project
}
