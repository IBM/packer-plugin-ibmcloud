package vpc

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepGetBaseImageID struct {
}

func (step *stepGetBaseImageID) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	// Fetching Base Image ID
	if config.VSIBaseImageName != "" {
		ui.Say("Fetching Base Image ID...")
		baseImageID, err := client.getImageIDByName(config.VSIBaseImageName, state)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error getting base-image ID: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		state.Put("baseImageID", baseImageID)
		ui.Say(fmt.Sprintf("Base Image ID fetched: %s", string(baseImageID)))
	}

	return multistep.ActionContinue
}

func (step *stepGetBaseImageID) Cleanup(state multistep.StateBag) {
}
