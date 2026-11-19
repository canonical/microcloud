package health

// checkMicroOVN verifies microovn status reports healthy Northbound/Southbound OVN databases on
// member.
func checkMicroOVN(h *healthChecker, member string) {
	h.checkAndRequire(member, []string{"OVN Northbound: OK", "OVN Southbound: OK"}, "sudo", "microovn", "status")
}
