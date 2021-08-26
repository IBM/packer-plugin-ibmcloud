package vpc

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepGenerateIAMToken struct {
}

func (step *stepGenerateIAMToken) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	ui := state.Get("ui").(packer.Ui)

	ui.Say("Generating IAM Access Token...")
	tokenData, err := client.getIAMToken(state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error generating the IAM Access Token %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}
	client.IAMToken = tokenData["token_type"].(string) + " " + tokenData["access_token"].(string)
	ui.Say("IAM Access Token successfully generated!")
	return multistep.ActionContinue
}

func (step *stepGenerateIAMToken) Cleanup(state multistep.StateBag) {
}
