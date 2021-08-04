package vpc

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepGetSubnetInfo struct{}

func (s *stepGetSubnetInfo) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)

	ui.Say(fmt.Sprintf("Retrieving Subnet %s information...", config.SubnetID))
	SubnetData, err := client.retrieveSubnet(state, config.SubnetID)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error retrieving Subnet information: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	vpc := SubnetData["vpc"].(map[string]interface{})
	state.Put("vpc_id", vpc["id"].(string))

	zone := SubnetData["zone"].(map[string]interface{})
	state.Put("zone", zone["name"].(string))

	ui.Say("Subnet Information successfully retrieved ...")
	ui.Say(fmt.Sprintf("VPC ID: %s", vpc["id"].(string)))
	ui.Say(fmt.Sprintf("Zone: %s", zone["name"].(string)))

	return multistep.ActionContinue
}

func (s *stepGetSubnetInfo) Cleanup(state multistep.StateBag) {

}
