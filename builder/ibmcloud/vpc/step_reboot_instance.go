package vpc

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepRebootInstance struct{}

func (s *stepRebootInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	ui.Say("Rebooting instance to cleanly complete any installed software components...")

	instanceData := state.Get("instance_data").(map[string]interface{})
	instanceID := instanceData["id"].(string)

	status, err := client.manageInstance(instanceID, "reboot", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error rebooting the instance: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	if status != "running" {
		err := client.waitForResourceReady(instanceID, "instances", config.StateTimeout, state)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error rebooting the instance: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return multistep.ActionHalt
		}
	}

	newInstanceData, err := client.retrieveResource(instanceID, state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error updating the instance: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}
	state.Put("instance_data", newInstanceData)

	ui.Say("Instance is ACTIVE!")
	return multistep.ActionContinue
}

func (client *stepRebootInstance) Cleanup(state multistep.StateBag) {}
