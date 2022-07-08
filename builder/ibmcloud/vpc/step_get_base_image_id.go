package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepGetBaseImageID struct {
}

func (step *stepGetBaseImageID) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)
	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	// Fetching Base Image ID
	if config.VSIBaseImageName != "" {
		ui.Say("Fetching Base Image ID...")
		options := &vpcv1.ListImagesOptions{
			Name: &config.VSIBaseImageName,
		}
		imageList, _, err := vpcService.ListImages(options)
		imageId := *imageList.Images[0].ID
		if err != nil {
			err := fmt.Errorf("[ERROR] Error getting base-image ID: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		state.Put("baseImageID", imageId)
		ui.Say(fmt.Sprintf("Base Image ID fetched: %s", imageId))
	}

	return multistep.ActionContinue
}

func (step *stepGetBaseImageID) Cleanup(state multistep.StateBag) {
}
