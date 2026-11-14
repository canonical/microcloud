package health

// checkMicroCloud verifies microcloud status reports a healthy cluster on member.
func checkMicroCloud(h *healthChecker, member string) {
	h.checkAndRequire(member, []string{"Status: HEALTHY"}, "sudo", "microcloud", "status")
}
