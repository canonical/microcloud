package health

// checkMicroCeph verifies microceph/ceph report a healthy cluster on member.
func checkMicroCeph(h *healthChecker, member string) {
	h.check(member, "sudo", "microceph", "status")
	h.checkAndRequire(member, []string{"HEALTH_OK"}, "sudo", "ceph", "-s")
}
