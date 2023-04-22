package vpc

import (
	"context"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepGreeting struct {
}

func (step *StepGreeting) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)

	ui.Say("*************************************************************")
	ui.Say("* Initializing IBM Cloud Packer Plugin - VPC Infrastructure *")
	ui.Say("*************************************************************")
	ui.Say("")

	return multistep.ActionContinue
}

func (step *StepGreeting) Cleanup(state multistep.StateBag) {
}
