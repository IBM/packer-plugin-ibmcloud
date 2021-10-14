package classic

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepWaitforInstance struct{}

func (s *stepWaitforInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*SoftlayerClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	ui.Say("Waiting for the instance to become ACTIVE...")

	instance := state.Get("instance_data").(map[string]interface{})
	err := client.waitForInstanceReady(instance["globalIdentifier"].(string), config.StateTimeout)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error waiting for instance to become ACTIVE: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	ui.Say("Active!")

	return multistep.ActionContinue
}

func (client *stepWaitforInstance) Cleanup(state multistep.StateBag) {}
