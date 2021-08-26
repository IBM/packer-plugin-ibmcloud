package vpc

import (
	"context"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepGreeting struct {
}

func (step *stepGreeting) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)

	ui.Say("*************************************************************")
	ui.Say("* Initializing IBM Cloud Packer Plugin - VPC Infrastructure *")
	ui.Say("*************************************************************")
	ui.Say("")

	return multistep.ActionContinue
}

func (step *stepGreeting) Cleanup(state multistep.StateBag) {
}
