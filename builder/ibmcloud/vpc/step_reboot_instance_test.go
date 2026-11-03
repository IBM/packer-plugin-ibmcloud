package vpc

import (
	"context"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

func TestStepRebootInstance_SkipReboot(t *testing.T) {
	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("config", Config{SkipReboot: true})

	step := &stepRebootInstance{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Fatalf("expected ActionContinue, got %v", action)
	}
}
