package vpc

import (
	"context"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepGreeting struct {
}

func (step *StepGreeting) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	imageID := config.ImageID
	if imageID == "" {
		ui.Say("*************************************************************")
		ui.Say("* Initializing IBM Cloud Packer Plugin - VPC Infrastructure *")
		ui.Say("*************************************************************")
		ui.Say("")
	} else {
		ui.Say("**********************************************************************************************")
		ui.Say("* Initializing IBM Cloud Packer Post Processor Plugin for Image Export  - VPC Infrastructure *")
		ui.Say("**********************************************************************************************")
		ui.Say("")
	}

	return multistep.ActionContinue
}

func (step *StepGreeting) Cleanup(state multistep.StateBag) {
}
