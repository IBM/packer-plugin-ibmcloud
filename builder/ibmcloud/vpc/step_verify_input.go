package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcemanagerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepVerifyInput struct{}

func (s *stepVerifyInput) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)

	// vpc service
	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}
	// region check
	getRegionOptions := &vpcv1.GetRegionOptions{
		Name: &config.Region,
	}
	_, _, err := vpcService.GetRegion(getRegionOptions)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error fetching region : %s: %s", config.Region, err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	ui.Say("Region verified, check Passed")
	// region check ends
	// resource group check
	if config.ResourceGroupID != "" {

		serviceClientOptions := &resourcemanagerv2.ResourceManagerV2Options{
			Authenticator: &core.IamAuthenticator{
				ApiKey: client.IBMApiKey,
				URL:    config.IAMEndpoint,
			},
			// URL: ,
		}
		serviceClient, err := resourcemanagerv2.NewResourceManagerV2UsingExternalConfig(serviceClientOptions)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error fetching resource group : %s: %s", config.ResourceGroupID, err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		result, _, err := serviceClient.GetResourceGroup(serviceClient.NewGetResourceGroupOptions(config.ResourceGroupID))
		if err != nil {
			err := fmt.Errorf("[ERROR] Error fetching resource group : %s: %s", config.ResourceGroupID, err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		} else if result == nil {
			err := fmt.Errorf("[ERROR] Resource group not found resource_group_id : %s: %s", config.ResourceGroupID, err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		} else {
			ui.Say(fmt.Sprintf("Continuing with resource group : %s", *result.Name))
		}
	}
	// image check
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
	// image check ends

	// security group verification
	if config.SecurityGroupID != "" {
		secgrpOption := &vpcv1.GetSecurityGroupOptions{
			ID: &config.SecurityGroupID,
		}
		secGrp, _, err := vpcService.GetSecurityGroup(secgrpOption)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error fetching security group %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		if *secGrp.ID != "" {
			ui.Say(fmt.Sprintf("Using provided security group: %s with security_group_id: %s", *secGrp.Name, *secGrp.ID))
		}
		// Check for security group's VPC, must be same for the subnet - Check should happen at the subnet level.
		// if secGrp.VPC.ID != &config.VPCID {
		// 	ui.Say("Security Group doesn't fall in same VPC as the subnet!")
		// 	// state.Put("error", err)
		// 	// ui.Error(err.Error())
		// 	// return multistep.ActionHalt
		// }
	}
	return multistep.ActionContinue
}

func (s *stepVerifyInput) Cleanup(state multistep.StateBag) {

}
