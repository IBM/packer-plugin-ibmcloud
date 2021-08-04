package vpc

import (
	"context"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepWaitWinRM struct{}

func (s *stepWaitWinRM) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)

	// Wait around 3 minutes until WinRM becomea available
	ui.Say("Waiting for WinRM to become available (~3 minutes)...")
	time.Sleep(3 * time.Minute)
	return multistep.ActionContinue
}

func (client *stepWaitWinRM) Cleanup(state multistep.StateBag) {}
