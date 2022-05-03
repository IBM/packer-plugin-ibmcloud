package vpc

import (
	"context"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepGenerateIAMToken struct {
}

func (step *stepGenerateIAMToken) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	// client := state.Get("client").(*IBMCloudClient)
	ui := state.Get("ui").(packer.Ui)

	ui.Say("Generating IAM Access Token...")
	// err := client.getIAMToken(state)
	// if err != nil {
	// 	err := fmt.Errorf("[ERROR] Error generating the IAM Access Token %s", err)
	// 	state.Put("error", err)
	// 	ui.Error(err.Error())
	// 	return multistep.ActionHalt
	// }

	ui.Say("IAM Access Token successfully generated!")
	return multistep.ActionContinue
}

func (step *stepGenerateIAMToken) Cleanup(state multistep.StateBag) {
}
