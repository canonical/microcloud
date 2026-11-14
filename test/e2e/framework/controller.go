package framework

import (
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/controller"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/testbed"
)

// testbedController is the cached controller.Controller returned by GetTestbedController.
var testbedController controller.Controller

// GetTestbedController returns the shared controller.Controller for the testbed config at
// E2ETestbedFile(), constructing it lazily on first call and reusing it on every subsequent call.
func GetTestbedController() (controller.Controller, error) {
	if testbedController != nil {
		return testbedController, nil
	}

	cfg, err := testbed.Load(E2ETestbedFile())
	if err != nil {
		return nil, err
	}

	c, err := controller.New(cfg)
	if err != nil {
		return nil, err
	}

	testbedController = c

	return testbedController, nil
}
