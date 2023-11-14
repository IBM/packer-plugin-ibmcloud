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
	subnetData, response, err := vpcService.GetSubnet(options)

	if err != nil {
		xRequestId := response.Headers["X-Request-Id"][0]
		xCorrelationId := ""
		if len(response.Headers["X-Correlation-Id"]) != 0 {
			xCorrelationId = fmt.Sprintf("\n X-Correlation-Id : %s", response.Headers["X-Correlation-Id"][0])
		}
		err := fmt.Errorf("[ERROR] Error fetching subnet %s \n X-Request-Id : %s  %s", err, xRequestId, xCorrelationId)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	vpcId := *subnetData.VPC.ID
	if config.SecurityGroupID != "" { // User provided security group
		secGrpVPC := state.Get("user_sec_grp_vpc")
		ui.Say("Verifying the security group and subnet belongs to same VPC..")
		if vpcId != secGrpVPC {
			err := fmt.Errorf("[ERROR] The security group and subnet provided are not connected to the same VPC id: %s", vpcId)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
	}
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
