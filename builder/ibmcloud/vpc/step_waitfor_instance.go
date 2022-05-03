package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepWaitforInstance struct{}

func (s *stepWaitforInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	ui.Say("Waiting for the instance to become ACTIVE...")
	instanceData := state.Get("instance_data").(*vpcv1.Instance)
	instanceID := *instanceData.ID
	err := client.waitForResourceReady(instanceID, "instances", config.StateTimeout, state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error step waiting for instance to become ACTIVE: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	// Update instance_data with new information unavailable at creation time (Private_IP, etc..)
	newInstanceData, _ := client.retrieveResource(instanceID, state)
	state.Put("instance_data", newInstanceData)
	ui.Say("Instance is ACTIVE!")
	return multistep.ActionContinue
}

func (client *stepWaitforInstance) Cleanup(state multistep.StateBag) {}
