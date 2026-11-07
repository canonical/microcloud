package framework

// RebootMember reboots member using the resolved NodeController.
func RebootMember(member string) error {
	c, err := resolveController()
	if err != nil {
		return err
	}

	return c.Reboot(member)
}
