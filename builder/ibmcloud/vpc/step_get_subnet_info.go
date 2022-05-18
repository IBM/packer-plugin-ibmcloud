package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepGetSubnetInfo struct{}

func (s *stepGetSubnetInfo) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	ui.Say(fmt.Sprintf("Retrieving Subnet %s information...", config.SubnetID))

	options := &vpcv1.GetSubnetOptions{}
	options.SetID(config.SubnetID)
	subnetData, _, err := vpcService.GetSubnet(options)

	if err != nil {
		err := fmt.Errorf("[ERROR] Error fetching subnet %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	vpcId := *subnetData.VPC.ID
	zone := *subnetData.Zone.Name

	state.Put("vpc_id", vpcId)
	state.Put("zone", zone)

	ui.Say("Subnet Information successfully retrieved ...")
	ui.Say(fmt.Sprintf("VPC ID: %s", vpcId))
	ui.Say(fmt.Sprintf("Zone: %s", zone))

	return multistep.ActionContinue
}

func (s *stepGetSubnetInfo) Cleanup(state multistep.StateBag) {

}
