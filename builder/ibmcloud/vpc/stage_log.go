package vpc

import (
	"context"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// Stage name constants — used in withStage() calls in builder.go.
const (
	StageCreateInstance       = "create_instance"
	StageWaitInstance         = "wait_instance"
	StageInstallingComponents = "installing_components"
	StageCaptureImage         = "capture_image"
	StageWaitImage            = "wait_image"
)

// emitStage writes a machine-readable stage marker to the Packer UI.
// The marker format is:  STAGE <stage> <status>
// status is one of: START, END, FAIL
//
// ui.Machine routes to the machine-readable output channel (log only for
// BasicUi; structured CSV when packer is run with -machine-readable) and
// does not pollute the human-readable terminal output.
func emitStage(ui packer.Ui, stage, status string) {
	ui.Machine("stage", stage, status)
}

// stagedStep wraps a multistep.Step and automatically emits START/END/FAIL
// machine-readable stage markers around the inner step's Run.
type stagedStep struct {
	stage string
	inner multistep.Step
}

// withStage wraps inner so that its Run is bracketed by stage markers.
func withStage(stage string, inner multistep.Step) multistep.Step {
	return &stagedStep{stage: stage, inner: inner}
}

func (s *stagedStep) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	emitStage(ui, s.stage, "START")
	action := s.inner.Run(ctx, state)
	if action == multistep.ActionHalt {
		emitStage(ui, s.stage, "FAIL")
	} else {
		emitStage(ui, s.stage, "END")
	}
	return action
}

func (s *stagedStep) Cleanup(state multistep.StateBag) {
	s.inner.Cleanup(state)
}
