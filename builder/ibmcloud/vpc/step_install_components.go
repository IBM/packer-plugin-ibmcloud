package vpc

import (
	"context"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
)

type stepInstallComponents struct {
	provision commonsteps.StepProvision
}

func (s *stepInstallComponents) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	return s.provision.Run(ctx, state)
}

func (s *stepInstallComponents) Cleanup(state multistep.StateBag) {
	s.provision.Cleanup(state)
}
