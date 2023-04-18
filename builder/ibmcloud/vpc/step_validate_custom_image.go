package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepValidateCustomImage struct{}

func (s *stepValidateCustomImage) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	ui.Say(fmt.Sprintf("Checking the custom image: %s for redundancy", config.ImageName))

	listImagesOptions := &vpcv1.ListImagesOptions{
		Name: &config.ImageName,
	}

	// if visibility != "" {
	// 	listImagesOptions.Visibility = &visibility
	// }
	availableImages, _, err := vpcService.ListImages(listImagesOptions)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error checking custom image %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	allrecs := availableImages.Images

	if len(allrecs) != 0 {
		err := fmt.Errorf("[ERROR] Existing custom image found with name: %s", config.ImageName)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	ui.Say("Custom Image verified for redundancy, check Passed")

	return multistep.ActionContinue
}

func (s *stepValidateCustomImage) Cleanup(state multistep.StateBag) {

}
